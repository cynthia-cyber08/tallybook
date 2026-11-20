package meter

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/Tallybook-Org/tallybook/internal/convert"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stellar/go-stellar-sdk/keypair"

	"github.com/Tallybook-Org/tallybook/internal/merkle"
	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

// Sentinel errors for Anchorer.
var (
	ErrPeriodNotClosed = errors.New("meter: period is not closed, cannot anchor it")
	ErrPeriodEmpty     = errors.New("meter: period has no requests to anchor")
	// ErrAnchorInFlight means an anchor_attempts row for this period is
	// already 'submitting' — a prior attempt is unresolved, most likely a
	// crash mid-call. AnchorPeriod refuses to act on it again without
	// ReconcileAnchoring resolving it first, mirroring
	// internal/settle.ErrSettlementInFlight.
	ErrAnchorInFlight = errors.New("meter: an anchor attempt for this period is already submitting; reconcile before retrying")
)

// isPermanentAnchorError draws the same line internal/settle.IsPermanent
// draws for settlements, applied to anchoring: *stellar.ErrSimulationFailed
// and *stellar.ErrTransactionFailed both mean the call genuinely reached
// the chain and was rejected there, so retrying changes nothing and the
// attempt is recorded failed outright. Anything else (a network/RPC-level
// problem) is treated as transient — the attempt is left 'submitting' for
// ReconcileAnchoring rather than guessed at.
//
// Duplicated in spirit from internal/settle.IsPermanent rather than
// imported: settle depends on nothing meter-specific and there's no
// import cycle either direction, but pulling in a whole sibling package
// for one two-line classification (whose settle-specific sentinels don't
// even apply here) is more coupling than the reuse is worth. The pattern
// — record intent before the call, distinguish permanent from transient,
// reconcile an unresolved attempt against the chain's real state rather
// than guessing — is what's reused; the Go symbol isn't.
func isPermanentAnchorError(err error) bool {
	var simErr *stellar.ErrSimulationFailed
	var txErr *stellar.ErrTransactionFailed
	return errors.As(err, &simErr) || errors.As(err, &txErr)
}

// protocolFor maps periods.protocol (lowercase — matching httpapi's own
// Protocol* constants) to statement_registry's Protocol enum (mixed/upper
// case; the wire format is case-sensitive, §4).
func protocolFor(protocol string) (stellar.Protocol, error) {
	switch protocol {
	case "x402":
		return stellar.ProtocolX402, nil
	case "mpp_charge":
		return stellar.ProtocolMppCharge, nil
	case "mpp_session":
		return stellar.ProtocolMppSession, nil
	default:
		return "", fmt.Errorf("meter: anchor: unknown protocol %q", protocol)
	}
}

// AnchorSubmitter is the one piece of *stellar.StatementRegistry
// anchoring needs. A narrow interface — satisfied by
// *stellar.StatementRegistry — keeps this package testable without a
// live Soroban RPC endpoint, matching every other package's own pattern.
type AnchorSubmitter interface {
	Anchor(ctx context.Context, signer *keypair.Full, consumer string, periodStart, periodEnd uint32,
		usageRoot [32]byte, requestCount uint64, token string, amountBilled, amountSettled *big.Int,
		priceBookVersion uint32, protocol stellar.Protocol, channel *string) (uint64, string, error)
}

// StatementReader is the read side of *stellar.StatementRegistry
// ReconcileAnchoring needs to resolve an unresolved anchor attempt
// against the chain's real state.
type StatementReader interface {
	ListStatements(ctx context.Context, operator, consumer string) ([]uint64, error)
	GetStatement(ctx context.Context, operator string, seq uint64) (*stellar.Statement, error)
}

// Anchorer builds the merkle usage root for a closed period's metered
// requests and anchors it on chain via statement_registry.anchor (§4),
// with the same crash-safe intent-recording discipline
// internal/settle.Submitter uses for settlements (see anchor_attempts.sql
// and this file's own doc comments for how the pattern was adapted).
type Anchorer struct {
	pool *pgxpool.Pool
}

// NewAnchorer returns an Anchorer backed by pool.
func NewAnchorer(pool *pgxpool.Pool) *Anchorer {
	return &Anchorer{pool: pool}
}

type periodRow struct {
	id                        int64
	operator, consumer        string
	protocol                  string
	channel                   *string
	priceVersion, periodStart uint32
	periodEnd                 *uint32 // nil while status is 'open'; always set once 'closed' or 'anchored'
	status                    string
	statementSeq              *uint64
}

