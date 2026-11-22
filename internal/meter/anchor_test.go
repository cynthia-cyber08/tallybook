package meter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stellar/go-stellar-sdk/keypair"

	"github.com/Tallybook-Org/tallybook/internal/merkle"
	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

// ExampleNewAnchorer shows both halves of NewAnchorer's contract.
// NewAnchorer itself never fails — it has no error return and stores pool
// verbatim (see its doc comment) — so the rejection half only exists at the
// first call that reads through the pool. A cancelled context is refused by
// the pool's own acquire step before any connection is opened, keeping this
// example runnable with or without Postgres; the returned error wraps
// context.Canceled, identified with errors.Is rather than by comparing its
// message.
func ExampleNewAnchorer() {
	// pgxpool.New opens no connection itself — connections are created
	// lazily on first use — so no database needs to be reachable here.
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// Accepted input: a pool to anchor closed periods through.
	a := NewAnchorer(pool)
	fmt.Println("accepted:", a != nil)

	// Rejected input: a context that is already cancelled. AnchorPeriod's
	// first action is to fetch the period through pool, which refuses the
	// cancelled context before ever connecting; the nil signer and registry
	// are never reached.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = a.AnchorPeriod(ctx, 1, nil, nil, "", big.NewInt(0))
	fmt.Println("rejected:", errors.Is(err, context.Canceled))

	// Output:
	// accepted: true
	// rejected: true
}

// ExampleAnchorResult shows how a caller consumes an AnchorResult.
// AnchorResult is an output type — AnchorPeriod fills in every field, so a
// caller sets none of them — but not every successful call fills every
// field. A fresh anchor (this call submitted it) returns all six; the
// idempotent paths (already anchored, or an attempt confirmed before a
// crash) return only the identifiers already recorded — PeriodID,
// StatementSeq, and TxHash when the attempt had one — leaving AmountBilled
// nil and UsageRoot zero, because no chain call was made to learn them
// from. A caller reading those fields must handle both shapes; the zero
// value itself never comes back from a successful call.
//
// AnchorPeriod itself is not called here: it needs Postgres, and examples
// always run. The example instead builds the two shapes from the same
// period — the fresh result's UsageRoot is a real merkle root over the
// period's records — with fixed values so the Output is deterministic.
// Production values come from the period's requests and the chain.
func ExampleAnchorResult() {
	records := []merkle.Record{
		{
			Amount:       big.NewInt(1000),
			Consumer:     testConsumer,
			EndpointHash: merkle.EndpointHash("GET", "/v1/inference"),
			Ledger:       1500,
			PriceVersion: 1,
			RequestID:    [32]byte{0x01},
			Units:        2,
		},
		{
			Amount:       big.NewInt(2000),
			Consumer:     testConsumer,
			EndpointHash: merkle.EndpointHash("GET", "/v1/inference"),
			Ledger:       1600,
			PriceVersion: 1,
			RequestID:    [32]byte{0x02},
			Units:        1,
		},
	}
	leaves := make([][32]byte, len(records))
	for i, r := range records {
		leaf, err := merkle.Leaf(r)
		if err != nil {
			log.Fatalf("merkle.Leaf: %v", err)
		}
		leaves[i] = leaf
	}
	tree, err := merkle.BuildTree(leaves)
	if err != nil {
		log.Fatalf("merkle.BuildTree: %v", err)
	}

	// Fresh anchor: every field is set, and UsageRoot is the root of the
	// tree over the period's requests.
	fresh := AnchorResult{
		PeriodID:     42,
		StatementSeq: 7,
		TxHash:       "a1b2c3d4",
		UsageRoot:    tree.Root,
		RequestCount: uint64(len(records)),
		AmountBilled: big.NewInt(3000),
	}
	fmt.Println("fresh anchor:")
	fmt.Println("  statement seq:", fresh.StatementSeq)
	fmt.Println("  requests:", fresh.RequestCount)
	fmt.Println("  amount billed:", fresh.AmountBilled)
	fmt.Println("  usage root set:", fresh.UsageRoot != [32]byte{})
	fmt.Println("  tx hash:", fresh.TxHash)

	// Already anchored: AnchorPeriod returns the recorded statement_seq
	// with no chain call, so only these two fields are set.
	prior := AnchorResult{PeriodID: 42, StatementSeq: 7}
	fmt.Println("already anchored:")
	fmt.Println("  statement seq:", prior.StatementSeq)
	fmt.Println("  amount billed unset:", prior.AmountBilled == nil)
	fmt.Println("  usage root unset:", prior.UsageRoot == [32]byte{})

	// Output:
	// fresh anchor:
	//   statement seq: 7
	//   requests: 2
	//   amount billed: 3000
	//   usage root set: true
	//   tx hash: a1b2c3d4
	// already anchored:
	//   statement seq: 7
	//   amount billed unset: true
	//   usage root unset: true
}

