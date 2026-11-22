package indexer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tallybook-Org/tallybook/internal/stellar"
	"github.com/Tallybook-Org/tallybook/internal/store"
)

// testIndexerPool connects to the local docker-compose Postgres with every
// migration applied, in a fresh database scoped to this test alone — the
// same pattern internal/store's and internal/custody's own tests use.
func testIndexerPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		t.Skipf("skipping: cannot reach local postgres (is `docker compose up -d` running?): %v", err)
	}
	defer adminPool.Close()
	if err := adminPool.Ping(ctx); err != nil {
		t.Skipf("skipping: cannot reach local postgres (is `docker compose up -d` running?): %v", err)
	}

	dbName := fmt.Sprintf("tb_test_indexer_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanupPool, err := pgxpool.New(cleanupCtx, "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
		if err != nil {
			return
		}
		defer cleanupPool.Close()
		_, _ = cleanupPool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+dbName)
	})

	pool, err := pgxpool.New(ctx, "postgres://tallybook:tallybook@localhost:5433/"+dbName+"?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test database %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)

	migrations, err := store.Migrations()
	if err != nil {
		t.Fatalf("store.Migrations returned unexpected error: %v", err)
	}
	if _, err := store.Migrate(context.Background(), pool, migrations); err != nil {
		t.Fatalf("store.Migrate returned unexpected error: %v", err)
	}
	return pool
}

const (
	testPriceBookID = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
	testRegistryID  = "CB75TTWGP3TLKEDGA2WOLEVCNKLUX6X5KS47GMWGBHUAVES7J55LY25M"
	testStartLedger = 4600000
)

// realPublishEvent and realAnchorEvent are genuine testnet events (the
// same fixtures internal/stellar/events_test.go verifies decode correctly
// against — see testdata/stellar/events_pricebook_publish.json and
// events_statement_registry.json), reused here rather than hand-built ScVal
// XDR, so classify+decode is exercised against the real wire format, not a
// shape this package's own author happened to construct consistently with
// itself.
var realPublishEvent = stellar.EventInfo{
	Type:           "contract",
	Ledger:         4611943,
	LedgerClosedAt: "2026-09-10T23:35:02Z",
	ContractID:     testPriceBookID,
	ID:             "0019808144356024320-0000000000",
	TxHash:         "9bf5edffad2c4dc70fa82e26b042dfc8b7d704a0054749154bfac064192fe68c",
	Topic: []string{
		"AAAADwAAAApwcmljZV9ib29rAAA=",
		"AAAADwAAAAdwdWJsaXNoAA==",
	},
	Value: "AAAAEAAAAAEAAAAEAAAAEgAAAAAAAAAAsaNmOtjSRe8VIIbpKIkNF7XkMriX3XrZPYDjkhRXbeAAAAADAAAAAgAAAA0AAAAgFa21KDOB/CtnYUenegFm4sG71uTjYQhAU6GpIxPEs9sAAAADAEh6sA==",
}

// realAnchorEvent decodes to Seq=2, Operator=GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A,
// Consumer=GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG,
// AmountBilled=2000, AmountSettled=2000, Protocol=X402 — verified via
// internal/stellar.DecodeStatementAnchorEvent directly against this exact
// value before being hardcoded here.
var realAnchorEvent = stellar.EventInfo{
	Type:           "contract",
	Ledger:         4612502,
	LedgerClosedAt: "2026-09-11T00:21:37Z",
	ContractID:     testRegistryID,
	ID:             "0019810545242755072-0000000000",
	TxHash:         "24ae654ce0eee96e50a042d1c8301b1fa22a5ee553620aca15944ef3f8ab84d7",
	Topic: []string{
		"AAAADwAAAAlzdGF0ZW1lbnQAAAA=",
		"AAAADwAAAAZhbmNob3IAAA==",
	},
	Value: "AAAAEAAAAAEAAAAHAAAAEgAAAAAAAAAAaSue9e7ifQ7u45Obpx8FFiCUrvDOYAF+ur/wm7XTZE0AAAASAAAAAAAAAACtXWITvZ/9QnTuL9KzW/xEfHokr4K/TUEhRQ7HUBTv7wAAAAUAAAAAAAAAAgAAAA0AAAAggaEkUz5nXjZ3AZgrgejf1gyEi/t9Axkzv7xLL7BNQiUAAAAKAAAAAAAAAAAAAAAAAAAH0AAAAAoAAAAAAAAAAAAAAAAAAAfQAAAAEAAAAAEAAAABAAAADwAAAARYNDAy",
}

