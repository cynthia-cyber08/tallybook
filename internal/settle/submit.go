package settle

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Tallybook-Org/tallybook/internal/convert"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stellar/go-stellar-sdk/keypair"
)

// Sentinel errors for Submitter.
var (
	// ErrCommitmentNotFound means there is no valid commitment for the
	// exact (channel, amount) pair being submitted — Submit refuses to
	// settle for an amount nothing on file actually signed for.
	ErrCommitmentNotFound = errors.New("settle: no valid commitment found for this channel and amount")
	// ErrInvalidSignatureLength means the stored commitment's signature
	// isn't the 64 bytes Channel.Settle requires.
	ErrInvalidSignatureLength = errors.New("settle: commitment signature is not 64 bytes")
	// ErrSettlementInFlight means a settlements row for this exact
	// (channel, amount) already exists with status 'submitting' — the
	// process crashed mid-attempt last time, or another submitter is
	// already working on it. Submit refuses to act on it again without
	// ReconcileSubmitting resolving it first, per §6 ordering rule 3: "On
	// restart, reconcile any submitting row against the chain before
	// doing anything else."
	ErrSettlementInFlight = errors.New("settle: a settlement for this channel and amount is already submitting; reconcile before retrying")
)

// ChannelSettler is the one piece of *stellar.Channel submission needs. A
// narrow interface keeps this package testable without a live Soroban RPC
// endpoint.
type ChannelSettler interface {
	Settle(ctx context.Context, signer *keypair.Full, amount *big.Int, sig [64]byte) (string, error)
}

// ChannelWithdrawnReader is the one piece of *stellar.Channel
// ReconcileSubmitting needs: a live read of how much has actually been
// withdrawn on chain.
type ChannelWithdrawnReader interface {
	Withdrawn(ctx context.Context) (*big.Int, error)
}

// Record is a settlements row (§5).
type Record struct {
	ID               int64
	Channel          string
	CumulativeAmount *big.Int
	Status           string // submitting | confirmed | failed
	TxHash           string
	FailureReason    string
	SubmittedAt      time.Time
	ConfirmedAt      *time.Time
}

// Submitter records settlement intent durably before ever calling the
// chain, and submits it — implementing §6 ordering rules 3 and 4.
type Submitter struct {
	pool *pgxpool.Pool
}

// NewSubmitter returns a Submitter backed by pool.
func NewSubmitter(pool *pgxpool.Pool) *Submitter {
	return &Submitter{pool: pool}
}

const findSettlementSQL = `
SELECT id, status, tx_hash, failure_reason, submitted_at, confirmed_at
FROM settlements WHERE channel = $1 AND cumulative_amount = $2`

func (s *Submitter) find(ctx context.Context, channel string, amount pgtype.Numeric) (*Record, error) {
	var rec Record
	var txHash, failureReason *string
	err := s.pool.QueryRow(ctx, findSettlementSQL, channel, amount).
		Scan(&rec.ID, &rec.Status, &txHash, &failureReason, &rec.SubmittedAt, &rec.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("settle: find settlement: %w", err)
	}
	rec.Channel = channel
	if txHash != nil {
		rec.TxHash = *txHash
	}
	if failureReason != nil {
		rec.FailureReason = *failureReason
	}
	return &rec, nil
}

const findValidCommitmentSignatureSQL = `
SELECT signature FROM commitments WHERE channel = $1 AND cumulative_amount = $2 AND valid = true`

// lookupCommitmentSignature fetches the funder's own signature for
// exactly this (channel, amount) pair — never one supplied by the caller —
// so Submit can never settle for an amount that wasn't actually signed and
// verified (custody.Store already did that verification before the
// commitment was ever stored; Submit only re-reads its outcome).
func (s *Submitter) lookupCommitmentSignature(ctx context.Context, channel string, amount pgtype.Numeric) ([64]byte, error) {
	var sig [64]byte
	var raw []byte
	err := s.pool.QueryRow(ctx, findValidCommitmentSignatureSQL, channel, amount).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return sig, ErrCommitmentNotFound
	}
	if err != nil {
		return sig, fmt.Errorf("settle: lookup commitment signature: %w", err)
	}
	if len(raw) != ed25519.SignatureSize {
		return sig, fmt.Errorf("%w: got %d bytes", ErrInvalidSignatureLength, len(raw))
	}
	copy(sig[:], raw)
	return sig, nil
}

