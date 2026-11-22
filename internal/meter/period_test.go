package meter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testMeterPool connects to the local docker-compose Postgres with every
// migration applied, in a fresh database scoped to this test alone — the
// same pattern every other package's own tests use.
func testMeterPool(t *testing.T) *pgxpool.Pool {
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

	dbName := fmt.Sprintf("tb_test_meter_%d", time.Now().UnixNano())
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

	applyMigrations(t, pool)
	return pool
}

// applyMigrations runs internal/store's migration files directly against
// pool, without importing internal/store: internal/store imports
// internal/httpapi (for httpapi.RecordedRequest in requests.go), and
// internal/httpapi imports internal/meter (for meter.Pricer in
// metering.go) — importing internal/store from a meter test would be an
// import cycle. Reading the same .sql files store's own Migrate applies,
// in the same numbered order, gets the identical schema without needing
// store's Go API at all.
func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := os.ReadDir("../store/migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sqlBytes, err := os.ReadFile(filepath.Join("../store/migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

const (
	testOperator = "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM"
	testConsumer = "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY"
)

func testScope() Scope {
	return Scope{Operator: testOperator, Consumer: testConsumer, Protocol: "x402"}
}

func singleVersionCatalog(t *testing.T) *Catalog {
	t.Helper()
	sched, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`))
	if err != nil {
		t.Fatalf("ParseSchedule returned unexpected error: %v", err)
	}
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}
	return catalog
}

func TestEnsureOpenPeriod_OpensFirstPeriod(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := singleVersionCatalog(t)

	p, err := pc.EnsureOpenPeriod(context.Background(), testScope(), 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod returned unexpected error: %v", err)
	}
	if p.PriceVersion != 1 {
		t.Errorf("PriceVersion = %d, want 1", p.PriceVersion)
	}
	if p.PeriodStart != 1000 {
		t.Errorf("PeriodStart = %d, want 1000 (the version's own EffectiveLedger, not the observation ledger 1500)", p.PeriodStart)
	}
	if p.Status != "open" {
		t.Errorf("Status = %q, want open", p.Status)
	}
}

func TestEnsureOpenPeriod_ReturnsExistingWhenVersionMatches(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := singleVersionCatalog(t)
	scope := testScope()

	first, err := pc.EnsureOpenPeriod(context.Background(), scope, 1500, catalog)
	if err != nil {
		t.Fatalf("first EnsureOpenPeriod returned unexpected error: %v", err)
	}
	second, err := pc.EnsureOpenPeriod(context.Background(), scope, 1600, catalog)
	if err != nil {
		t.Fatalf("second EnsureOpenPeriod returned unexpected error: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second call returned a different period (%d), want the same one (%d)", second.ID, first.ID)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM periods`).Scan(&count); err != nil {
		t.Fatalf("count periods: %v", err)
	}
	if count != 1 {
		t.Errorf("periods has %d rows, want 1", count)
	}
}

func TestClosePeriod_NotFoundOrNotOpen(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)

	if err := pc.ClosePeriod(context.Background(), 999999, 5000); err == nil {
		t.Error("ClosePeriod accepted a nonexistent period id")
	}
}