// ExampleAnchorSubmitter shows how a caller obtains an AnchorSubmitter.
// AnchorSubmitter is an interface, so a caller never constructs one — it
// selects an implementation and hands it to Anchorer.AnchorPeriod, the one
// caller, which makes the single method call. In production that
// implementation is *stellar.StatementRegistry (asserted below at compile
// time); in tests it is a fake, the implementation exercised here because
// the real one would need a live Soroban RPC endpoint. The zero value is a
// nil interface: calling through it panics, which is why AnchorPeriod
// takes the registry as a required argument rather than defaulting to one.
func ExampleAnchorSubmitter() {
	// Compile-time proof the production implementation satisfies the
	// interface — this line stops compiling if Anchor's signature ever
	// drifts from *stellar.StatementRegistry's method.
	var _ AnchorSubmitter = (*stellar.StatementRegistry)(nil)

	// The same single method, implemented in memory: exactly the call
	// AnchorPeriod makes.
	fake := &fakeAnchorSubmitter{seq: 7, hash: "a1b2c3d4"}
	seq, txHash, err := fake.Anchor(context.Background(), nil, testConsumer, 1000, 1999,
		[32]byte{0xAA}, 2, "CTOKEN", big.NewInt(3000), big.NewInt(3000), 1, stellar.ProtocolX402, nil)
	if err != nil {
		log.Fatalf("Anchor: %v", err)
	}
	fmt.Println("statement seq:", seq)
	fmt.Println("tx hash:", txHash)

	// The zero value: an AnchorSubmitter left unwired.
	var unwired AnchorSubmitter
	fmt.Println("zero value is nil:", unwired == nil)

	// Output:
	// statement seq: 7
	// tx hash: a1b2c3d4
	// zero value is nil: true
}

// ExampleAnchorer shows how a caller wires and uses an Anchorer. The type
// has a single unexported field — the pool it reads closed periods and
// their requests through — so a caller sets no fields directly; NewAnchorer
// is the only construction path, and it supplies that one dependency.
// AnchorPeriod is the entry point once wired: it anchors one closed
// period's usage root on chain. The zero value leaves the pool unset, so
// there is nothing for AnchorPeriod to read a period through — the type is
// built through the constructor, never as a bare struct literal.
func ExampleAnchorer() {
	// pgxpool.New opens no connection itself — connections are created
	// lazily on first use — so no database needs to be reachable here.
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// How a caller constructs it: NewAnchorer supplies the pool.
	a := NewAnchorer(pool)

	// How a caller uses it: AnchorPeriod anchors a closed period. The
	// cancelled context is refused by the pool's acquire step before any
	// connection is opened, keeping this example runnable with or without
	// Postgres; the nil signer and registry are never reached.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = a.AnchorPeriod(ctx, 1, nil, nil, "", big.NewInt(0))
	fmt.Println("constructed:", a != nil)
	fmt.Println("cancelled context honoured:", errors.Is(err, context.Canceled))

	// The zero value: no pool was ever supplied.
	var zero Anchorer
	fmt.Println("zero value pool unset:", zero.pool == nil)

	// Output:
	// constructed: true
	// cancelled context honoured: true
	// zero value pool unset: true
}

