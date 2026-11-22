package indexer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Tallybook-Org/tallybook/internal/convert"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	reconcileOperator = "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM"
	reconcileConsumer = "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY"
)

// insertPeriod inserts a minimal periods row and returns its id.
// statementSeq nil leaves the period open and un-anchored; non-nil inserts
// an already-closed, anchored period (periods' own CHECK constraints
// require period_end and closed_at together whenever status != 'open').
func insertPeriod(t *testing.T, pool *pgxpool.Pool, statementSeq *int64) int64 {
	t.Helper()
	var id int64
	var err error
	if statementSeq == nil {
		err = pool.QueryRow(context.Background(), `
			INSERT INTO periods (operator, consumer, protocol, price_version, period_start, status)
			VALUES ($1, $2, 'x402', 1, 1000, 'open')
			RETURNING id`,
			reconcileOperator, reconcileConsumer,
		).Scan(&id)
	} else {
		err = pool.QueryRow(context.Background(), `
			INSERT INTO periods (operator, consumer, protocol, price_version, period_start, period_end, status, statement_seq, closed_at)
			VALUES ($1, $2, 'x402', 1, 1000, 2000, 'anchored', $3, now())
			RETURNING id`,
			reconcileOperator, reconcileConsumer, statementSeq,
		).Scan(&id)
	}
	if err != nil {
		t.Fatalf("insert period: %v", err)
	}
	return id
}

// insertMeteredRequest inserts one requests row charging amount against
// periodID.
func insertMeteredRequest(t *testing.T, pool *pgxpool.Pool, periodID int64, requestIDByte byte, amount int64) {
	t.Helper()
	requestID := make([]byte, 32)
	requestID[0] = requestIDByte
	endpointHash := make([]byte, 32)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO requests (request_id, operator, consumer, endpoint_hash, method, path_template,
		                       unit_count, price_version, charged_amount, protocol, observed_ledger, period_id)
		VALUES ($1, $2, $3, $4, 'GET', '/v1/x', 1, 1, $5, 'x402', 1000, $6)`,
		requestID, reconcileOperator, reconcileConsumer, endpointHash, amount, periodID,
	)
	if err != nil {
		t.Fatalf("insert request: %v", err)
	}
}

// insertAnchorChainEvent inserts a synthetic statement.anchor chain_events
// row directly (as Tick itself would, after decoding a real anchor event) —
// reconciliation only reads chain_events, so exercising it this way keeps
// these tests focused on Reconciler's own SQL rather than re-deriving a
// real anchor event's XDR.
func insertAnchorChainEvent(t *testing.T, pool *pgxpool.Pool, seq int64, billed, settled *big.Int) {
	t.Helper()
	eventID := "anchor-event-" + big.NewInt(seq).String()
	billedNum := pgtype.Numeric{Int: billed, Exp: 0, Valid: true}
	settledNum := pgtype.Numeric{Int: settled, Exp: 0, Valid: true}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO chain_events (event_id, ledger, ledger_closed_at, contract_id, tx_hash, topic, kind, value_xdr,
		                           operator, consumer, seq, amount_billed, amount_settled)
		VALUES ($1, 1500, now(), 'CREGISTRY', 'txhash', ARRAY['dGVzdA=='], 'statement.anchor', 'dGVzdA==',
		        $2, $3, $4, $5, $6)`,
		eventID, reconcileOperator, reconcileConsumer, seq, billedNum, settledNum,
	)
	if err != nil {
		t.Fatalf("insert anchor chain_event: %v", err)
	}
}

// TestNumericToBigInt_HandlesPositiveExponent is a regression test: a
// first version of numericToBigInt rejected any non-zero Exp outright,
// which broke on a real SUM() result — Postgres's NUMERIC wire encoding
// represents whole numbers like 1000 as coefficient 1 with Exp=3, not
// coefficient 1000 with Exp=0. Caught by TestReconcilePeriod_MatchingAmounts
// failing against real Postgres before this function was fixed; this test
// pins the fix down at the unit level, with no database needed.
func TestNumericToBigInt_HandlesPositiveExponent(t *testing.T) {
	tests := []struct {
		name string
		n    pgtype.Numeric
		want *big.Int
	}{
		{"zero exponent", pgtype.Numeric{Int: big.NewInt(1000), Exp: 0, Valid: true}, big.NewInt(1000)},
		{"trailing-zero-compressed", pgtype.Numeric{Int: big.NewInt(1), Exp: 3, Valid: true}, big.NewInt(1000)},
		{"larger positive exponent", pgtype.Numeric{Int: big.NewInt(7), Exp: 6, Valid: true}, big.NewInt(7000000)},
		{"nil Int treated as zero", pgtype.Numeric{Int: nil, Exp: 0, Valid: true}, big.NewInt(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert.NumericToBigInt(tt.n)
			if err != nil {
				t.Fatalf("numericToBigInt returned unexpected error: %v", err)
			}
			if got.Cmp(tt.want) != 0 {
				t.Errorf("numericToBigInt(%+v) = %s, want %s", tt.n, got, tt.want)
			}
		})
	}
}