const insertSettlementSQL = `
INSERT INTO settlements (channel, cumulative_amount, status)
VALUES ($1, $2, 'submitting')
ON CONFLICT (channel, cumulative_amount) DO NOTHING
RETURNING id, submitted_at`

const markConfirmedSQL = `
UPDATE settlements SET status = 'confirmed', tx_hash = NULLIF($2, ''), confirmed_at = now() WHERE id = $1`

const markFailedSQL = `
UPDATE settlements SET status = 'failed', tx_hash = NULLIF($2, ''), failure_reason = $3 WHERE id = $1`

// Submit is the whole settle-and-record flow for one channel and amount,
// implementing §6 ordering rules 3 ("record intent before submitting")
// and 4 ("idempotent by construction, keyed by (channel,
// cumulative_amount)").
//
// If a row already exists for this exact (channel, amount):
//   - status 'confirmed' or 'failed': returned as-is, no chain call at
//     all — a retry of an already-resolved settlement is always safe.
//   - status 'submitting': returned alongside ErrSettlementInFlight — an
//     unresolved prior attempt exists (a crash mid-flight, most likely),
//     and Submit refuses to guess whether it landed. Call
//     ReconcileSubmitting first.
//
// Otherwise: a 'submitting' row is written and durably committed before
// channel.Settle is ever called (so a crash between here and the chain
// call leaves unambiguous evidence, not a silent gap), the commitment's
// own on-file signature is used (never one supplied by the caller), and
// the outcome — confirmed with its tx hash, or failed with the error —
// is recorded once Settle returns.
func (s *Submitter) Submit(ctx context.Context, channel ChannelSettler, signer *keypair.Full, channelAddress string, amount *big.Int) (*Record, error) {
	rec, sig, resolved, err := s.recordOrFindIntent(ctx, channelAddress, amount)
	if err != nil {
		return nil, err
	}
	if resolved {
		if SettlementStatus(rec.Status) == SettlementStatusSubmitting {
			return rec, fmt.Errorf("settle: submit %s %s: %w", channelAddress, amount, ErrSettlementInFlight)
		}
		return rec, nil
	}

	hash, sendErr := channel.Settle(ctx, signer, amount, sig)
	if sendErr != nil {
		// A single-attempt Submit marks any send failure final —
		// permanent or not. A caller that wants transient failures
		// retried without prematurely closing out this (channel, amount)
		// as failed should use SubmitWithRetry instead, which manages
		// this same row's lifecycle across attempts rather than settling
		// it after just one.
		if markErr := s.markFailed(ctx, rec.ID, hash, sendErr.Error()); markErr != nil {
			return nil, fmt.Errorf("settle: submit %s %s: send failed (%w) and marking it failed also failed: %w",
				channelAddress, amount, sendErr, markErr)
		}
		rec.Status = "failed"
		rec.TxHash = hash
		rec.FailureReason = sendErr.Error()
		return rec, fmt.Errorf("settle: submit %s %s: %w", channelAddress, amount, sendErr)
	}

	if err := s.markConfirmed(ctx, rec.ID, hash); err != nil {
		return nil, fmt.Errorf("settle: submit %s %s: mark confirmed: %w", channelAddress, amount, err)
	}
	rec.Status = "confirmed"
	rec.TxHash = hash
	return rec, nil
}

// recordOrFindIntent is the shared find-or-record-intent step both Submit
// and SubmitWithRetry use: if a row already exists for (channelAddress,
// amount), it's returned with resolved=true (the caller does not proceed
// to call the chain — Submit and SubmitWithRetry each decide what "already
// exists" means for their own semantics). Otherwise a fresh 'submitting'
// row is written, durably, before returning — §6 ordering rule 3 — along
// with the commitment's own signature, ready for the caller's chain call.
func (s *Submitter) recordOrFindIntent(ctx context.Context, channelAddress string, amount *big.Int) (rec *Record, sig [64]byte, resolved bool, err error) {
	if amount == nil {
		return nil, sig, false, errors.New("settle: submit: amount is nil")
	}
	amountNum := pgtype.Numeric{Int: new(big.Int).Set(amount), Exp: 0, Valid: true}

	existing, err := s.find(ctx, channelAddress, amountNum)
	if err != nil {
		return nil, sig, false, err
	}
	if existing != nil {
		return existing, sig, true, nil
	}

	sig, err = s.lookupCommitmentSignature(ctx, channelAddress, amountNum)
	if err != nil {
		return nil, sig, false, fmt.Errorf("settle: submit %s %s: %w", channelAddress, amount, err)
	}

	var fresh Record
	err = s.pool.QueryRow(ctx, insertSettlementSQL, channelAddress, amountNum).Scan(&fresh.ID, &fresh.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Lost a race with a concurrent caller for the same key between
		// find and here. Re-fetch and treat it as already existing.
		again, findErr := s.find(ctx, channelAddress, amountNum)
		if findErr != nil {
			return nil, sig, false, findErr
		}
		if again == nil {
			return nil, sig, false, fmt.Errorf("settle: submit %s %s: insert conflicted but no row found on re-read", channelAddress, amount)
		}
		return again, sig, true, nil
	}
	if err != nil {
		return nil, sig, false, fmt.Errorf("settle: submit %s %s: record intent: %w", channelAddress, amount, err)
	}
	fresh.Channel = channelAddress
	fresh.CumulativeAmount = amount
	fresh.Status = "submitting"
	return &fresh, sig, false, nil
}

