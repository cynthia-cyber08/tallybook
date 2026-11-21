package meter

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"testing"
)

func mustSchedule(t *testing.T, raw string) *Schedule {
	t.Helper()
	sched, err := ParseSchedule([]byte(raw))
	if err != nil {
		t.Fatalf("ParseSchedule returned unexpected error: %v", err)
	}
	return sched
}

func TestNewCatalog_Rejects(t *testing.T) {
	sched := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`)

	tests := []struct {
		name     string
		versions []CatalogVersion
		wantErr  error
	}{
		{
			name:     "empty",
			versions: nil,
			wantErr:  ErrEmptyCatalog,
		},
		{
			name:     "nil schedule",
			versions: []CatalogVersion{{Version: 1, EffectiveLedger: 100, Schedule: nil}},
			wantErr:  ErrNilSchedule,
		},
		{
			name: "duplicate version number",
			versions: []CatalogVersion{
				{Version: 1, EffectiveLedger: 100, Schedule: sched},
				{Version: 1, EffectiveLedger: 200, Schedule: sched},
			},
			wantErr: ErrDuplicateVersion,
		},
		{
			name: "duplicate effective ledger",
			versions: []CatalogVersion{
				{Version: 1, EffectiveLedger: 100, Schedule: sched},
				{Version: 2, EffectiveLedger: 100, Schedule: sched},
			},
			wantErr: ErrDuplicateVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCatalog(tt.versions...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewCatalog() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// ExampleNewCatalog shows both halves of NewCatalog's contract: entries are
// validated up front (non-empty, no entry with a nil Schedule, no two
// entries sharing a version number or effective ledger), and the rejection
// wraps the exact sentinel for the failure, so a caller acts on it with
// errors.Is rather than by comparing the message. Entry order does not
// matter — NewCatalog sorts by EffectiveLedger internally.
func ExampleNewCatalog() {
	schedule, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`))
	if err != nil {
		panic(err)
	}

	// Accepted input: one version with a parsed schedule.
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedule})
	fmt.Println("accepted:", err == nil && catalog != nil)

	// Rejected input: an entry with no schedule at all.
	_, err = NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000})
	fmt.Println("rejected:", errors.Is(err, ErrNilSchedule))

	// Output:
	// accepted: true
	// rejected: true
}

// ExampleCatalogVersion shows how a caller fills in a CatalogVersion. All
// three fields are the caller's to set — the version number and the
// effective ledger price_book published for it, and the Schedule parsed
// from that version's own document — because a CatalogVersion is input to
// NewCatalog, never an output of this package. The zero value sets none of
// them, and NewCatalog rejects it with ErrNilSchedule rather than accepting
// a version that could never price anything.
func ExampleCatalogVersion() {
	schedule, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1500"}]}`))
	if err != nil {
		panic(err)
	}

	// A caller sets every field. The version number and effective ledger
	// come from the price_book publish event (§4); the schedule is parsed
	// from the document that version's uri pointed at.
	entry := CatalogVersion{
		Version:         1,
		EffectiveLedger: 1000,
		Schedule:        schedule,
	}
	fmt.Println("version:", entry.Version)
	fmt.Println("effective ledger:", entry.EffectiveLedger)
	fmt.Println("schedule set:", entry.Schedule != nil)

	// The zero value sets none of them, and is rejected outright.
	_, err = NewCatalog(CatalogVersion{})
	fmt.Println("zero value rejected:", errors.Is(err, ErrNilSchedule))

	// Output:
	// version: 1
	// effective ledger: 1000
	// schedule set: true
	// zero value rejected: true
}