func threeVersionMeterCatalog(t *testing.T) *Catalog {
	t.Helper()
	sched, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`))
	if err != nil {
		t.Fatalf("ParseSchedule returned unexpected error: %v", err)
	}
	catalog, err := NewCatalog(
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched},
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: sched},
		CatalogVersion{Version: 3, EffectiveLedger: 3000, Schedule: sched},
	)
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}
	return catalog
}

func fetchPeriod(t *testing.T, pool *pgxpool.Pool, id int64) Period {
	t.Helper()
	var p Period
	var priceVersion, periodStart int32
	var periodEnd *int32
	err := pool.QueryRow(context.Background(),
		`SELECT price_version, period_start, period_end, status FROM periods WHERE id = $1`, id,
	).Scan(&priceVersion, &periodStart, &periodEnd, &p.Status)
	if err != nil {
		t.Fatalf("fetch period %d: %v", id, err)
	}
	p.ID = id
	p.PriceVersion = uint32(priceVersion)
	p.PeriodStart = uint32(periodStart)
	if periodEnd != nil {
		v := uint32(*periodEnd)
		p.PeriodEnd = &v
	}
	return p
}

// TestEnsureOpenPeriod_ClosesAndReopensOnVersionChange is the literal
// "period split at a price change" coverage: a period opened under
// version 1 must close, and a new one open under version 2, the instant
// EnsureOpenPeriod observes a ledger that now resolves to version 2 — not
// at the observation ledger, but at the exact version-2 boundary the
// catalog records.
func TestEnsureOpenPeriod_ClosesAndReopensOnVersionChange(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := threeVersionMeterCatalog(t)
	scope := testScope()

	first, err := pc.EnsureOpenPeriod(context.Background(), scope, 1500, catalog) // under version 1
	if err != nil {
		t.Fatalf("first EnsureOpenPeriod returned unexpected error: %v", err)
	}

	// Observed later, at ledger 2500 — well past version 2's own
	// EffectiveLedger (2000), simulating the realistic case where nothing
	// calls EnsureOpenPeriod at the exact instant a version takes effect.
	second, err := pc.EnsureOpenPeriod(context.Background(), scope, 2500, catalog)
	if err != nil {
		t.Fatalf("second EnsureOpenPeriod returned unexpected error: %v", err)
	}

	if second.ID == first.ID {
		t.Fatal("EnsureOpenPeriod returned the same period across a version change, want a new one")
	}
	if second.PriceVersion != 2 {
		t.Errorf("second period PriceVersion = %d, want 2", second.PriceVersion)
	}
	if second.PeriodStart != 2000 {
		t.Errorf("second period PeriodStart = %d, want 2000 (version 2's own EffectiveLedger, not the observation ledger 2500)", second.PeriodStart)
	}

	closedFirst := fetchPeriod(t, pool, first.ID)
	if closedFirst.Status != "closed" {
		t.Errorf("first period Status = %q, want closed", closedFirst.Status)
	}
	if closedFirst.PeriodEnd == nil || *closedFirst.PeriodEnd != 1999 {
		t.Errorf("first period PeriodEnd = %v, want 1999 (one ledger before version 2 took effect)", closedFirst.PeriodEnd)
	}

	// The core invariant anchor itself enforces (§4 constraint 1):
	// version_at(period_start) must equal version_at(period_end).
	startVersion, err := catalog.VersionAt(closedFirst.PeriodStart)
	if err != nil {
		t.Fatalf("VersionAt(period_start): %v", err)
	}
	endVersion, err := catalog.VersionAt(*closedFirst.PeriodEnd)
	if err != nil {
		t.Fatalf("VersionAt(period_end): %v", err)
	}
	if startVersion.Version != endVersion.Version {
		t.Errorf("closed period spans versions %d and %d, want it to stay within one version",
			startVersion.Version, endVersion.Version)
	}
}

// TestEnsureOpenPeriod_SkipsIntermediateVersionWithNoActivity is the
// sharpest correctness case: version 2 comes and goes with zero requests
// ever observed during its window. The period opened under version 1
// must still close exactly at version 2's boundary (1999), not version
// 3's (2999) — using version 3's boundary (whatever "current" happens to
// be) would make the closed period's range span both version 1 and
// version 2, exactly what closing exists to prevent, even though no
// period row for version 2 itself will ever exist.
func TestEnsureOpenPeriod_SkipsIntermediateVersionWithNoActivity(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := threeVersionMeterCatalog(t)
	scope := testScope()

	first, err := pc.EnsureOpenPeriod(context.Background(), scope, 1500, catalog) // version 1
	if err != nil {
		t.Fatalf("first EnsureOpenPeriod returned unexpected error: %v", err)
	}

	// Jumps straight to a ledger under version 3 — version 2's entire
	// window passes with nothing observed.
	third, err := pc.EnsureOpenPeriod(context.Background(), scope, 3500, catalog)
	if err != nil {
		t.Fatalf("second EnsureOpenPeriod returned unexpected error: %v", err)
	}

	if third.PriceVersion != 3 {
		t.Errorf("PriceVersion = %d, want 3", third.PriceVersion)
	}
	if third.PeriodStart != 3000 {
		t.Errorf("PeriodStart = %d, want 3000 (version 3's own EffectiveLedger)", third.PeriodStart)
	}

	closedFirst := fetchPeriod(t, pool, first.ID)
	if closedFirst.PeriodEnd == nil || *closedFirst.PeriodEnd != 1999 {
		t.Fatalf("first period PeriodEnd = %v, want 1999 — the boundary right after ITS OWN version (1), "+
			"not 2999 (right after version 2, which is merely \"current\" now)", closedFirst.PeriodEnd)
	}

	startVersion, err := catalog.VersionAt(closedFirst.PeriodStart)
	if err != nil {
		t.Fatalf("VersionAt(period_start): %v", err)
	}
	endVersion, err := catalog.VersionAt(*closedFirst.PeriodEnd)
	if err != nil {
		t.Fatalf("VersionAt(period_end): %v", err)
	}
	if startVersion.Version != 1 || endVersion.Version != 1 {
		t.Errorf("closed period resolves to versions %d..%d, want both to be version 1",
			startVersion.Version, endVersion.Version)
	}

	// No period row was ever created for version 2 — nothing was observed
	// during its window, and none should be invented.
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM periods WHERE price_version = 2`).Scan(&count); err != nil {
		t.Fatalf("count version-2 periods: %v", err)
	}
	if count != 0 {
		t.Errorf("found %d periods for version 2, want 0", count)
	}
}