func (s *Submitter) markConfirmed(ctx context.Context, id int64, txHash string) error {
	_, err := s.pool.Exec(ctx, markConfirmedSQL, id, txHash)
	return err
}

func (s *Submitter) markFailed(ctx context.Context, id int64, txHash, reason string) error {
	_, err := s.pool.Exec(ctx, markFailedSQL, id, txHash, reason)
	return err
}

const findSubmittingSQL = `
SELECT id, cumulative_amount FROM settlements WHERE channel = $1 AND status = 'submitting'`

// ReconcileSubmitting resolves every 'submitting' settlements row for
// channelAddress against the channel's own live on-chain state — §6
// ordering rule 3: "On restart, reconcile any submitting row against the
// chain before doing anything else." A submitting row whose
// cumulative_amount is already covered by the channel's current live
// Withdrawn total is marked confirmed (no tx_hash — a live balance read
// can't recover one, and the row's own existence plus the on-chain amount
// is already sufficient evidence the settle landed). Anything not yet
// covered is marked failed, clearing the way for a fresh Submit.
func (s *Submitter) ReconcileSubmitting(ctx context.Context, channelAddress string, channel ChannelWithdrawnReader) ([]*Record, error) {
	rows, err := s.pool.Query(ctx, findSubmittingSQL, channelAddress)
	if err != nil {
		return nil, fmt.Errorf("settle: reconcile submitting for %s: query: %w", channelAddress, err)
	}
	type pending struct {
		id     int64
		amount *big.Int
	}
	var items []pending
	for rows.Next() {
		var id int64
		var amountNum pgtype.Numeric
		if err := rows.Scan(&id, &amountNum); err != nil {
			rows.Close()
			return nil, fmt.Errorf("settle: reconcile submitting for %s: scan: %w", channelAddress, err)
		}
		amount, err := convert.NumericToBigInt(amountNum)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("settle: reconcile submitting for %s: %w", channelAddress, err)
		}
		items = append(items, pending{id: id, amount: amount})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settle: reconcile submitting for %s: %w", channelAddress, err)
	}
	rows.Close()

	if len(items) == 0 {
		return nil, nil
	}

	withdrawn, err := channel.Withdrawn(ctx)
	if err != nil {
		return nil, fmt.Errorf("settle: reconcile submitting for %s: read live withdrawn: %w", channelAddress, err)
	}

	resolved := make([]*Record, 0, len(items))
	for _, p := range items {
		if withdrawn.Cmp(p.amount) >= 0 {
			if err := s.markConfirmed(ctx, p.id, ""); err != nil {
				return resolved, fmt.Errorf("settle: reconcile submitting for %s: mark confirmed: %w", channelAddress, err)
			}
			resolved = append(resolved, &Record{ID: p.id, Channel: channelAddress, CumulativeAmount: p.amount, Status: "confirmed"})
		} else {
			reason := "reconciled after restart: amount not covered by the channel's live withdrawn total"
			if err := s.markFailed(ctx, p.id, "", reason); err != nil {
				return resolved, fmt.Errorf("settle: reconcile submitting for %s: mark failed: %w", channelAddress, err)
			}
			resolved = append(resolved, &Record{ID: p.id, Channel: channelAddress, CumulativeAmount: p.amount, Status: "failed", FailureReason: reason})
		}
	}
	return resolved, nil
}