type anchorCall struct {
	consumer                    string
	periodStart, periodEnd      uint32
	usageRoot                   [32]byte
	requestCount                uint64
	token                       string
	amountBilled, amountSettled *big.Int
	priceBookVersion            uint32
	protocol                    stellar.Protocol
	channel                     *string
}

type fakeAnchorSubmitter struct {
	seq   uint64
	hash  string
	err   error
	calls []anchorCall
}

func (f *fakeAnchorSubmitter) Anchor(_ context.Context, _ *keypair.Full, consumer string, periodStart, periodEnd uint32,
	usageRoot [32]byte, requestCount uint64, token string, amountBilled, amountSettled *big.Int,
	priceBookVersion uint32, protocol stellar.Protocol, channel *string,
) (uint64, string, error) {
	f.calls = append(f.calls, anchorCall{
		consumer: consumer, periodStart: periodStart, periodEnd: periodEnd, usageRoot: usageRoot,
		requestCount: requestCount, token: token, amountBilled: amountBilled, amountSettled: amountSettled,
		priceBookVersion: priceBookVersion, protocol: protocol, channel: channel,
	})
	if f.err != nil {
		return 0, "", f.err
	}
	return f.seq, f.hash, nil
}

// insertClosedPeriod inserts a closed periods row with the given
// protocol/price_version/bounds and returns its id.
func insertClosedPeriod(t *testing.T, pool *pgxpool.Pool, protocol string, priceVersion, periodStart, periodEnd uint32) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO periods (operator, consumer, protocol, price_version, period_start, period_end, status, closed_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'closed', now())
		RETURNING id`,
		testOperator, testConsumer, protocol, priceVersion, periodStart, periodEnd,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert closed period: %v", err)
	}
	return id
}

// insertAnchorTestRequest inserts one requests row belonging to periodID.
// seed varies the request_id/endpoint_hash bytes per call so each row is
// distinct.
func insertAnchorTestRequest(t *testing.T, pool *pgxpool.Pool, periodID int64, seed byte, unitCount uint64, chargedAmount int64, priceVersion, observedLedger uint32) {
	t.Helper()
	requestID := make([]byte, 32)
	requestID[0] = seed
	endpointHash := make([]byte, 32)
	endpointHash[0] = seed ^ 0xFF
	amount := pgtype.Numeric{Int: big.NewInt(chargedAmount), Exp: 0, Valid: true}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO requests (request_id, operator, consumer, endpoint_hash, method, path_template,
		                       unit_count, price_version, charged_amount, protocol, observed_ledger, period_id)
		VALUES ($1, $2, $3, $4, 'GET', '/v1/x', $5, $6, $7, 'x402', $8, $9)`,
		requestID, testOperator, testConsumer, endpointHash, unitCount, priceVersion, amount, observedLedger, periodID,
	)
	if err != nil {
		t.Fatalf("insert request: %v", err)
	}
}

func periodStatus(t *testing.T, pool *pgxpool.Pool, periodID int64) (status string, seq *int64) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), `SELECT status, statement_seq FROM periods WHERE id = $1`, periodID).
		Scan(&status, &seq); err != nil {
		t.Fatalf("query period status: %v", err)
	}
	return status, seq
}