// realUnrelatedFeeEvent is genuinely captured testnet noise (the first
// event in testdata/stellar/get_events.json): a two-segment
// Symbol("fee")+Address topic that must not match any of this package's
// eight known kinds.
var realUnrelatedFeeEvent = stellar.EventInfo{
	Type:           "contract",
	Ledger:         4611150,
	LedgerClosedAt: "2026-09-10T22:28:57Z",
	ContractID:     "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
	ID:             "0019804738446950400-0000000000",
	TxHash:         "3c5f3ab3ed8e15dbf2f82e8031eda0f85d8abb186a549f78276885474f25f365",
	Topic: []string{
		"AAAADwAAAANmZWUA",
		"AAAAEgAAAAAAAAAAA+0oimKONzC713z+Cl8orRHR8/s0JY+JMothA0fY7bY=",
	},
	Value: "AAAACgAAAAAAAAAAAAAAAAAAAGQ=",
}

type fakeEventsSource struct {
	calls   []stellar.GetEventsParams
	results []*stellar.GetEventsResult
	err     error
}

func (f *fakeEventsSource) GetEvents(_ context.Context, params stellar.GetEventsParams) (*stellar.GetEventsResult, error) {
	f.calls = append(f.calls, params)
	if f.err != nil {
		return nil, f.err
	}
	idx := len(f.calls) - 1
	if idx >= len(f.results) {
		return &stellar.GetEventsResult{}, nil
	}
	return f.results[idx], nil
}

func newTestIngestor(t *testing.T, pool *pgxpool.Pool, client EventsSource) *Ingestor {
	t.Helper()
	ix, err := New(Config{
		Client: client, Pool: pool,
		PriceBookID: testPriceBookID, StatementRegistryID: testRegistryID,
		OperatorAddress: testOperator,
		StartLedger:     testStartLedger,
	})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	return ix
}

func TestNew_Validation(t *testing.T) {
	valid := Config{
		Client: &fakeEventsSource{}, Pool: &pgxpool.Pool{},
		PriceBookID: "CB...", StatementRegistryID: "CB...", OperatorAddress: "GA...", StartLedger: 1,
	}
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"nil client", func(c *Config) { c.Client = nil }},
		{"nil pool", func(c *Config) { c.Pool = nil }},
		{"empty price book id", func(c *Config) { c.PriceBookID = "" }},
		{"empty statement registry id", func(c *Config) { c.StatementRegistryID = "" }},
		{"empty operator address", func(c *Config) { c.OperatorAddress = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if _, err := New(cfg); err == nil {
				t.Error("New accepted an invalid config")
			}
		})
	}
}