const fetchPeriodSQL = `
SELECT id, operator, consumer, protocol, channel, price_version, period_start, period_end, status, statement_seq
FROM periods WHERE id = $1`

// fetchPeriod does not require period_end to be set — an 'open' period
// never has one (periods' own CHECK constraint, 0001_periods.sql), and
// AnchorPeriod must still be able to fetch such a row far enough to
// report ErrPeriodNotClosed with a clear status, rather than an opaque
// "no period_end" error before ever checking status at all.
func (a *Anchorer) fetchPeriod(ctx context.Context, periodID int64) (*periodRow, error) {
	var p periodRow
	var priceVersion, periodStart int32
	var periodEnd *int32
	var statementSeq *int64
	err := a.pool.QueryRow(ctx, fetchPeriodSQL, periodID).Scan(
		&p.id, &p.operator, &p.consumer, &p.protocol, &p.channel, &priceVersion, &periodStart, &periodEnd, &p.status, &statementSeq,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("meter: anchor: period %d not found", periodID)
	}
	if err != nil {
		return nil, fmt.Errorf("meter: anchor: fetch period %d: %w", periodID, err)
	}
	p.priceVersion = uint32(priceVersion)
	p.periodStart = uint32(periodStart)
	if periodEnd != nil {
		v := uint32(*periodEnd)
		p.periodEnd = &v
	}
	if statementSeq != nil {
		v := uint64(*statementSeq)
		p.statementSeq = &v
	}
	return &p, nil
}

const fetchPeriodRequestsSQL = `
SELECT request_id, endpoint_hash, unit_count, charged_amount, observed_ledger
FROM requests WHERE period_id = $1 ORDER BY id`

type requestRow struct {
	requestID, endpointHash [32]byte
	unitCount               uint64
	chargedAmount           *big.Int
	observedLedger          uint32
}

func (a *Anchorer) fetchRequests(ctx context.Context, periodID int64) ([]requestRow, error) {
	rows, err := a.pool.Query(ctx, fetchPeriodRequestsSQL, periodID)
	if err != nil {
		return nil, fmt.Errorf("meter: anchor: fetch requests for period %d: %w", periodID, err)
	}
	defer rows.Close()

	var out []requestRow
	for rows.Next() {
		var requestID, endpointHash []byte
		var chargedAmount pgtype.Numeric
		var unitCount int64
		var observedLedger int32
		if err := rows.Scan(&requestID, &endpointHash, &unitCount, &chargedAmount, &observedLedger); err != nil {
			return nil, fmt.Errorf("meter: anchor: scan request row: %w", err)
		}
		amount, err := convert.NumericToBigInt(chargedAmount)
		if err != nil {
			return nil, fmt.Errorf("meter: anchor: charged_amount: %w", err)
		}
		row := requestRow{unitCount: uint64(unitCount), chargedAmount: amount, observedLedger: uint32(observedLedger)}
		if len(requestID) != 32 || len(endpointHash) != 32 {
			return nil, fmt.Errorf("meter: anchor: request/endpoint id is not 32 bytes")
		}
		copy(row.requestID[:], requestID)
		copy(row.endpointHash[:], endpointHash)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meter: anchor: fetch requests for period %d: %w", periodID, err)
	}
	return out, nil
}

// usage is what computeUsage derives from a period's requests — the same
// deterministic computation both a fresh AnchorPeriod attempt and a later
// ReconcileAnchoring call make, from the same (immutable, once the period
// is closed) requests rows, so the two always agree on what the usage
// root "should" be for a given period without needing to persist it
// anywhere in between.
type usage struct {
	tree         *merkle.Tree
	requestCount uint64
	amountBilled *big.Int
}

func (a *Anchorer) computeUsage(ctx context.Context, period *periodRow, periodID int64) (*usage, error) {
	requests, err := a.fetchRequests(ctx, periodID)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("meter: anchor period %d: %w", periodID, ErrPeriodEmpty)
	}

	leaves := make([][32]byte, len(requests))
	amountBilled := big.NewInt(0)
	for i, r := range requests {
		leaf, err := merkle.Leaf(merkle.Record{
			Amount:       r.chargedAmount,
			Consumer:     period.consumer,
			EndpointHash: r.endpointHash,
			Ledger:       r.observedLedger,
			PriceVersion: period.priceVersion,
			RequestID:    r.requestID,
			Units:        r.unitCount,
		})
		if err != nil {
			return nil, fmt.Errorf("meter: anchor period %d: build leaf for request %x: %w", periodID, r.requestID, err)
		}
		leaves[i] = leaf
		amountBilled.Add(amountBilled, r.chargedAmount)
	}

	tree, err := merkle.BuildTree(leaves)
	if err != nil {
		return nil, fmt.Errorf("meter: anchor period %d: build tree: %w", periodID, err)
	}
	return &usage{tree: tree, requestCount: uint64(len(requests)), amountBilled: amountBilled}, nil
}