// ExampleCatalog shows how a caller obtains a Catalog and reads it. Its
// one field, versions, is unexported — NewCatalog validates and sorts the
// caller-supplied CatalogVersion entries and is the only way to get a
// working Catalog — so a caller sets no Catalog fields directly. VersionAt
// is the read path: a local mirror of price_book.version_at (§4) that
// resolves an observation ledger to the version whose schedule prices it,
// with no chain round trip. The zero value has no versions at all.
func ExampleCatalog() {
	schedV1, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`))
	if err != nil {
		panic(err)
	}
	schedV2, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"200"}]}`))
	if err != nil {
		panic(err)
	}

	// How a caller builds it: every published version, in any order —
	// NewCatalog sorts by EffectiveLedger.
	catalog, err := NewCatalog(
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: schedV2},
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedV1},
	)
	if err != nil {
		panic(err)
	}

	// How a caller reads it: which version was effective at an
	// observation ledger. A version applies from its own EffectiveLedger
	// up to the ledger before the next one.
	v, err := catalog.VersionAt(1999)
	if err != nil {
		panic(err)
	}
	fmt.Println("ledger 1999 prices under version", v.Version)
	v, err = catalog.VersionAt(2000)
	if err != nil {
		panic(err)
	}
	fmt.Println("ledger 2000 prices under version", v.Version)

	// The zero value: no versions were supplied, and there is no exported
	// field a caller could have supplied them through.
	var zero Catalog
	fmt.Println("zero value version count:", len(zero.versions))

	// Output:
	// ledger 1999 prices under version 1
	// ledger 2000 prices under version 2
	// zero value version count: 0
}

// ExampleCatalog_EffectiveLedgerAfter shows how to find the effective ledger
// of the version immediately following a given version in a Catalog. If the
// version exists and has a successor, it returns the successor's effective
// ledger and true; if the version is unknown or is the latest one, it returns
// zero and false.
func ExampleCatalog_EffectiveLedgerAfter() {
	schedV1, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`))
	if err != nil {
		panic(err)
	}
	schedV2, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"200"}]}`))
	if err != nil {
		panic(err)
	}
	schedV3, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"300"}]}`))
	if err != nil {
		panic(err)
	}

	catalog, err := NewCatalog(
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedV1},
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: schedV2},
		CatalogVersion{Version: 3, EffectiveLedger: 3000, Schedule: schedV3},
	)
	if err != nil {
		panic(err)
	}

	// Version 1 is followed by version 2 (effective at ledger 2000).
	nextLedger, ok := catalog.EffectiveLedgerAfter(1)
	fmt.Println("after v1:", nextLedger, ok)

	// Version 3 is the latest version, so there is no successor.
	nextLedger, ok = catalog.EffectiveLedgerAfter(3)
	fmt.Println("after v3:", nextLedger, ok)

	// An unknown version returns false.
	nextLedger, ok = catalog.EffectiveLedgerAfter(99)
	fmt.Println("after unknown:", nextLedger, ok)

	// Output:
	// after v1: 2000 true
	// after v3: 0 false
	// after unknown: 0 false
}

func TestCatalog_VersionAt_PicksTheEffectiveVersion(t *testing.T) {
	schedV1 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"100"}]}`)
	schedV2 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"200"}]}`)

	// Constructed out of order deliberately — NewCatalog must sort.
	catalog, err := NewCatalog(
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: schedV2},
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedV1},
	)
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}

	v, err := catalog.VersionAt(1500)
	if err != nil {
		t.Fatalf("VersionAt(1500) returned unexpected error: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("VersionAt(1500).Version = %d, want 1", v.Version)
	}

	v, err = catalog.VersionAt(2500)
	if err != nil {
		t.Fatalf("VersionAt(2500) returned unexpected error: %v", err)
	}
	if v.Version != 2 {
		t.Errorf("VersionAt(2500).Version = %d, want 2", v.Version)
	}
}

// ExampleCatalog_VersionAt demonstrates how a caller uses VersionAt to find
// which CatalogVersion was active at a specific observation ledger, as
// well as handling the case where the ledger predates all versions.
func ExampleCatalog_VersionAt() {
	schedV1, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`))
	if err != nil {
		panic(err)
	}
	schedV2, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"200"}]}`))
	if err != nil {
		panic(err)
	}

	catalog, err := NewCatalog(
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedV1},
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: schedV2},
	)
	if err != nil {
		panic(err)
	}

	// A ledger at or after an effective ledger resolves to that version.
	v, err := catalog.VersionAt(1500)
	if err != nil {
		panic(err)
	}
	fmt.Println("ledger 1500 version:", v.Version)

	// A ledger prior to the earliest effective ledger returns ErrNoVersionAt.
	_, err = catalog.VersionAt(999)
	fmt.Println("ledger 999 error:", errors.Is(err, ErrNoVersionAt))

	// Output:
	// ledger 1500 version: 1
	// ledger 999 error: true
}

func TestCatalog_VersionAt_BeforeEarliestReturnsErrNoVersionAt(t *testing.T) {
	sched := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`)
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}
	if _, err := catalog.VersionAt(999); !errors.Is(err, ErrNoVersionAt) {
		t.Fatalf("VersionAt(999) error = %v, want ErrNoVersionAt", err)
	}
}