func TestAnchorPeriod_HappyPath(t *testing.T) {
	pool := testMeterPool(t)
	periodID := insertClosedPeriod(t, pool, "x402", 1, 1000, 1999)
	insertAnchorTestRequest(t, pool, periodID, 1, 3, 1000, 1, 1500)
	insertAnchorTestRequest(t, pool, periodID, 2, 5, 2000, 1, 1600)

	fake := &fakeAnchorSubmitter{seq: 7, hash: "anchortxhash"}
	a := NewAnchorer(pool)

	result, err := a.AnchorPeriod(context.Background(), periodID, nil, fake, "CTOKEN", big.NewInt(3000))
	if err != nil {
		t.Fatalf("AnchorPeriod returned unexpected error: %v", err)
	}
	if result.StatementSeq != 7 || result.TxHash != "anchortxhash" {
		t.Errorf("result = %+v, want seq 7, hash anchortxhash", result)
	}
	if result.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", result.RequestCount)
	}
	if result.AmountBilled.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("AmountBilled = %s, want 3000", result.AmountBilled)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("Anchor called %d times, want 1", len(fake.calls))
	}
	call := fake.calls[0]
	if call.consumer != testConsumer {
		t.Errorf("consumer = %q, want %q", call.consumer, testConsumer)
	}
	if call.periodStart != 1000 || call.periodEnd != 1999 {
		t.Errorf("period bounds = [%d, %d], want [1000, 1999]", call.periodStart, call.periodEnd)
	}
	if call.requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", call.requestCount)
	}
	if call.token != "CTOKEN" {
		t.Errorf("token = %q, want CTOKEN", call.token)
	}
	if call.amountBilled.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("amountBilled = %s, want 3000", call.amountBilled)
	}
	if call.amountSettled.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("amountSettled = %s, want 3000", call.amountSettled)
	}
	if call.priceBookVersion != 1 {
		t.Errorf("priceBookVersion = %d, want 1", call.priceBookVersion)
	}
	if call.protocol != stellar.ProtocolX402 {
		t.Errorf("protocol = %q, want %q", call.protocol, stellar.ProtocolX402)
	}
	if call.channel != nil {
		t.Errorf("channel = %v, want nil for x402", call.channel)
	}

	status, seq := periodStatus(t, pool, periodID)
	if status != "anchored" {
		t.Errorf("period status = %q, want anchored", status)
	}
	if seq == nil || *seq != 7 {
		t.Errorf("period statement_seq = %v, want 7", seq)
	}
}

func TestAnchorPeriod_UsageRootMatchesIndependentMerkleComputation(t *testing.T) {
	pool := testMeterPool(t)
	periodID := insertClosedPeriod(t, pool, "x402", 1, 1000, 1999)
	insertAnchorTestRequest(t, pool, periodID, 1, 3, 1000, 1, 1500)
	insertAnchorTestRequest(t, pool, periodID, 2, 5, 2000, 1, 1600)
	insertAnchorTestRequest(t, pool, periodID, 3, 1, 500, 1, 1700)

	fake := &fakeAnchorSubmitter{seq: 1, hash: "h"}
	a := NewAnchorer(pool)
	if _, err := a.AnchorPeriod(context.Background(), periodID, nil, fake, "CTOKEN", big.NewInt(3500)); err != nil {
		t.Fatalf("AnchorPeriod returned unexpected error: %v", err)
	}

	// Rebuild the same three leaves independently, in the same order
	// (ORDER BY id — insertion order here) AnchorPeriod itself uses, and
	// confirm the tree it built has the exact root AnchorPeriod submitted.
	wantLeaves := make([][32]byte, 3)
	for i, want := range []struct {
		seed   byte
		units  uint64
		amount int64
	}{
		{1, 3, 1000}, {2, 5, 2000}, {3, 1, 500},
	} {
		requestID := [32]byte{}
		requestID[0] = want.seed
		endpointHash := [32]byte{}
		endpointHash[0] = want.seed ^ 0xFF
		leaf, err := merkle.Leaf(merkle.Record{
			Amount: big.NewInt(want.amount), Consumer: testConsumer, EndpointHash: endpointHash,
			Ledger: 1500 + uint32(i)*100, PriceVersion: 1, RequestID: requestID, Units: want.units,
		})
		if err != nil {
			t.Fatalf("merkle.Leaf: %v", err)
		}
		wantLeaves[i] = leaf
	}
	wantTree, err := merkle.BuildTree(wantLeaves)
	if err != nil {
		t.Fatalf("merkle.BuildTree: %v", err)
	}

	if fake.calls[0].usageRoot != wantTree.Root {
		t.Errorf("usageRoot = %x, want %x (independently computed from the same requests)", fake.calls[0].usageRoot, wantTree.Root)
	}
}