type attemptRow struct {
	id           int64
	status       string
	statementSeq *uint64
	txHash       *string
}

const findActiveAttemptSQL = `
SELECT id, status, statement_seq, tx_hash FROM anchor_attempts
WHERE period_id = $1 AND status IN ('submitting', 'confirmed')
ORDER BY id DESC LIMIT 1`

func (a *Anchorer) findActiveAttempt(ctx context.Context, periodID int64) (*attemptRow, error) {
	var ar attemptRow
	var statementSeq *int64
	err := a.pool.QueryRow(ctx, findActiveAttemptSQL, periodID).Scan(&ar.id, &ar.status, &statementSeq, &ar.txHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("meter: anchor: find active attempt for period %d: %w", periodID, err)
	}
	if statementSeq != nil {
		v := uint64(*statementSeq)
		ar.statementSeq = &v
	}
	return &ar, nil
}

const insertAttemptSQL = `
INSERT INTO anchor_attempts (period_id, status) VALUES ($1, 'submitting') RETURNING id`

func (a *Anchorer) recordAttempt(ctx context.Context, periodID int64) (int64, error) {
	var id int64
	err := a.pool.QueryRow(ctx, insertAttemptSQL, periodID).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Lost a race with a concurrent AnchorPeriod for the same
			// period between the earlier find and here.
			active, findErr := a.findActiveAttempt(ctx, periodID)
			if findErr != nil {
				return 0, findErr
			}
			if active != nil {
				return 0, fmt.Errorf("meter: anchor period %d: %w", periodID, ErrAnchorInFlight)
			}
		}
		return 0, fmt.Errorf("meter: anchor: record attempt for period %d: %w", periodID, err)
	}
	return id, nil
}

const markAttemptConfirmedSQL = `
UPDATE anchor_attempts SET status = 'confirmed', statement_seq = $2, tx_hash = NULLIF($3, ''), confirmed_at = now() WHERE id = $1`

func (a *Anchorer) markAttemptConfirmed(ctx context.Context, attemptID int64, seq uint64, txHash string) error {
	_, err := a.pool.Exec(ctx, markAttemptConfirmedSQL, attemptID, int64(seq), txHash)
	if err != nil {
		return fmt.Errorf("meter: anchor: mark attempt %d confirmed: %w", attemptID, err)
	}
	return nil
}

const markAttemptFailedSQL = `
UPDATE anchor_attempts SET status = 'failed', failure_reason = $2 WHERE id = $1`

func (a *Anchorer) markAttemptFailed(ctx context.Context, attemptID int64, reason string) error {
	_, err := a.pool.Exec(ctx, markAttemptFailedSQL, attemptID, reason)
	if err != nil {
		return fmt.Errorf("meter: anchor: mark attempt %d failed: %w", attemptID, err)
	}
	return nil
}

const markPeriodAnchoredSQL = `
UPDATE periods SET status = 'anchored', statement_seq = $2 WHERE id = $1 AND status = 'closed'`

func (a *Anchorer) markPeriodAnchored(ctx context.Context, periodID int64, seq uint64) error {
	_, err := a.pool.Exec(ctx, markPeriodAnchoredSQL, periodID, int64(seq))
	if err != nil {
		return fmt.Errorf("meter: anchor: mark period %d anchored: %w", periodID, err)
	}
	return nil
}

// BilledAmount returns the total amount billed for a period — the sum of
// its requests' charged amounts — which is exactly the figure
// AnchorPeriod submits as amount_billed.
//
// A caller that has to supply amount_settled alongside it needs to know
// what the billed amount is: statement_registry rejects a statement
// whose settled amount exceeds its billed amount, and "everything
// billed has been collected" is only a statement about an amount the
// caller knows.
func (a *Anchorer) BilledAmount(ctx context.Context, periodID int64) (*big.Int, error) {
	period, err := a.fetchPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	usg, err := a.computeUsage(ctx, period, periodID)
	if err != nil {
		return nil, err
	}
	return usg.amountBilled, nil
}