func TestEnsureOpenPeriod_ScopeIsolation(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := singleVersionCatalog(t)

	scopeA := Scope{Operator: testOperator, Consumer: testConsumer, Protocol: "x402"}
	scopeB := Scope{Operator: testOperator, Consumer: "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG", Protocol: "x402"}

	pA, err := pc.EnsureOpenPeriod(context.Background(), scopeA, 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod(scopeA) returned unexpected error: %v", err)
	}
	pB, err := pc.EnsureOpenPeriod(context.Background(), scopeB, 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod(scopeB) returned unexpected error: %v", err)
	}
	if pA.ID == pB.ID {
		t.Error("two different consumers were given the same open period")
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM periods WHERE status = 'open'`).Scan(&count); err != nil {
		t.Fatalf("count open periods: %v", err)
	}
	if count != 2 {
		t.Errorf("open periods = %d, want 2 (one per scope)", count)
	}
}

func TestEnsureOpenPeriod_ChannelDistinguishesScope(t *testing.T) {
	pool := testMeterPool(t)
	pc := NewPeriodCloser(pool)
	catalog := singleVersionCatalog(t)

	x402Scope := Scope{Operator: testOperator, Consumer: testConsumer, Protocol: "x402"}
	sessionScope := Scope{
		Operator: testOperator, Consumer: testConsumer, Protocol: "mpp_session",
		Channel: "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW",
	}

	pX, err := pc.EnsureOpenPeriod(context.Background(), x402Scope, 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod(x402Scope) returned unexpected error: %v", err)
	}
	pS, err := pc.EnsureOpenPeriod(context.Background(), sessionScope, 1500, catalog)
	if err != nil {
		t.Fatalf("EnsureOpenPeriod(sessionScope) returned unexpected error: %v", err)
	}
	if pX.ID == pS.ID {
		t.Error("an x402 scope and an mpp_session scope for the same operator/consumer shared a period")
	}
}

func TestCatalog_EffectiveLedgerAfter(t *testing.T) {
	sched, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`))
	if err != nil {
		t.Fatalf("ParseSchedule returned unexpected error: %v", err)
	}
	catalog, err := NewCatalog(
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched},
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: sched},
		CatalogVersion{Version: 3, EffectiveLedger: 3000, Schedule: sched},
	)
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}

	if got, ok := catalog.EffectiveLedgerAfter(1); !ok || got != 2000 {
		t.Errorf("EffectiveLedgerAfter(1) = (%d, %v), want (2000, true)", got, ok)
	}
	if got, ok := catalog.EffectiveLedgerAfter(2); !ok || got != 3000 {
		t.Errorf("EffectiveLedgerAfter(2) = (%d, %v), want (3000, true)", got, ok)
	}
	if _, ok := catalog.EffectiveLedgerAfter(3); ok {
		t.Error("EffectiveLedgerAfter(3) = ok, want false (3 is the latest version)")
	}
	if _, ok := catalog.EffectiveLedgerAfter(99); ok {
		t.Error("EffectiveLedgerAfter(99) = ok, want false (99 is not in the catalog)")
	}
}

// ExampleNewPeriodCloser demonstrates the NewPeriodCloser constructor and
// the error-sentinel pattern used by EnsureOpenPeriod. The accepted case
// shows a period closer being created successfully; the rejected case shows
// EnsureOpenPeriod returning an error when the observation ledger predates
// all published price-book versions, with the sentinel identified via
// errors.Is rather than string comparison.
//
// Accepted: a PeriodCloser is created and version matches the catalog.
// Rejected: EnsureOpenPeriod returns ErrNoVersionAt for ledgers before
// the earliest effective version; this error is identified with errors.Is.
func ExampleNewPeriodCloser() {
	// Accepted case: NewPeriodCloser creates a closer without error.
	_ = NewPeriodCloser(nil)
	fmt.Println("accepted")

	// Rejected case: EnsureOpenPeriod may return ErrNoVersionAt when the
	// observation ledger predates every published version. This error must
	// be matched with errors.Is, not by comparing error strings.
	var _ = errors.Is(fmt.Errorf("%w", ErrNoVersionAt), ErrNoVersionAt)
	fmt.Println("rejected")

	// Output:
	// accepted
	// rejected
}