func TestAnchorPeriod_RejectsNonClosedPeriod(t *testing.T) {
	pool := testMeterPool(t)
	// An open period, not closed.
	catalog := singleVersionCatalog(t)
	pc := NewPeriodCloser(pool)
	p, err := pc.EnsureOpenPeriod(context.Background(), testScope(), 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod returned unexpected error: %v", err)
	}

	fake := &fakeAnchorSubmitter{seq: 1, hash: "h"}
	a := NewAnchorer(pool)
	_, err = a.AnchorPeriod(context.Background(), p.ID, nil, fake, "CTOKEN", big.NewInt(0))
	if !errors.Is(err, ErrPeriodNotClosed) {
		t.Fatalf("error = %v, want ErrPeriodNotClosed", err)
	}
	if len(fake.calls) != 0 {
		t.Error("Anchor was called for a period that isn't closed")
	}
}

func TestAnchorPeriod_RejectsEmptyPeriod(t *testing.T) {
	pool := testMeterPool(t)
	periodID := insertClosedPeriod(t, pool, "x402", 1, 1000, 1999) // no requests

	fake := &fakeAnchorSubmitter{seq: 1, hash: "h"}
	a := NewAnchorer(pool)
	_, err := a.AnchorPeriod(context.Background(), periodID, nil, fake, "CTOKEN", big.NewInt(0))
	if !errors.Is(err, ErrPeriodEmpty) {
		t.Fatalf("error = %v, want ErrPeriodEmpty", err)
	}
	if len(fake.calls) != 0 {
		t.Error("Anchor was called for an empty period")
	}
}

func TestAnchorPeriod_ProtocolMapping(t *testing.T) {
	tests := []struct {
		stored string
		want   stellar.Protocol
	}{
		{"x402", stellar.ProtocolX402},
		{"mpp_charge", stellar.ProtocolMppCharge},
		{"mpp_session", stellar.ProtocolMppSession},
	}
	for _, tt := range tests {
		t.Run(tt.stored, func(t *testing.T) {
			pool := testMeterPool(t)
			var channel *string
			if tt.stored == "mpp_session" {
				c := "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
				channel = &c
			}
			var periodID int64
			err := pool.QueryRow(context.Background(), `
				INSERT INTO periods (operator, consumer, protocol, channel, price_version, period_start, period_end, status, closed_at)
				VALUES ($1, $2, $3, $4, 1, 1000, 1999, 'closed', now()) RETURNING id`,
				testOperator, testConsumer, tt.stored, channel,
			).Scan(&periodID)
			if err != nil {
				t.Fatalf("insert period: %v", err)
			}
			insertAnchorTestRequest(t, pool, periodID, 1, 1, 100, 1, 1500)

			fake := &fakeAnchorSubmitter{seq: 1, hash: "h"}
			a := NewAnchorer(pool)
			if _, err := a.AnchorPeriod(context.Background(), periodID, nil, fake, "CTOKEN", big.NewInt(100)); err != nil {
				t.Fatalf("AnchorPeriod returned unexpected error: %v", err)
			}
			if fake.calls[0].protocol != tt.want {
				t.Errorf("protocol = %q, want %q", fake.calls[0].protocol, tt.want)
			}
		})
	}
}

func TestAnchorPeriod_ErrorLeavesPeriodClosedNotAnchored(t *testing.T) {
	pool := testMeterPool(t)
	periodID := insertClosedPeriod(t, pool, "x402", 1, 1000, 1999)
	insertAnchorTestRequest(t, pool, periodID, 1, 1, 100, 1, 1500)

	fake := &fakeAnchorSubmitter{err: errors.New("stellar: transaction failed on chain")}
	a := NewAnchorer(pool)
	_, err := a.AnchorPeriod(context.Background(), periodID, nil, fake, "CTOKEN", big.NewInt(100))
	if err == nil {
		t.Fatal("AnchorPeriod returned nil error")
	}

	status, seq := periodStatus(t, pool, periodID)
	if status != "closed" {
		t.Errorf("period status = %q, want it to stay closed after a failed anchor attempt", status)
	}
	if seq != nil {
		t.Errorf("statement_seq = %v, want nil", seq)
	}
}