func TestNumericToBigInt_RejectsInvalid(t *testing.T) {
	if _, err := convert.NumericToBigInt(pgtype.Numeric{Valid: false}); err == nil {
		t.Error("numericToBigInt accepted a NULL (Valid=false) numeric")
	}
	if _, err := convert.NumericToBigInt(pgtype.Numeric{Int: big.NewInt(1), Exp: -2, Valid: true}); err == nil {
		t.Error("numericToBigInt accepted a negative exponent (a fractional value) for a scale-0 column")
	}
}

func TestReconcilePeriod_NotFound(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	_, err := r.ReconcilePeriod(context.Background(), 999999)
	if !errors.Is(err, ErrPeriodNotFound) {
		t.Fatalf("error = %v, want ErrPeriodNotFound", err)
	}
}

func TestReconcilePeriod_NotAnchored(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	periodID := insertPeriod(t, pool, nil)

	_, err := r.ReconcilePeriod(context.Background(), periodID)
	if !errors.Is(err, ErrPeriodNotAnchored) {
		t.Fatalf("error = %v, want ErrPeriodNotAnchored", err)
	}
}

func TestReconcilePeriod_AnchorEventNotIngestedYet(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	seq := int64(7)
	periodID := insertPeriod(t, pool, &seq)
	insertMeteredRequest(t, pool, periodID, 1, 1000)
	// Deliberately no matching chain_events row inserted.

	_, err := r.ReconcilePeriod(context.Background(), periodID)
	if !errors.Is(err, ErrAnchorEventNotIngested) {
		t.Fatalf("error = %v, want ErrAnchorEventNotIngested", err)
	}
}

func TestReconcilePeriod_MatchingAmounts(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	seq := int64(1)
	periodID := insertPeriod(t, pool, &seq)
	insertMeteredRequest(t, pool, periodID, 1, 600)
	insertMeteredRequest(t, pool, periodID, 2, 400)
	insertAnchorChainEvent(t, pool, seq, big.NewInt(1000), big.NewInt(1000))

	result, err := r.ReconcilePeriod(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ReconcilePeriod returned unexpected error: %v", err)
	}
	if result.MeteredAmount.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("MeteredAmount = %s, want 1000", result.MeteredAmount)
	}
	if !result.BilledMatch {
		t.Error("BilledMatch = false, want true")
	}
	if !result.SettledMatch {
		t.Error("SettledMatch = false, want true")
	}
	if result.Operator != reconcileOperator || result.Consumer != reconcileConsumer {
		t.Errorf("Operator/Consumer = %s/%s, want %s/%s", result.Operator, result.Consumer, reconcileOperator, reconcileConsumer)
	}
	if result.StatementSeq != uint64(seq) {
		t.Errorf("StatementSeq = %d, want %d", result.StatementSeq, seq)
	}
}

// TestReconcilePeriod_SettledMismatchOnly is the realistic case: metered
// usage matches what was billed exactly, but a dispute credited part of it
// (§4's resolve_dispute), so what actually settled is lower. This is not a
// bug — it's exactly the gap reconciliation exists to surface.
func TestReconcilePeriod_SettledMismatchOnly(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	seq := int64(2)
	periodID := insertPeriod(t, pool, &seq)
	insertMeteredRequest(t, pool, periodID, 1, 1000)
	insertAnchorChainEvent(t, pool, seq, big.NewInt(1000), big.NewInt(700)) // 300 credited via dispute

	result, err := r.ReconcilePeriod(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ReconcilePeriod returned unexpected error: %v", err)
	}
	if !result.BilledMatch {
		t.Error("BilledMatch = false, want true (metered matches what was billed)")
	}
	if result.SettledMatch {
		t.Error("SettledMatch = true, want false (700 settled != 1000 metered)")
	}
	if result.AmountSettled.Cmp(big.NewInt(700)) != 0 {
		t.Errorf("AmountSettled = %s, want 700", result.AmountSettled)
	}
}

// TestReconcilePeriod_BilledMismatch is the more serious case: the
// collector's own metered total disagrees with what was anchored as
// billed at all — a data integrity bug, not a dispute outcome.
func TestReconcilePeriod_BilledMismatch(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	seq := int64(3)
	periodID := insertPeriod(t, pool, &seq)
	insertMeteredRequest(t, pool, periodID, 1, 1500)                         // collector thinks 1500...
	insertAnchorChainEvent(t, pool, seq, big.NewInt(1000), big.NewInt(1000)) // ...but 1000 was anchored

	result, err := r.ReconcilePeriod(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ReconcilePeriod returned unexpected error: %v", err)
	}
	if result.BilledMatch {
		t.Error("BilledMatch = true, want false (1500 metered != 1000 billed)")
	}
	if result.SettledMatch {
		t.Error("SettledMatch = true, want false")
	}
}