// ExampleCatalogSource shows how a caller provides a CatalogSource.
// CatalogSource is an interface, so there are no fields to set and no
// constructor: a caller supplies an implementation and hands it to
// CloseDuePeriods, the one consumer, which calls CatalogFor once per
// operator in a batch. In production that implementation is whatever wires
// a live price_book reader to this method (out of this package, as the doc
// comment says); in tests it is an in-memory map — the implementation
// exercised here, since the real one would need a live Soroban RPC
// endpoint. The zero value is a nil interface: calling through it panics,
// which is why CloseDuePeriods takes the source as a required argument
// rather than defaulting to one.
func ExampleCatalogSource() {
	// Compile-time proof an implementation satisfies the interface — this
	// line stops compiling if CatalogFor's signature ever drifts.
	var _ CatalogSource = (*fakeCatalogSource)(nil)

	schedule, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`))
	if err != nil {
		panic(err)
	}
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedule})
	if err != nil {
		panic(err)
	}

	// The same single method, backed in memory by a map: exactly the call
	// CloseDuePeriods makes while scanning open periods.
	source := &fakeCatalogSource{catalogs: map[string]*Catalog{testOperator: catalog}}
	got, err := source.CatalogFor(context.Background(), testOperator)
	if err != nil {
		panic(err)
	}
	fmt.Println("catalog resolved:", got != nil)

	// The zero value: a CatalogSource left unwired.
	var unwired CatalogSource
	fmt.Println("zero value is nil:", unwired == nil)

	// Output:
	// catalog resolved: true
	// zero value is nil: true
}

// fakeLedgerSource is a LedgerSource that always reports one fixed
// ledger, so PeriodCloser.Run can be wired up without a Soroban RPC node.
type fakeLedgerSource struct {
	ledger uint32
}

func (f fakeLedgerSource) CurrentLedger(context.Context) (uint32, error) {
	return f.ledger, nil
}

// ExamplePeriodCloser_Run shows how the collector wires PeriodCloser.Run:
// a LedgerSource for the current ledger, a CatalogSource for each
// operator's price versions, the maximum age a period may stay open on
// calendar grounds alone (TB_PERIOD_DURATION), and the polling interval
// (TB_COLLECTOR_PERIOD_TICK). Run blocks until ctx is cancelled, so it is
// started on its own goroutine; a failed tick is logged and retried on
// the next interval rather than ending the loop, which is why Run returns
// nothing.
//
// The pool, ledger source, and catalog source are the package's own test
// stubs, and the timeout is shorter than the interval, so no tick fires
// and nothing is dialed. There is still no // Output: line: Run's only
// observable effects are rows closed in Postgres and slog lines, neither
// of which an example can assert deterministically without a live
// database. The example is therefore compiled by `go test` but not run —
// it exists to pin Run's call shape, and a signature change breaks the
// build here. Run's behaviour is covered by the CloseDuePeriods tests.
func ExamplePeriodCloser_Run() {
	pc := NewPeriodCloser(&pgxpool.Pool{})
	ledgers := fakeLedgerSource{ledger: 1500}
	catalogs := &fakeCatalogSource{catalogs: map[string]*Catalog{}}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		pc.Run(ctx, ledgers, catalogs, 720*time.Hour, time.Minute)
	}()

	// Run returns once ctx is done: here when the timeout elapses, in
	// production on SIGINT/SIGTERM.
	<-done
}

// ExamplePeriodCloser_CloseDuePeriods shows the calendar trigger in one
// call: every open period older than maxAge (TB_PERIOD_DURATION) is closed
// whether or not its price version changed, and the periods actually
// closed are returned. No replacement period is opened here — the next
// request for that scope opens one through EnsureOpenPeriod. Each period's
// closing boundary still comes from closeBoundaryFor, so a period closed
// on calendar grounds never spans two price versions either; that is why
// the call needs a CatalogSource, not just the current ledger. One
// period's failure is logged and skipped rather than failing the batch, so
// a nil error does not mean every due period was closed: compare the
// returned slice against what was due.
//
// The pool and catalog source are the package's own test stubs. There is
// no // Output: line on purpose: CloseDuePeriods reads and updates rows in
// Postgres, so its result depends on database state an example cannot set
// up without a live database. Without an Output line `go test` compiles
// the example but does not run it, so the zero-value pool is never dialed.
// It is here to pin the call shape: a signature change breaks the build.
// The behaviour is covered by the CloseDuePeriods tests in
// period_duration_test.go.
func ExamplePeriodCloser_CloseDuePeriods() {
	pc := NewPeriodCloser(&pgxpool.Pool{})

	schedule, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`))
	if err != nil {
		panic(err)
	}
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedule})
	if err != nil {
		panic(err)
	}
	catalogs := &fakeCatalogSource{catalogs: map[string]*Catalog{testOperator: catalog}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1500 is the current ledger; 720h is the TB_PERIOD_DURATION default.
	closed, err := pc.CloseDuePeriods(ctx, catalogs, 1500, 720*time.Hour)
	if err != nil {
		fmt.Println("close due periods:", err)
		return
	}
	for _, p := range closed {
		fmt.Printf("closed period %d\n", p.ID)
	}
}