// ExampleNew shows both halves of New's contract. Per its doc comment,
// New validates cfg once at startup: a complete Config is accepted as-is,
// and a malformed one is rejected with an error wrapping ErrInvalidConfig,
// so a caller identifies the class of failure with errors.Is rather than
// by comparing its message.
//
// The pool is only stored by New, never dialed, so this example passes
// with or without Postgres running.
func ExampleNew() {
	cfg := Config{
		Client:              &fakeEventsSource{},
		Pool:                &pgxpool.Pool{},
		PriceBookID:         testPriceBookID,
		StatementRegistryID: testRegistryID,
		OperatorAddress:     testOperator,
		StartLedger:         testStartLedger,
	}

	// Accepted input: every required field is set.
	ix, err := New(cfg)
	fmt.Println("accepted:", err == nil && ix != nil)

	// Rejected input: the same config with no events source to read from.
	cfg.Client = nil
	_, err = New(cfg)
	fmt.Println("rejected:", errors.Is(err, ErrInvalidConfig))

	// Output:
	// accepted: true
	// rejected: true
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		topic   []string
		want    Kind
		wantErr bool
	}{
		{"price_book publish", realPublishEvent.Topic, KindPriceBookPublish, false},
		{"statement anchor", realAnchorEvent.Topic, KindStatementAnchor, false},
		{"unrelated fee event", realUnrelatedFeeEvent.Topic, "", true},
		{"empty topic", nil, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classify(tt.topic)
			if tt.wantErr {
				if err == nil {
					t.Fatal("classify returned nil error, want one")
				}
				return
			}
			if err != nil {
				t.Fatalf("classify returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("classify = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassify_MalformedTopicIsAnErrorNotANonMatch(t *testing.T) {
	_, err := classify([]string{"price_book", "publish"}) // plain strings, not base64 XDR
	if err == nil {
		t.Fatal("classify accepted malformed (non-base64-XDR) topic segments")
	}
	if errors.Is(err, ErrUnknownEventKind) {
		t.Error("a decode failure was reported as ErrUnknownEventKind, want a distinct decode error")
	}
}

func TestIngestor_Tick_PersistsRealEventsWithDenormalizedAnchorFields(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{results: []*stellar.GetEventsResult{
		{Events: []stellar.EventInfo{realPublishEvent, realAnchorEvent}, Cursor: "CURSOR-1"},
	}}
	ix := newTestIngestor(t, pool, client)

	n, err := ix.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("Tick persisted %d events, want 2", n)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM chain_events`).Scan(&count); err != nil {
		t.Fatalf("count chain_events: %v", err)
	}
	if count != 2 {
		t.Fatalf("chain_events has %d rows, want 2", count)
	}

	var kind string
	var operator, consumer *string
	var seq *int64
	var amountBilled, amountSettled *string
	err = pool.QueryRow(context.Background(),
		`SELECT kind, operator, consumer, seq, amount_billed::text, amount_settled::text
		 FROM chain_events WHERE event_id = $1`, realAnchorEvent.ID,
	).Scan(&kind, &operator, &consumer, &seq, &amountBilled, &amountSettled)
	if err != nil {
		t.Fatalf("query anchor row: %v", err)
	}
	if kind != string(KindStatementAnchor) {
		t.Errorf("kind = %q, want %q", kind, KindStatementAnchor)
	}
	if operator == nil || *operator != "GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A" {
		t.Errorf("operator = %v, want the decoded operator", operator)
	}
	if consumer == nil || *consumer != "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG" {
		t.Errorf("consumer = %v, want the decoded consumer", consumer)
	}
	if seq == nil || *seq != 2 {
		t.Errorf("seq = %v, want 2", seq)
	}
	if amountBilled == nil || *amountBilled != "2000" {
		t.Errorf("amount_billed = %v, want 2000", amountBilled)
	}
	if amountSettled == nil || *amountSettled != "2000" {
		t.Errorf("amount_settled = %v, want 2000", amountSettled)
	}

	// The publish row must NOT have any of the anchor-only fields set.
	var pbOperator *string
	if err := pool.QueryRow(context.Background(),
		`SELECT operator FROM chain_events WHERE event_id = $1`, realPublishEvent.ID,
	).Scan(&pbOperator); err != nil {
		t.Fatalf("query publish row: %v", err)
	}
	if pbOperator != nil {
		t.Errorf("price_book.publish row has operator = %v, want NULL", *pbOperator)
	}

	// The cursor must be durably persisted.
	var cursor string
	var ledger int
	if err := pool.QueryRow(context.Background(),
		`SELECT cursor, ledger FROM indexer_cursors WHERE name = $1`, cursorName,
	).Scan(&cursor, &ledger); err != nil {
		t.Fatalf("query cursor: %v", err)
	}
	if cursor != "CURSOR-1" {
		t.Errorf("cursor = %q, want %q", cursor, "CURSOR-1")
	}
	if ledger != int(realAnchorEvent.Ledger) {
		t.Errorf("ledger = %d, want %d (the last event's ledger)", ledger, realAnchorEvent.Ledger)
	}
}

func TestIngestor_Tick_NoEventsIsANoop(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{results: []*stellar.GetEventsResult{{Events: nil}}}
	ix := newTestIngestor(t, pool, client)

	n, err := ix.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("Tick returned %d, want 0", n)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM indexer_cursors`).Scan(&count); err != nil {
		t.Fatalf("count indexer_cursors: %v", err)
	}
	if count != 0 {
		t.Errorf("indexer_cursors has %d rows after a no-op tick, want 0", count)
	}
}

func TestIngestor_TickFailureIsVisibleAndRecoveryResetsReadiness(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{err: errors.New("rpc unavailable")}
	ix := newTestIngestor(t, pool, client)

	if _, err := ix.Tick(context.Background()); err == nil {
		t.Fatal("Tick returned nil error for a failing events source")
	}
	if got := ix.ConsecutiveTickFailures(); got != 1 {
		t.Fatalf("ConsecutiveTickFailures() = %d, want 1", got)
	}
	recorder := httptest.NewRecorder()
	ixMetrics := NewMetrics(ix)
	ixMetrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if recorder.Code != 200 {
		t.Fatalf("metrics status = %d, want 200", recorder.Code)
	}
	for _, want := range []string{
		"tallybook_indexer_tick_failures_total 1",
		"tallybook_indexer_consecutive_tick_failures 1",
	} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("metrics response does not contain %q:\n%s", want, recorder.Body.String())
		}
	}

	client.err = nil
	if _, err := ix.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned unexpected error after events source recovered: %v", err)
	}
	if got := ix.ConsecutiveTickFailures(); got != 0 {
		t.Fatalf("ConsecutiveTickFailures() after recovery = %d, want 0", got)
	}
	recorder = httptest.NewRecorder()
	ixMetrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	for _, want := range []string{
		"tallybook_indexer_tick_failures_total 1",
		"tallybook_indexer_consecutive_tick_failures 0",
		"tallybook_indexer_seconds_since_last_successful_tick",
	} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("recovered metrics response does not contain %q:\n%s", want, recorder.Body.String())
		}
	}
}