func TestReconcilePeriod_NoRequestsStillReconciles(t *testing.T) {
	pool := testIndexerPool(t)
	r := NewReconciler(pool)

	seq := int64(4)
	periodID := insertPeriod(t, pool, &seq)
	// No requests at all for this period.
	insertAnchorChainEvent(t, pool, seq, big.NewInt(0), big.NewInt(0))

	result, err := r.ReconcilePeriod(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ReconcilePeriod returned unexpected error: %v", err)
	}
	if result.MeteredAmount.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("MeteredAmount = %s, want 0", result.MeteredAmount)
	}
	if !result.BilledMatch || !result.SettledMatch {
		t.Errorf("BilledMatch=%v SettledMatch=%v, want both true", result.BilledMatch, result.SettledMatch)
	}
}

// ExampleNewReconciler shows both halves of NewReconciler's contract.
// NewReconciler itself never fails — it has no error return and stores
// pool verbatim (see its doc comment) — so the rejection half only exists
// at the first call that reads through the pool. A cancelled context is
// refused by the pool's own acquire step before any connection is opened,
// keeping this example runnable with or without Postgres; the returned
// error wraps context.Canceled, identified with errors.Is rather than by
// comparing its message.
func ExampleNewReconciler() {
	// pgxpool.New opens no connection itself — connections are created
	// lazily on first use — so no database needs to be reachable here.
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// Accepted input: a pool to reconcile periods through.
	r := NewReconciler(pool)
	fmt.Println("accepted:", r != nil)

	// Rejected input: a context that is already cancelled. ReconcilePeriod's
	// first action is to read the period through pool, which refuses the
	// cancelled context before ever connecting.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.ReconcilePeriod(ctx, 1)
	fmt.Println("rejected:", errors.Is(err, context.Canceled))

	// Output:
	// accepted: true
	// rejected: true
}

// ExampleReconciler_ReconcilePeriod shows the call shape of
// Reconciler.ReconcilePeriod and how a caller handles each kind of result.
// The function's doc comment lists three sentinels that mean there is
// nothing to compare yet, and they call for different responses.
// ErrAnchorEventNotIngested is transient and clears after the next
// ingestion tick. ErrPeriodNotAnchored means the period has not been
// anchored on chain yet. ErrPeriodNotFound means the caller passed a bad
// id. When a Result does come back, BilledMatch and SettledMatch are
// checked separately, because Result's doc comment explains they mean
// different things. A billed mismatch is a data integrity bug. A settled
// mismatch on its own is what a resolved dispute looks like.
//
// No // Output: is asserted, so `go test` compiles this example but never
// runs it. ReconcilePeriod reads periods, requests and chain_events
// through a live Postgres pool, and a published example must not depend on
// a reachable database. Compiling it still breaks the build if
// ReconcilePeriod's signature or Result's fields change, which is the
// part worth protecting. The pool is built the same way
// ExampleNewReconciler builds one. pgxpool.New connects lazily, so
// constructing it opens nothing.
func ExampleReconciler_ReconcilePeriod() {
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		fmt.Println("open pool:", err)
		return
	}
	defer pool.Close()

	r := NewReconciler(pool)

	// Every external call gets its own timeout (CLAUDE.md §8).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const periodID int64 = 42
	res, err := r.ReconcilePeriod(ctx, periodID)
	switch {
	case errors.Is(err, ErrAnchorEventNotIngested):
		fmt.Println("anchored, but the anchor event is not indexed yet; retry after the next tick")
		return
	case errors.Is(err, ErrPeriodNotAnchored):
		fmt.Println("period not anchored on chain yet; nothing settled to compare")
		return
	case errors.Is(err, ErrPeriodNotFound):
		fmt.Println("no such period")
		return
	case err != nil:
		fmt.Println("reconcile:", err)
		return
	}

	switch {
	case !res.BilledMatch:
		// The collector's own metering disagrees with what was anchored.
		fmt.Println("integrity: metered", res.MeteredAmount, "but billed", res.AmountBilled)
	case !res.SettledMatch:
		// Billed matches; settled does not, e.g. after resolve_dispute
		// credited the consumer.
		fmt.Println("settled", res.AmountSettled, "of metered", res.MeteredAmount, "for statement", res.StatementSeq)
	default:
		fmt.Println("period", res.PeriodID, "reconciles exactly")
	}
}