// AnchorResult is what AnchorPeriod and ReconcileAnchoring produce for a
// period that ends up anchored (by this call or a prior one).
type AnchorResult struct {
	PeriodID     int64
	StatementSeq uint64
	TxHash       string
	UsageRoot    [32]byte
	RequestCount uint64
	AmountBilled *big.Int
}

// AnchorCandidate is a closed period that has not been anchored yet, with
// the fields a caller needs to work out what to submit for it: which
// statement it becomes, and the two arguments AnchorPeriod deliberately
// does not derive on its own — the token it is denominated in and how
// much of it has already settled (see AnchorPeriod's doc comment).
type AnchorCandidate struct {
	PeriodID     int64
	Operator     string
	Consumer     string
	Protocol     string // matches httpapi's Protocol* constants: x402 | mpp_charge | mpp_session
	Channel      string // non-empty iff Protocol == "mpp_session"
	PeriodStart  uint32
	PeriodEnd    uint32
	PriceVersion uint32
}

const findDueForAnchoringSQL = `
SELECT id, operator, consumer, protocol, channel, price_version, period_start, period_end
FROM periods
WHERE status = 'closed'
ORDER BY id`

// DueForAnchoring returns every closed period that has not been anchored
// yet, oldest first. A period becomes a candidate the moment it is
// closed, and stops being one the moment it is anchored — the same
// status transition AnchorPeriod itself performs.
//
// This is the list a caller drives anchoring from: AnchorPeriod needs a
// period ID plus the token and settled amount, and the latter two are
// exactly the fields it cannot work out for itself, so the caller needs
// to see the period's protocol and channel to resolve them.
func (a *Anchorer) DueForAnchoring(ctx context.Context) ([]AnchorCandidate, error) {
	rows, err := a.pool.Query(ctx, findDueForAnchoringSQL)
	if err != nil {
		return nil, fmt.Errorf("meter: anchor: find periods due for anchoring: %w", err)
	}
	defer rows.Close()

	var out []AnchorCandidate
	for rows.Next() {
		var c AnchorCandidate
		var channel *string
		var priceVersion, periodStart, periodEnd int32
		if err := rows.Scan(&c.PeriodID, &c.Operator, &c.Consumer, &c.Protocol, &channel, &priceVersion, &periodStart, &periodEnd); err != nil {
			return nil, fmt.Errorf("meter: anchor: find periods due for anchoring: scan: %w", err)
		}
		c.Channel = derefOrEmpty(channel)
		c.PriceVersion = uint32(priceVersion)
		c.PeriodStart = uint32(periodStart)
		c.PeriodEnd = uint32(periodEnd)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meter: anchor: find periods due for anchoring: %w", err)
	}
	return out, nil
}

const findUnresolvedAttemptsSQL = `
SELECT DISTINCT period_id FROM anchor_attempts WHERE status = 'submitting' ORDER BY period_id`