func TestPricer_Price(t *testing.T) {
	sched1500 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1500"}]}`)
	sched1 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1"}]}`)

	tests := []struct {
		name        string
		schedule    *Schedule
		ledger      uint32
		method      string
		path        string
		units       uint64
		wantErr     error
		wantAmount  *big.Int
		wantVersion uint32
	}{
		{
			name:        "computes charged amount",
			schedule:    sched1500,
			ledger:      1000,
			method:      "GET",
			path:        "/v1/items",
			units:       3,
			wantAmount:  big.NewInt(4500),
			wantVersion: 1,
		},
		{
			name:     "unknown endpoint",
			schedule: sched1,
			ledger:   1000,
			method:   "GET",
			path:     "/v1/nope",
			units:    1,
			wantErr:  ErrUnknownEndpoint,
		},
		{
			name:     "ledger before earliest version",
			schedule: sched1,
			ledger:   500,
			method:   "GET",
			path:     "/v1/items",
			units:    1,
			wantErr:  ErrNoVersionAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: tt.schedule})
			if err != nil {
				t.Fatalf("NewCatalog returned unexpected error: %v", err)
			}
			pricer := NewPricer(catalog)

			priced, err := pricer.Price(tt.ledger, tt.method, tt.path, tt.units)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Price error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Price returned unexpected error: %v", err)
			}
			if priced.ChargedAmount.Cmp(tt.wantAmount) != 0 {
				t.Errorf("ChargedAmount = %s, want %s", priced.ChargedAmount, tt.wantAmount)
			}
			if priced.PriceVersion != tt.wantVersion {
				t.Errorf("PriceVersion = %d, want %d", priced.PriceVersion, tt.wantVersion)
			}
		})
	}
}

// --- version-boundary coverage ---
//
// version_at's contract semantics (§4) are "which schedule applied at a
// ledger" — an inclusive lower bound, exclusive upper bound per version:
// version N applies to every ledger in [EffectiveLedger_N, EffectiveLedger_{N+1}).
// The tests below probe exactly the ledgers where that arithmetic is easiest
// to get off by one: the ledger a new version becomes effective on, the
// ledger immediately before it, and the exact ledger of the very first
// version. This is also the arithmetic anchor's PeriodSpansPriceChange
// check (§4 constraint 1) depends on — a period whose start and end ledger
// resolve to different Catalog versions is exactly the statement the
// contract will reject, so a boundary bug here would surface there too,
// on chain, as a rejected anchor rather than a local test failure.