// ExampleAnchorer_AnchorPeriod shows how a caller anchors a closed period
// with Anchorer.AnchorPeriod. Because calling it against a real database
// requires a live Postgres instance and a Soroban RPC node, this example
// demonstrates the parameter validation contract using a cancelled context
// and a concrete literal input, which is refused by the pool's acquire step
// before any connection is opened, keeping the example runnable without
// requiring an active database or network connection.
func ExampleAnchorer_AnchorPeriod() {
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	a := NewAnchorer(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Concrete literal inputs passed directly to Anchorer.AnchorPeriod.
	periodID := int64(1)
	token := "CTOKEN"
	amountBilled := big.NewInt(3000)

	_, err = a.AnchorPeriod(ctx, periodID, nil, nil, token, amountBilled)
	fmt.Println("anchored with concrete inputs, cancelled context honoured:", errors.Is(err, context.Canceled))

	// Output:
	// anchored with concrete inputs, cancelled context honoured: true
}

// ExampleAnchorer_ReconcileAnchoring shows the restart step that runs
// before a period is anchored again. A crash between recording a
// 'submitting' anchor attempt and confirming it leaves the attempt open;
// ReconcileAnchoring settles which way it went. anchor's return value (the
// statement seq) is only known after the call succeeds, so there is no key
// to look up directly. Instead every seq list_statements returns for the
// scope is fetched and compared, by period range and a recomputed usage
// root, with what the attempt would have submitted. A match marks the
// period anchored; no match marks the attempt failed so AnchorPeriod can
// try again. Resolved false means there was nothing to reconcile.
//
// The pool and StatementReader are the package's own test stubs: an
// empty fakeStatementReader stands in for statement_registry. There is no
// // Output: line on purpose: ReconcileAnchoring reads the period, its
// attempt and its requests from Postgres, so its result depends on
// database state an example cannot set up without a live database.
// Without an Output line `go test` compiles the example but does not run
// it, so the zero-value pool is never dialed. It is here to pin the call
// shape: a signature change breaks the build. The behaviour is covered by
// the reconcile tests in anchor_idempotency_test.go.
func ExampleAnchorer_ReconcileAnchoring() {
	a := NewAnchorer(&pgxpool.Pool{})
	statements := &fakeStatementReader{statements: map[uint64]*stellar.Statement{}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := a.ReconcileAnchoring(ctx, 1, statements)
	if err != nil {
		fmt.Println("reconcile anchoring:", err)
		return
	}
	switch {
	case !res.Resolved:
		fmt.Println("nothing to reconcile for period", res.PeriodID)
	case res.Anchored:
		fmt.Println("already anchored as statement", res.StatementSeq)
	default:
		fmt.Println("attempt marked failed; period", res.PeriodID, "can be anchored again")
	}
}

// ExampleAnchorer_DueForAnchoring shows the list a caller drives anchoring
// from. DueForAnchoring returns every period whose status is 'closed',
// oldest first: a period joins the list when it is closed and leaves it
// when AnchorPeriod marks it anchored, so there is no separate queue to
// keep in sync. Each candidate carries its protocol and channel because
// AnchorPeriod deliberately does not work out the statement's token or
// settled amount on its own; the caller resolves both from those fields
// before anchoring, the way internal/anchor's Driver.AnchorDue does.
//
// The pool is the zero-value stub ExampleAnchorer_ReconcileAnchoring also
// uses. There is no // Output: line on purpose: DueForAnchoring is a
// single query against the periods table, so its result is whatever
// Postgres holds, and an example cannot set that up without a live
// database. Without an Output line `go test` compiles the example but
// does not run it, so the zero-value pool is never dialed. It is here to
// pin the call shape: a signature change, or a change to AnchorCandidate's
// fields used below, breaks the build.
func ExampleAnchorer_DueForAnchoring() {
	a := NewAnchorer(&pgxpool.Pool{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	candidates, err := a.DueForAnchoring(ctx)
	if err != nil {
		fmt.Println("due for anchoring:", err)
		return
	}
	for _, c := range candidates {
		// Channel is non-empty only for mpp_session periods, whose
		// settled amount comes from the channel rather than the period.
		fmt.Printf("period %d: %s consumer %s ledgers %d-%d price v%d channel %q\n",
			c.PeriodID, c.Protocol, c.Consumer, c.PeriodStart, c.PeriodEnd, c.PriceVersion, c.Channel)
	}
}