// UnresolvedAttempts returns the IDs of periods with an anchor attempt
// still marked 'submitting' — an attempt whose outcome is unknown
// because the process died between recording the intent and recording
// the result.
//
// §6's ordering rule 3 requires these to be reconciled against the chain
// before anything else happens after a restart: an attempt that actually
// landed must not be retried into a second statement, and one that did
// not must not block a fresh attempt forever.
func (a *Anchorer) UnresolvedAttempts(ctx context.Context) ([]int64, error) {
	rows, err := a.pool.Query(ctx, findUnresolvedAttemptsSQL)
	if err != nil {
		return nil, fmt.Errorf("meter: anchor: find unresolved attempts: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("meter: anchor: find unresolved attempts: scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meter: anchor: find unresolved attempts: %w", err)
	}
	return out, nil
}

// AnchorPeriod builds the merkle usage root for periodID's metered
// requests — byte-for-byte the same leaf format statement_registry's own
// verify_usage checks (§4; internal/merkle is the single source of truth
// for it) — anchors it on chain, then marks the period 'anchored' with
// the resulting statement sequence.
//
// Idempotent by construction, the same posture internal/settle.Submit
// takes (§6 ordering rule 4): if periodID is already 'anchored', its
// recorded statement_seq is returned directly, no chain call. If an
// anchor_attempts row is already 'submitting' for it, AnchorPeriod
// refuses with ErrAnchorInFlight rather than guessing whether it landed —
// call ReconcileAnchoring first. If one is already 'confirmed' but the
// period itself wasn't updated yet (a narrow crash window between the two
// writes), this finishes that update locally, with no new chain call.
//
// Otherwise: an anchor_attempts row is written and durably committed
// before the chain call (ordering rule 3), then the call is made. A
// permanent failure (isPermanentAnchorError — a genuine contract-level
// rejection) marks the attempt 'failed' and returns the error; the period
// stays 'closed', so a fresh AnchorPeriod call later starts a new
// attempt rather than being blocked. A transient failure leaves the
// attempt 'submitting' — there is no way to know from here whether it
// actually reached the chain — for ReconcileAnchoring to resolve later.
//
// token and amountSettled are supplied by the caller rather than derived
// here: neither is tracked anywhere this package can read on its own —
// see this function's earlier revision history for the full reasoning
// (requests has no token column; how much of a period has actually
// settled is protocol-specific).
func (a *Anchorer) AnchorPeriod(
	ctx context.Context, periodID int64, signer *keypair.Full, registry AnchorSubmitter,
	token string, amountSettled *big.Int,
) (*AnchorResult, error) {
	period, err := a.fetchPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}

	if PeriodStatus(period.status) == PeriodStatusAnchored {
		if period.statementSeq == nil {
			return nil, fmt.Errorf("meter: anchor period %d: status is anchored but statement_seq is unset", periodID)
		}
		return &AnchorResult{PeriodID: periodID, StatementSeq: *period.statementSeq}, nil
	}
	if PeriodStatus(period.status) != PeriodStatusClosed {
		return nil, fmt.Errorf("meter: anchor period %d: %w (status is %q)", periodID, ErrPeriodNotClosed, period.status)
	}
	if period.periodEnd == nil {
		// Unreachable given periods' own periods_closed_fields CHECK
		// constraint — guarded anyway rather than dereferencing nil below.
		return nil, fmt.Errorf("meter: anchor period %d: status is closed but period_end is unset", periodID)
	}

	if active, err := a.findActiveAttempt(ctx, periodID); err != nil {
		return nil, err
	} else if active != nil {
		if AnchorStatus(active.status) == AnchorStatusSubmitting {
			return nil, fmt.Errorf("meter: anchor period %d: %w", periodID, ErrAnchorInFlight)
		}
		// confirmed, but the period wasn't updated yet — finish locally.
		if active.statementSeq == nil {
			return nil, fmt.Errorf("meter: anchor period %d: attempt %d is confirmed but has no statement_seq", periodID, active.id)
		}
		if err := a.markPeriodAnchored(ctx, periodID, *active.statementSeq); err != nil {
			return nil, err
		}
		result := &AnchorResult{PeriodID: periodID, StatementSeq: *active.statementSeq}
		if active.txHash != nil {
			result.TxHash = *active.txHash
		}
		return result, nil
	}

	usg, err := a.computeUsage(ctx, period, periodID)
	if err != nil {
		return nil, err
	}

	protocol, err := protocolFor(period.protocol)
	if err != nil {
		return nil, fmt.Errorf("meter: anchor period %d: %w", periodID, err)
	}

	attemptID, err := a.recordAttempt(ctx, periodID)
	if err != nil {
		return nil, err
	}

	seq, hash, err := registry.Anchor(ctx, signer, period.consumer, period.periodStart, *period.periodEnd,
		usg.tree.Root, usg.requestCount, token, usg.amountBilled, amountSettled, period.priceVersion, protocol, period.channel)
	if err != nil {
		if isPermanentAnchorError(err) {
			if markErr := a.markAttemptFailed(ctx, attemptID, err.Error()); markErr != nil {
				return nil, fmt.Errorf("meter: anchor period %d: send failed (%w) and marking the attempt failed also failed: %w",
					periodID, err, markErr)
			}
			return nil, fmt.Errorf("meter: anchor period %d: %w", periodID, err)
		}
		// Transient: the attempt stays 'submitting'. Whether this actually
		// reached the chain is genuinely unknown from here.
		return nil, fmt.Errorf("meter: anchor period %d: %w", periodID, err)
	}

	if err := a.markAttemptConfirmed(ctx, attemptID, seq, hash); err != nil {
		return nil, fmt.Errorf("meter: anchor period %d: anchored on chain (seq %d, tx %s) but failed to record the attempt: %w",
			periodID, seq, hash, err)
	}
	if err := a.markPeriodAnchored(ctx, periodID, seq); err != nil {
		return nil, fmt.Errorf("meter: anchor period %d: anchored on chain (seq %d, tx %s) and recorded the attempt, but failed to update the period: %w",
			periodID, seq, hash, err)
	}

	return &AnchorResult{
		PeriodID: periodID, StatementSeq: seq, TxHash: hash,
		UsageRoot: usg.tree.Root, RequestCount: usg.requestCount, AmountBilled: usg.amountBilled,
	}, nil
}