// TestIngestor_Tick_ResumesFromPersistedCursor is the literal "resume from
// cursor" coverage: the first tick has no cursor yet and must use
// StartLedger; the second tick, run against the same database (a fresh
// Ingestor value, to prove the resume point is read back from Postgres and
// not held in memory), must use the cursor the first tick persisted —
// never StartLedger again.
func TestIngestor_Tick_ResumesFromPersistedCursor(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{results: []*stellar.GetEventsResult{
		{Events: []stellar.EventInfo{realPublishEvent}, Cursor: "CURSOR-AFTER-FIRST"},
		{Events: []stellar.EventInfo{realAnchorEvent}, Cursor: "CURSOR-AFTER-SECOND"},
	}}

	first := newTestIngestor(t, pool, client)
	if _, err := first.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick returned unexpected error: %v", err)
	}

	second := newTestIngestor(t, pool, client) // deliberately a new instance
	if _, err := second.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick returned unexpected error: %v", err)
	}

	if len(client.calls) != 2 {
		t.Fatalf("GetEvents called %d times, want 2", len(client.calls))
	}
	firstCall, secondCall := client.calls[0], client.calls[1]

	if firstCall.StartLedger != testStartLedger {
		t.Errorf("first call StartLedger = %d, want %d", firstCall.StartLedger, testStartLedger)
	}
	if firstCall.Pagination != nil {
		t.Errorf("first call Pagination = %+v, want nil (no cursor yet)", firstCall.Pagination)
	}

	if secondCall.StartLedger != 0 {
		t.Errorf("second call StartLedger = %d, want 0 (must use the cursor, not a ledger)", secondCall.StartLedger)
	}
	if secondCall.Pagination == nil || secondCall.Pagination.Cursor != "CURSOR-AFTER-FIRST" {
		t.Errorf("second call Pagination = %+v, want Cursor=%q", secondCall.Pagination, "CURSOR-AFTER-FIRST")
	}

	var cursor string
	if err := pool.QueryRow(context.Background(),
		`SELECT cursor FROM indexer_cursors WHERE name = $1`, cursorName).Scan(&cursor); err != nil {
		t.Fatalf("query final cursor: %v", err)
	}
	if cursor != "CURSOR-AFTER-SECOND" {
		t.Errorf("final cursor = %q, want %q", cursor, "CURSOR-AFTER-SECOND")
	}
}

