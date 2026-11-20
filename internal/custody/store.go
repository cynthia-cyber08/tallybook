package custody

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// commitmentsMonotonicViolationSQLState is the custom SQLSTATE the
// commitments_reject_regression trigger raises (see
// internal/store/migrations/0003_commitments.sql) when an insert's
// cumulative_amount is lower than the channel's highest already stored.
const commitmentsMonotonicViolationSQLState = "TB001"

// ErrMonotonicViolation means Submit's insert was rejected by the
// database's own monotonic-invariant trigger: the commitment's
// cumulative_amount is lower than the highest already recorded for this
// channel. Nothing was written — the trigger runs BEFORE INSERT.
var ErrMonotonicViolation = errors.New("custody: cumulative_amount is lower than the highest already stored for this channel")

// ErrVerificationIncomplete wraps a VerifyCommitment error that means no
// verification outcome was reached at all — as opposed to a completed
// verification that found the commitment invalid. Submit persists nothing
// in this case; see VerifyCommitment's doc comment for why the two are
// kept distinct.
var ErrVerificationIncomplete = errors.New("custody: commitment verification did not complete")

// SubmittedCommitment is what a caller hands to Store.Submit: a payer's
// claimed commitment, plus the signer key the caller has decided to trust
// for this channel (see VerifyCommitment's doc comment on why that
// decision belongs to the caller, not this package).
type SubmittedCommitment struct {
	Channel          string
	CumulativeAmount *big.Int
	Signature        []byte
	// TrustedSignerKey must be this channel's real commitment_key, sourced
	// from the indexer's durable channel-state tracking (captured once at
	// channel-open time; §4 exposes no on-chain getter for it) — never a
	// value read off the payer's own request. It is NOT the channel's
	// `from` (funder) Stellar account address, which is a different value
	// entirely; see VerifyCommitment's doc comment for why the distinction
	// matters and why this is the caller's responsibility, not this
	// package's.
	TrustedSignerKey ed25519.PublicKey
}

// Record is a persisted commitments row.
type Record struct {
	ID               int64
	Channel          string
	CumulativeAmount *big.Int
	Signature        []byte
	SignerKey        []byte
	VerifiedAt       time.Time
	Valid            bool
	// AlreadyRecorded is true when Submit found an existing row for this
	// exact (channel, cumulative_amount) instead of inserting a new one —
	// a retry, not a fresh commitment. See Submit's doc comment.
	AlreadyRecorded bool
}

// Store persists commitments to the commitments table (§5).
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const insertCommitmentSQL = `
INSERT INTO commitments (channel, cumulative_amount, signature, signer_key, verified_at, valid)
VALUES ($1, $2, $3, $4, now(), $5)
ON CONFLICT (channel, cumulative_amount) DO NOTHING
RETURNING id, verified_at, valid`

const selectCommitmentSQL = `
SELECT id, verified_at, valid FROM commitments WHERE channel = $1 AND cumulative_amount = $2`

// Submit verifies sc (via VerifyCommitment, simulating prepare_commitment
// against channel) and durably persists the result — valid or not —
// before returning, so the row exists in Postgres before any caller
// acknowledges the payer (§6 ordering rule 1: "Persist before
// acknowledging"). It never acts on the commitment beyond recording it;
// deciding whether to settle against a Record is the settler's job,
// against rows this method has already made durable (ordering rule 2:
// Record.Valid is exactly the gate "never settle against an unverified
// commitment" depends on).
//
// If verification does not complete at all — a malformed argument, or the
// prepare_commitment simulation itself failing — Submit persists nothing
// and returns an error wrapping ErrVerificationIncomplete: ordering rule 2
// is "verify before storing", and an incomplete verification has no
// outcome yet to store. This is different from a completed verification
// that finds the commitment invalid, which Submit does persist — per §6's
// failure handling ("A commitment that fails verification: store it, mark
// it invalid, alert, never settle against it"): an invalid commitment is
// still evidence (of a bug, an attack, or a stale signer key) and must not
// be silently dropped, only prevented from being settled against.
//
// A retry with the exact same (channel, cumulative_amount) is safe:
// commitments' UNIQUE(channel, cumulative_amount) constraint (§5) makes a
// second Submit for the same pair a read of the existing row
// (Record.AlreadyRecorded = true) rather than an error or a duplicate
// write — so a payer's retry after an acknowledgement was lost to a crash
// cannot fail spuriously or double-record anything.
//
// A cumulative_amount lower than the channel's highest already stored is
// rejected by the database's own monotonic trigger before the row is
// written at all; Submit surfaces that as an error wrapping
// ErrMonotonicViolation.
func (s *Store) Submit(ctx context.Context, channel ChannelReader, sc SubmittedCommitment) (*Record, error) {
	if sc.CumulativeAmount == nil {
		return nil, ErrNilAmount
	}

	// sc.TrustedSignerKey must already be the caller's indexed
	// commitment_key for this channel, not the payer's own claim — see
	// SubmittedCommitment.TrustedSignerKey's doc comment. Submit trusts
	// whatever key it's handed here; it does not, and cannot, verify that
	// the key itself is the right one.
	valid, err := VerifyCommitment(ctx, channel, sc.TrustedSignerKey, sc.CumulativeAmount, sc.Signature)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVerificationIncomplete, err)
	}

	amount := pgtype.Numeric{Int: new(big.Int).Set(sc.CumulativeAmount), Exp: 0, Valid: true}
	signerKey := []byte(sc.TrustedSignerKey)

	var (
		id         int64
		verifiedAt time.Time
		gotValid   bool
	)
	err = s.pool.QueryRow(ctx, insertCommitmentSQL, sc.Channel, amount, sc.Signature, signerKey, valid).
		Scan(&id, &verifiedAt, &gotValid)

	switch {
	case err == nil:
		return &Record{
			ID: id, Channel: sc.Channel, CumulativeAmount: sc.CumulativeAmount,
			Signature: sc.Signature, SignerKey: signerKey, VerifiedAt: verifiedAt, Valid: gotValid,
		}, nil

	case errors.Is(err, pgx.ErrNoRows):
		// ON CONFLICT DO NOTHING found an existing row instead of
		// inserting: this exact (channel, cumulative_amount) was already
		// recorded by an earlier Submit. Read it back rather than treat
		// the retry as a failure.
		var existing Record
		if selErr := s.pool.QueryRow(ctx, selectCommitmentSQL, sc.Channel, amount).
			Scan(&existing.ID, &existing.VerifiedAt, &existing.Valid); selErr != nil {
			return nil, fmt.Errorf("custody: submit commitment: read existing row after conflict: %w", selErr)
		}
		existing.Channel = sc.Channel
		existing.CumulativeAmount = sc.CumulativeAmount
		existing.Signature = sc.Signature
		existing.SignerKey = signerKey
		existing.AlreadyRecorded = true
		return &existing, nil

	default:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == commitmentsMonotonicViolationSQLState {
			return nil, fmt.Errorf("%w: %s", ErrMonotonicViolation, pgErr.Message)
		}
		return nil, fmt.Errorf("custody: submit commitment: %w", err)
	}
}