// ReconcileResult is ReconcileAnchoring's outcome for one period.
type ReconcileResult struct {
	PeriodID int64
	// Resolved is true if an unresolved ('submitting') attempt was found
	// and resolved, either way. False means there was nothing to
	// reconcile — no 'submitting' attempt existed.
	Resolved bool
	// Anchored is true if the attempt turned out to have already landed
	// on chain (periods.status is now 'anchored'). False (with Resolved
	// true) means no matching statement was found and the attempt was
	// marked 'failed', freeing the period for a fresh AnchorPeriod call.
	Anchored     bool
	StatementSeq uint64 // set iff Anchored
}

// ReconcileAnchoring resolves periodID's unresolved anchor attempt, if
// any, against the chain's real state — §6 ordering rule 3's pattern
// ("on restart, reconcile any submitting row against the chain before
// doing anything else"), applied to anchoring the way
// internal/settle.Submitter.ReconcileSubmitting applies it to settlements.
//
// Unlike settle, there is no live getter to check a known key against —
// statement_registry.anchor's return value, the seq, is only known once
// the call has already succeeded, so there's nothing to look up directly
// the way Submitter checks a channel's live Withdrawn total. Instead:
// list_statements(operator, consumer) returns every statement seq that
// exists for this scope, and each candidate is fetched via get_statement
// and compared, by content — period_start, period_end, and a freshly
// recomputed usage_root (computeUsage, deterministic from requests, which
// don't change once a period is closed) — against what this attempt would
// have submitted. A match means it landed before the crash; exhausting
// every candidate with no match means it didn't.
func (a *Anchorer) ReconcileAnchoring(ctx context.Context, periodID int64, statements StatementReader) (*ReconcileResult, error) {
	period, err := a.fetchPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	if period.periodEnd == nil {
		return nil, fmt.Errorf("meter: reconcile anchoring for period %d: period_end is unset", periodID)
	}

	active, err := a.findActiveAttempt(ctx, periodID)
	if err != nil {
		return nil, err
	}
	if active == nil || active.status != "submitting" {
		return &ReconcileResult{PeriodID: periodID}, nil
	}

	usg, err := a.computeUsage(ctx, period, periodID)
	if err != nil {
		return nil, err
	}

	seqs, err := statements.ListStatements(ctx, period.operator, period.consumer)
	if err != nil {
		return nil, fmt.Errorf("meter: reconcile anchoring for period %d: list statements: %w", periodID, err)
	}

	for _, seq := range seqs {
		stmt, err := statements.GetStatement(ctx, period.operator, seq)
		if err != nil {
			// One candidate failing to read must not abort the whole
			// reconciliation — try the rest; a later reconcile call can
			// retry this one.
			continue
		}
		if stmt.PeriodStart != period.periodStart || stmt.PeriodEnd != *period.periodEnd || stmt.UsageRoot != usg.tree.Root {
			continue
		}

		if err := a.markAttemptConfirmed(ctx, active.id, seq, ""); err != nil {
			return nil, fmt.Errorf("meter: reconcile anchoring for period %d: %w", periodID, err)
		}
		if err := a.markPeriodAnchored(ctx, periodID, seq); err != nil {
			return nil, fmt.Errorf("meter: reconcile anchoring for period %d: %w", periodID, err)
		}
		return &ReconcileResult{PeriodID: periodID, Resolved: true, Anchored: true, StatementSeq: seq}, nil
	}

	reason := "reconciled after restart: no matching statement found on chain"
	if err := a.markAttemptFailed(ctx, active.id, reason); err != nil {
		return nil, fmt.Errorf("meter: reconcile anchoring for period %d: %w", periodID, err)
	}
	return &ReconcileResult{PeriodID: periodID, Resolved: true, Anchored: false}, nil
}