func threeVersionCatalog(t *testing.T) *Catalog {
	t.Helper()
	schedV1 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"100"}]}`)
	schedV2 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"200"}]}`)
	schedV3 := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"300"}]}`)

	catalog, err := NewCatalog(
		CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedV1},
		CatalogVersion{Version: 2, EffectiveLedger: 2000, Schedule: schedV2},
		CatalogVersion{Version: 3, EffectiveLedger: 3000, Schedule: schedV3},
	)
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}
	return catalog
}

func TestCatalog_VersionAt_ExactBoundaryLedgersBelongToTheNewVersion(t *testing.T) {
	catalog := threeVersionCatalog(t)

	tests := []struct {
		ledger      uint32
		wantVersion uint32
	}{
		{999, 1},    // just before v1 even starts: still an error case, checked separately
		{1000, 1},   // v1's own effective ledger: belongs to v1
		{1999, 1},   // last ledger before v2 takes effect: still v1
		{2000, 2},   // v2's effective ledger: belongs to v2, not v1
		{2001, 2},   // just after v2 takes effect: still v2
		{2999, 2},   // last ledger before v3 takes effect: still v2
		{3000, 3},   // v3's effective ledger: belongs to v3
		{999999, 3}, // far beyond the last version: still v3, the latest published
	}

	for _, tt := range tests {
		if tt.ledger == 999 {
			continue // covered by TestCatalog_VersionAt_JustBeforeEarliestVersionErrors below with this exact catalog
		}
		t.Run(fmt.Sprintf("ledger_%d", tt.ledger), func(t *testing.T) {
			v, err := catalog.VersionAt(tt.ledger)
			if err != nil {
				t.Fatalf("VersionAt(%d) returned unexpected error: %v", tt.ledger, err)
			}
			if v.Version != tt.wantVersion {
				t.Errorf("VersionAt(%d).Version = %d, want %d", tt.ledger, v.Version, tt.wantVersion)
			}
		})
	}
}

func TestCatalog_VersionAt_JustBeforeEarliestVersionErrors(t *testing.T) {
	catalog := threeVersionCatalog(t)
	if _, err := catalog.VersionAt(999); !errors.Is(err, ErrNoVersionAt) {
		t.Fatalf("VersionAt(999) error = %v, want ErrNoVersionAt", err)
	}
}

// ExamplePricer_Price shows how a Pricer computes the charged amount for
// a request against a catalog. It resolves the observation ledger to the
// price schedule version that was effective at that time, looks up the
// endpoint's unit price, and multiplies it by the request unit count.
func ExamplePricer_Price() {
	schedule, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"150"}]}`))
	if err != nil {
		panic(err)
	}

	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: schedule})
	if err != nil {
		panic(err)
	}

	pricer := NewPricer(catalog)

	// Price 4 units of GET /v1/items observed at ledger 1200 under version 1.
	priced, err := pricer.Price(1200, "GET", "/v1/items", 4)
	if err != nil {
		panic(err)
	}

	fmt.Println("charged amount:", priced.ChargedAmount)
	fmt.Println("price version:", priced.PriceVersion)

	// Output:
	// charged amount: 600
	// price version: 1
}

func TestCatalog_VersionAt_SingleVersionAtItsOwnEffectiveLedger(t *testing.T) {
	sched := mustSchedule(t, `{"rules":[{"method":"GET","path_template":"/v1/x","unit_price":"1"}]}`)
	catalog, err := NewCatalog(CatalogVersion{Version: 1, EffectiveLedger: 500, Schedule: sched})
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}

	v, err := catalog.VersionAt(500)
	if err != nil {
		t.Fatalf("VersionAt(500) returned unexpected error: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("VersionAt(500).Version = %d, want 1", v.Version)
	}

	if _, err := catalog.VersionAt(499); !errors.Is(err, ErrNoVersionAt) {
		t.Fatalf("VersionAt(499) error = %v, want ErrNoVersionAt", err)
	}
}

// TestPricer_Price_ChargesTheBoundaryVersionsDifferentAmounts confirms the
// boundary isn't just picking the right version number — it's actually
// pricing against that version's own distinct schedule, not accidentally
// reusing whichever schedule happened to be loaded first.
func TestPricer_Price_ChargesTheBoundaryVersionsDifferentAmounts(t *testing.T) {
	catalog := threeVersionCatalog(t)
	pricer := NewPricer(catalog)

	beforeBoundary, err := pricer.Price(1999, "GET", "/v1/items", 1)
	if err != nil {
		t.Fatalf("Price(1999) returned unexpected error: %v", err)
	}
	if beforeBoundary.PriceVersion != 1 || beforeBoundary.ChargedAmount.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("Price(1999) = version %d, amount %s, want version 1, amount 100",
			beforeBoundary.PriceVersion, beforeBoundary.ChargedAmount)
	}

	atBoundary, err := pricer.Price(2000, "GET", "/v1/items", 1)
	if err != nil {
		t.Fatalf("Price(2000) returned unexpected error: %v", err)
	}
	if atBoundary.PriceVersion != 2 || atBoundary.ChargedAmount.Cmp(big.NewInt(200)) != 0 {
		t.Errorf("Price(2000) = version %d, amount %s, want version 2, amount 200",
			atBoundary.PriceVersion, atBoundary.ChargedAmount)
	}
}