// TestIngestor_Tick_DuplicateEventIsIdempotent simulates a cursor overlap:
// the same event arrives again (as it legitimately can at a page boundary,
// or after a resume). chain_events' own UNIQUE(event_id) plus this
// package's ON CONFLICT DO NOTHING must make that a no-op, not a duplicate
// row or an error.
func TestIngestor_Tick_DuplicateEventIsIdempotent(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{results: []*stellar.GetEventsResult{
		{Events: []stellar.EventInfo{realPublishEvent}, Cursor: "CURSOR-A"},
		{Events: []stellar.EventInfo{realPublishEvent}, Cursor: "CURSOR-B"}, // same event again
	}}
	ix := newTestIngestor(t, pool, client)

	n1, err := ix.Tick(context.Background())
	if err != nil {
		t.Fatalf("first Tick returned unexpected error: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("first Tick persisted %d, want 1", n1)
	}

	n2, err := ix.Tick(context.Background())
	if err != nil {
		t.Fatalf("second Tick returned unexpected error: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("second Tick reported persisting %d, want 0 for a duplicate event", n2)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM chain_events`).Scan(&count); err != nil {
		t.Fatalf("count chain_events: %v", err)
	}
	if count != 1 {
		t.Errorf("chain_events has %d rows after a duplicate event, want 1", count)
	}
}

// TestIngestor_Tick_UnknownEventSkippedButOthersStillPersist confirms
// resilience: one event this package can't classify (real testnet noise,
// not a contrived case) must not abort the whole tick — every other event
// in the same batch is still persisted, and the cursor still advances.
func TestIngestor_Tick_UnknownEventSkippedButOthersStillPersist(t *testing.T) {
	pool := testIndexerPool(t)
	client := &fakeEventsSource{results: []*stellar.GetEventsResult{
		{Events: []stellar.EventInfo{realUnrelatedFeeEvent, realPublishEvent}, Cursor: "CURSOR-1"},
	}}
	ix := newTestIngestor(t, pool, client)

	n, err := ix.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("Tick persisted %d events, want 1 (only the classifiable one)", n)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM chain_events`).Scan(&count); err != nil {
		t.Fatalf("count chain_events: %v", err)
	}
	if count != 1 {
		t.Errorf("chain_events has %d rows, want 1", count)
	}

	// The cursor still advances past the whole batch, including the
	// skipped event — there is nothing to retry it as, since it will
	// never classify successfully.
	var cursor string
	if err := pool.QueryRow(context.Background(),
		`SELECT cursor FROM indexer_cursors WHERE name = $1`, cursorName).Scan(&cursor); err != nil {
		t.Fatalf("query cursor: %v", err)
	}
	if cursor != "CURSOR-1" {
		t.Errorf("cursor = %q, want %q", cursor, "CURSOR-1")
	}
}

func TestIngestor_Tick_GetEventsErrorPropagatesAndPersistsNothing(t *testing.T) {
	pool := testIndexerPool(t)
	boom := errors.New("rpc: getEvents: connection refused")
	client := &fakeEventsSource{err: boom}
	ix := newTestIngestor(t, pool, client)

	_, err := ix.Tick(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("Tick error = %v, want it to wrap %v", err, boom)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM chain_events`).Scan(&count); err != nil {
		t.Fatalf("count chain_events: %v", err)
	}
	if count != 0 {
		t.Errorf("chain_events has %d rows after a GetEvents failure, want 0", count)
	}
}

// Sanity check that the fixtures embedded above really do decode the way
// their comments claim, independent of this package's own classify/persist
// logic — if this ever fails, the fixtures themselves drifted, not the
// code under test.
func TestFixtures_DecodeAsDocumented(t *testing.T) {
	pb, err := stellar.DecodePriceBookPublishEvent(realPublishEvent.Value)
	if err != nil {
		t.Fatalf("decode realPublishEvent: %v", err)
	}
	if pb.Version != 2 {
		t.Errorf("realPublishEvent Version = %d, want 2", pb.Version)
	}

	an, err := stellar.DecodeStatementAnchorEvent(realAnchorEvent.Value)
	if err != nil {
		t.Fatalf("decode realAnchorEvent: %v", err)
	}
	if an.Seq != 2 {
		t.Errorf("realAnchorEvent Seq = %d, want 2", an.Seq)
	}
	if an.AmountBilled.Cmp(big.NewInt(2000)) != 0 {
		t.Errorf("realAnchorEvent AmountBilled = %s, want 2000", an.AmountBilled)
	}
}

// ExampleIngestor_Run shows how cmd/indexer drives an Ingestor: build it
// once with New, then call Run with the polling interval
// (TB_INDEXER_TICK). Run blocks until ctx is cancelled, so it is started
// on its own goroutine; a failed tick is logged and retried on the next
// interval rather than ending the loop, which is why Run returns nothing.
//
// The events source and pool are the package's own test stubs, and the
// timeout is shorter than the interval, so no tick fires and nothing is
// dialed. There is still no // Output: line: Run's only observable
// effects are rows written to Postgres and slog lines, neither of which
// an example can assert deterministically without a live database. The
// example is therefore compiled by `go test` but not run — it exists to
// pin Run's call shape, and a signature change breaks the build here.
// Tick's behaviour is covered by the ingestion and resume tests.
func ExampleIngestor_Run() {
	ix, err := New(Config{
		Client:              &fakeEventsSource{},
		Pool:                &pgxpool.Pool{},
		PriceBookID:         testPriceBookID,
		StatementRegistryID: testRegistryID,
		OperatorAddress:     testOperator,
		StartLedger:         testStartLedger,
	})
	if err != nil {
		fmt.Println("New:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ix.Run(ctx, 5*time.Second)
	}()

	// Run returns once ctx is done: here when the timeout elapses, in
	// production on SIGINT/SIGTERM.
	<-done
}