// TestCatalog_VersionAt_DetectsAPeriodSpanningAPriceChange exercises the
// exact scenario CLAUDE.md §4 constraint 1 warns about: a billing period
// whose start and end ledger straddle a price change resolves to two
// different versions here, locally — the same mismatch
// statement_registry.anchor's PeriodSpansPriceChange check would reject on
// chain. This package does not close periods on that boundary itself (that
// is meter's period-closing responsibility, built later); this test only
// confirms the version-resolution arithmetic a period-closer would depend
// on is correct at the boundary.
func TestCatalog_VersionAt_DetectsAPeriodSpanningAPriceChange(t *testing.T) {
	catalog := threeVersionCatalog(t)

	periodStart := uint32(1500)
	periodEnd := uint32(2500)

	startVersion, err := catalog.VersionAt(periodStart)
	if err != nil {
		t.Fatalf("VersionAt(periodStart) returned unexpected error: %v", err)
	}
	endVersion, err := catalog.VersionAt(periodEnd)
	if err != nil {
		t.Fatalf("VersionAt(periodEnd) returned unexpected error: %v", err)
	}
	if startVersion.Version == endVersion.Version {
		t.Fatalf("expected periodStart and periodEnd to resolve to different versions, both resolved to %d",
			startVersion.Version)
	}

	// A period entirely within one version's range must not trip the same
	// check — this is the non-spanning control case.
	sameStart, err := catalog.VersionAt(1100)
	if err != nil {
		t.Fatalf("VersionAt(1100) returned unexpected error: %v", err)
	}
	sameEnd, err := catalog.VersionAt(1900)
	if err != nil {
		t.Fatalf("VersionAt(1900) returned unexpected error: %v", err)
	}
	if sameStart.Version != sameEnd.Version {
		t.Errorf("period [1100, 1900] should resolve to one version, got %d and %d",
			sameStart.Version, sameEnd.Version)
	}
}

// ExampleNewPricer demonstrates the NewPricer constructor and the error-sentinel
// pattern used by Pricer.Price. The accepted case shows a pricer being created
// successfully with a valid catalog; the rejected case shows Price returning an
// error when the observation ledger predates all published price-book versions,
// with the sentinel identified via errors.Is rather than string comparison.
//
// Accepted: a Pricer is created and can price requests against a valid catalog.
// Rejected: Price returns ErrNoVersionAt for ledgers before the earliest effective
// version; this error is identified with errors.Is.
func ExampleNewPricer() {
	// Accepted case: NewPricer creates a pricer backed by a valid catalog.
	sched, err := ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1"}]}`))
	if err != nil {
		fmt.Fprintln(os.Stderr, "unexpected:", err)
		return
	}
	catalog := &Catalog{versions: []CatalogVersion{
		{Version: 1, EffectiveLedger: 1000, Schedule: sched},
	}}
	pricer := NewPricer(catalog)
	_ = pricer
	fmt.Println("accepted")

	// Rejected case: Price may return ErrNoVersionAt when the observation
	// ledger predates every published version. This error must be matched
	// with errors.Is, not by comparing error strings.
	_, err = pricer.Price(500, "GET", "/v1/nope", 1)
	if err != nil {
		if errors.Is(err, ErrNoVersionAt) {
			fmt.Println("rejected")
		}
	}

	// Output:
	// accepted
	// rejected
}
