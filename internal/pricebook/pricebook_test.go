package pricebook

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tallybook-Org/tallybook/internal/meter"
	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

const testOperator = "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM"

// fakeBook is a VersionReader backed by an in-memory price book, counting
// the calls it receives so a test can assert the cache did — or did not
// — keep a request off the chain.
type fakeBook struct {
	mu sync.Mutex
	// versions maps a version number to the record the chain would hold.
	versions map[uint32]*stellar.PriceBookVersion
	// latest is what Latest reports.
	latest uint32
	// latestErr, when set, is returned by Latest.
	latestErr error
	// latestCalls counts Latest calls.
	latestCalls int
	// getCalls counts GetVersion calls.
	getCalls int
}

func newFakeBook(versions ...*stellar.PriceBookVersion) *fakeBook {
	book := &fakeBook{versions: make(map[uint32]*stellar.PriceBookVersion)}
	for _, v := range versions {
		book.versions[v.Version] = v
		if v.Version > book.latest {
			book.latest = v.Version
		}
	}
	return book
}

func (b *fakeBook) Latest(_ context.Context, _ string) (uint32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.latestCalls++
	if b.latestErr != nil {
		return 0, b.latestErr
	}
	return b.latest, nil
}

func (b *fakeBook) GetVersion(_ context.Context, _ string, version uint32) (*stellar.PriceBookVersion, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.getCalls++
	v, ok := b.versions[version]
	if !ok {
		return nil, fmt.Errorf("no such version %d", version)
	}
	return v, nil
}

func (b *fakeBook) counts() (latest, get int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.latestCalls, b.getCalls
}

// scheduleDoc is a minimal, valid price schedule document.
func scheduleDoc(unitPrice string) []byte {
	return []byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"` + unitPrice + `"}]}`)
}

// published builds the chain record for a version, with the hash its
// document actually has.
func published(version uint32, effectiveLedger uint32, doc []byte, uri string) *stellar.PriceBookVersion {
	return &stellar.PriceBookVersion{
		Operator:        testOperator,
		Version:         version,
		ScheduleHash:    meter.ScheduleHash(doc),
		URI:             uri,
		EffectiveLedger: effectiveLedger,
		PublishedLedger: effectiveLedger - 1,
	}
}

func TestSource_CatalogFor_AssemblesVerifiedSchedule(t *testing.T) {
	doc := scheduleDoc("1000")
	book := newFakeBook(published(1, 1000, doc, "https://example.com/schedule.json"))

	var fetched []string
	source, err := NewSource(Config{
		Book: book,
		Fetch: func(_ context.Context, uri string) ([]byte, error) {
			fetched = append(fetched, uri)
			return doc, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	catalog, err := source.CatalogFor(context.Background(), testOperator)
	if err != nil {
		t.Fatalf("CatalogFor returned unexpected error: %v", err)
	}

	version, err := catalog.VersionAt(1500)
	if err != nil {
		t.Fatalf("VersionAt returned unexpected error: %v", err)
	}
	if version.Version != 1 {
		t.Errorf("version = %d, want 1", version.Version)
	}
	price, ok := version.Schedule.Lookup("GET", "/v1/items")
	if !ok {
		t.Fatal("no rule for GET /v1/items")
	}
	if price.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("unit price = %s, want 1000", price)
	}
	if len(fetched) != 1 || fetched[0] != "https://example.com/schedule.json" {
		t.Errorf("fetched = %v, want the published URI once", fetched)
	}
}

func TestSource_CatalogFor_RejectsHashMismatch(t *testing.T) {
	publishedDoc := scheduleDoc("1000")
	// The chain records the hash of the document as published; what the
	// URI serves is a different document entirely.
	book := newFakeBook(published(1, 1000, publishedDoc, "https://example.com/schedule.json"))

	source, err := NewSource(Config{
		Book: book,
		Fetch: func(context.Context, string) ([]byte, error) {
			return scheduleDoc("999999"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	_, err = source.CatalogFor(context.Background(), testOperator)
	if !errors.Is(err, ErrScheduleHashMismatch) {
		t.Fatalf("CatalogFor error = %v, want ErrScheduleHashMismatch", err)
	}
}

func TestSource_CatalogFor_RejectsUnparseableSchedule(t *testing.T) {
	doc := []byte("not json")
	book := newFakeBook(published(1, 1000, doc, "https://example.com/schedule.json"))

	source, err := NewSource(Config{
		Book:  book,
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	_, err = source.CatalogFor(context.Background(), testOperator)
	if err == nil {
		t.Fatal("CatalogFor accepted a document that is not a schedule")
	}
	if !strings.Contains(err.Error(), "parse schedule") {
		t.Errorf("error = %v, want it to name the parse failure", err)
	}
}

func TestSource_CatalogFor_NoPublishedVersions(t *testing.T) {
	source, err := NewSource(Config{
		Book:  newFakeBook(),
		Fetch: func(context.Context, string) ([]byte, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	_, err = source.CatalogFor(context.Background(), testOperator)
	if !errors.Is(err, ErrNoVersions) {
		t.Fatalf("CatalogFor error = %v, want ErrNoVersions", err)
	}
}

func TestSource_CatalogFor_CachesBetweenCalls(t *testing.T) {
	doc := scheduleDoc("1000")
	book := newFakeBook(published(1, 1000, doc, "https://example.com/schedule.json"))

	fetches := 0
	source, err := NewSource(Config{
		Book: book,
		Fetch: func(context.Context, string) ([]byte, error) {
			fetches++
			return doc, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := source.CatalogFor(context.Background(), testOperator); err != nil {
			t.Fatalf("CatalogFor returned unexpected error: %v", err)
		}
	}

	if fetches != 1 {
		t.Errorf("fetched %d documents, want 1 — the catalog should be cached", fetches)
	}
	latest, get := book.counts()
	if latest != 1 {
		t.Errorf("Latest called %d times, want 1 — the catalog should be cached", latest)
	}
	if get != 1 {
		t.Errorf("GetVersion called %d times, want 1", get)
	}
}

// TestSource_Refresh_PicksUpNewVersion covers the case that matters
// most for pricing: the operator publishes a new schedule version, and
// the next refresh must include it — with the older version still in
// the catalog, since requests observed before the new version took
// effect are priced against it.
func TestSource_Refresh_PicksUpNewVersion(t *testing.T) {
	firstDoc := scheduleDoc("1000")
	secondDoc := scheduleDoc("2000")
	book := newFakeBook(published(1, 1000, firstDoc, "https://example.com/v1.json"))

	source, err := NewSource(Config{
		Book: book,
		Fetch: func(_ context.Context, uri string) ([]byte, error) {
			if uri == "https://example.com/v1.json" {
				return firstDoc, nil
			}
			return secondDoc, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	catalog, err := source.CatalogFor(context.Background(), testOperator)
	if err != nil {
		t.Fatalf("CatalogFor returned unexpected error: %v", err)
	}
	if v, err := catalog.VersionAt(1500); err != nil || v.Version != 1 {
		t.Fatalf("version at 1500 = %v, %v; want 1", v, err)
	}

	// The operator publishes version 2, effective from ledger 2000.
	book.mu.Lock()
	book.versions[2] = published(2, 2000, secondDoc, "https://example.com/v2.json")
	book.latest = 2
	book.mu.Unlock()

	catalog, err = source.Refresh(context.Background(), testOperator)
	if err != nil {
		t.Fatalf("Refresh returned unexpected error: %v", err)
	}
	if v, err := catalog.VersionAt(1500); err != nil || v.Version != 1 {
		t.Errorf("version at 1500 = %v, %v; want 1 — the old version still prices the ledgers before 2000", v, err)
	}
	v, err := catalog.VersionAt(2500)
	if err != nil {
		t.Fatalf("VersionAt(2500) returned unexpected error: %v", err)
	}
	if v.Version != 2 {
		t.Errorf("version at 2500 = %d, want 2", v.Version)
	}
	price, ok := v.Schedule.Lookup("GET", "/v1/items")
	if !ok || price.Cmp(big.NewInt(2000)) != 0 {
		t.Errorf("unit price of version 2 = %v (%v), want 2000", price, ok)
	}
}

func TestSource_Refresh_RejectsVersionGoingBackwards(t *testing.T) {
	doc := scheduleDoc("1000")
	book := newFakeBook(published(1, 1000, doc, "https://example.com/schedule.json"))

	source, err := NewSource(Config{
		Book:  book,
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}
	if _, err := source.CatalogFor(context.Background(), testOperator); err != nil {
		t.Fatalf("CatalogFor returned unexpected error: %v", err)
	}

	// The chain now reports version 2, and the source picks it up.
	book.mu.Lock()
	book.versions[2] = published(2, 2000, doc, "https://example.com/v2.json")
	book.latest = 2
	book.mu.Unlock()
	if _, err := source.Refresh(context.Background(), testOperator); err != nil {
		t.Fatalf("Refresh returned unexpected error: %v", err)
	}

	// And then reports version 1 again — a price book that un-publishes.
	// Continuing would price requests against a schedule that may no
	// longer be the operator's.
	book.mu.Lock()
	book.latest = 1
	book.mu.Unlock()
	_, err = source.Refresh(context.Background(), testOperator)
	if !errors.Is(err, ErrVersionWentBackwards) {
		t.Fatalf("Refresh error = %v, want ErrVersionWentBackwards", err)
	}
}

// TestSource_CatalogFor_RefreshesStaleCacheInBackground covers the
// staleness policy: a cached catalog is served even when it has gone
// stale, and a refresh is started behind it — a request must not become
// latency-bound on a chain read.
func TestSource_CatalogFor_RefreshesStaleCacheInBackground(t *testing.T) {
	doc := scheduleDoc("1000")
	book := newFakeBook(published(1, 1000, doc, "https://example.com/schedule.json"))

	now := time.Now()
	source, err := NewSource(Config{
		Book:  book,
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
		TTL:   time.Minute,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewSource returned unexpected error: %v", err)
	}

	if _, err := source.CatalogFor(context.Background(), testOperator); err != nil {
		t.Fatalf("CatalogFor returned unexpected error: %v", err)
	}

	// Move the clock past the TTL. The next call must still return a
	// catalog immediately — and must start a background refresh.
	now = now.Add(2 * time.Minute)
	catalog, err := source.CatalogFor(context.Background(), testOperator)
	if err != nil {
		t.Fatalf("CatalogFor returned unexpected error: %v", err)
	}
	if catalog == nil {
		t.Fatal("CatalogFor returned no catalog")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		latest, _ := book.counts()
		if latest >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stale cache was never refreshed in the background")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNewSource_RejectsMissingDependencies(t *testing.T) {
	if _, err := NewSource(Config{Fetch: func(context.Context, string) ([]byte, error) { return nil, nil }}); err == nil {
		t.Error("NewSource accepted a nil book")
	}
	if _, err := NewSource(Config{Book: newFakeBook()}); err == nil {
		t.Error("NewSource accepted a nil fetch")
	}
}

func TestHTTPFetcher_Fetch(t *testing.T) {
	doc := scheduleDoc("1000")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/huge" {
			_, _ = w.Write(make([]byte, 4096))
			return
		}
		_, _ = w.Write(doc)
	}))
	defer server.Close()

	fetcher := NewHTTPFetcher(server.Client())

	got, err := fetcher.Fetch(context.Background(), server.URL+"/schedule.json")
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if string(got) != string(doc) {
		t.Errorf("fetched %q, want %q", got, doc)
	}

	if _, err := fetcher.Fetch(context.Background(), server.URL+"/missing"); err == nil {
		t.Error("Fetch accepted a 404 response")
	}

	small := &HTTPFetcher{Client: server.Client(), MaxBytes: 10}
	if _, err := small.Fetch(context.Background(), server.URL+"/huge"); err == nil {
		t.Error("Fetch accepted a body larger than its limit")
	}
}

func TestHTTPFetcher_RejectsNonHTTPScheme(t *testing.T) {
	fetcher := NewHTTPFetcher(nil)
	for _, uri := range []string{"file:///etc/passwd", "ftp://example.com/schedule.json", "data:text/plain,hello"} {
		if _, err := fetcher.Fetch(context.Background(), uri); !errors.Is(err, ErrUnsupportedScheme) {
			t.Errorf("Fetch(%q) error = %v, want ErrUnsupportedScheme", uri, err)
		}
	}
}

// ExampleSource shows the two halves of a Source's contract: an
// operator who has published a schedule gets a catalog whose prices come
// from that schedule, and a document whose hash does not match what the
// chain recorded is refused rather than priced against.
func ExampleSource() {
	doc := []byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1000"}]}`)
	book := &fakeBook{versions: map[uint32]*stellar.PriceBookVersion{
		1: {
			Operator:        testOperator,
			Version:         1,
			ScheduleHash:    meter.ScheduleHash(doc),
			URI:             "https://example.com/schedule.json",
			EffectiveLedger: 1000,
		},
	}, latest: 1}

	source, err := NewSource(Config{
		Book:  book,
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}

	catalog, err := source.CatalogFor(context.Background(), testOperator)
	if err != nil {
		fmt.Println("catalog:", err)
		return
	}
	price, err := catalog.Price(1500, "GET", "/v1/items", 1)
	if err != nil {
		fmt.Println("price:", err)
		return
	}
	fmt.Println("charged:", price.ChargedAmount, "price version:", price.PriceVersion)

	// The same operator, but the document at the URI is not the one that
	// was published: the hash no longer matches, so it is refused.
	tampered, err := NewSource(Config{
		Book: book,
		Fetch: func(context.Context, string) ([]byte, error) {
			return []byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1"}]}`), nil
		},
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}
	_, err = tampered.CatalogFor(context.Background(), testOperator)
	fmt.Println("tampered document rejected:", errors.Is(err, ErrScheduleHashMismatch))

	// Output:
	// charged: 1000 price version: 1
	// tampered document rejected: true
}

// ExampleFetchFunc shows the seam FetchFunc's doc comment describes. A
// Source never fetches anything itself; it calls whatever FetchFunc it was
// configured with. In production that is HTTPFetcher.Fetch, passed as a
// method value. In the tests it is a plain function literal that returns a
// fixture, which is how this package's tests run without a network.
// Whichever one is used, Source still checks the returned bytes against the
// schedule hash published on chain before it trusts them.
//
// No // Output: is asserted, so `go test` compiles this example but never
// runs it. The production half wires up a fetcher that would make a real
// HTTP request, and a published example must not depend on a reachable
// endpoint. Compiling it still breaks the build if FetchFunc's signature
// changes, or if HTTPFetcher.Fetch stops satisfying it. The test half uses
// the package's existing fakeBook, published, and scheduleDoc stubs.
func ExampleFetchFunc() {
	// Production: the HTTP fetcher's method value is a FetchFunc.
	var production FetchFunc = NewHTTPFetcher(&http.Client{Timeout: DefaultHTTPTimeout}).Fetch
	_ = production

	// Tests: any function with the same shape is a FetchFunc. This one
	// serves a fixed document and still honours cancellation, as every
	// I/O seam must (CLAUDE.md §8).
	doc := scheduleDoc("1000")
	const uri = "https://example.com/schedule.json"
	var stub FetchFunc = func(ctx context.Context, got string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if got != uri {
			return nil, fmt.Errorf("unexpected uri %q", got)
		}
		return doc, nil
	}

	source, err := NewSource(Config{
		Book:  newFakeBook(published(1, 1000, doc, uri)),
		Fetch: stub,
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	catalog, err := source.CatalogFor(ctx, testOperator)
	if err != nil {
		fmt.Println("catalog:", err)
		return
	}
	price, err := catalog.Price(1500, "GET", "/v1/items", 1)
	if err != nil {
		fmt.Println("price:", err)
		return
	}
	fmt.Println("charged:", price.ChargedAmount)
}

// ExampleSource_Run shows how the collector keeps cached catalogs fresh.
// Run refreshes every operator the Source has already been asked about
// once per TTL, until ctx is cancelled. It blocks, so it runs on its own
// goroutine. A failed refresh is logged and retried on the next tick
// rather than ending the loop, which is why Run returns nothing: while the
// chain is unreachable the collector keeps pricing against the last
// schedule it read. Because published versions are immutable, that
// schedule is still one the operator really published.
//
// The book and fetcher are the package's own test stubs (newFakeBook,
// published, scheduleDoc), so nothing reaches a Soroban RPC node or an
// HTTP endpoint. There is no // Output: line on purpose: Run's only
// observable effects are a swapped cache pointer and slog lines, and when
// a tick fires relative to the deadline depends on the scheduler, so no
// output would be deterministic. Without an Output line `go test` compiles
// the example but does not run it. It is here to pin the call shape: a
// signature change breaks the build. Refresh behaviour is covered by this
// file's tests.
func ExampleSource_Run() {
	doc := scheduleDoc("1000")
	const uri = "https://example.com/schedule.json"
	source, err := NewSource(Config{
		Book:  newFakeBook(published(1, 1000, doc, uri)),
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
		TTL:   time.Minute,
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run only refreshes operators the source already knows, so ask for
	// one first, the way the metering middleware does on a request.
	if _, err := source.CatalogFor(ctx, testOperator); err != nil {
		fmt.Println("catalog:", err)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		source.Run(ctx)
	}()

	// Run returns once ctx is done: here when the timeout elapses, in
	// production on SIGINT/SIGTERM.
	<-done
}

// ExampleSource_CatalogFor shows the call the metering middleware makes on
// every request. CatalogFor serves a cached catalog without waiting on the
// chain; if the cache has gone stale it starts a background refresh and
// still returns the cached one. Only the very first call for an operator
// waits, because until the schedule arrives there is nothing to price
// against, and failing the request closed beats serving it unmetered. A
// stale catalog is still a schedule the operator really published, since
// price book versions are immutable.
//
// The book and fetcher are the package's own test stubs (newFakeBook,
// published, scheduleDoc), so nothing reaches a Soroban RPC node or an
// HTTP endpoint. There is no // Output: line on purpose: in production
// the Book and Fetch behind a Source are the chain and the schedule's
// URI, so what CatalogFor returns depends on what the operator has
// published, and a stale-cache call also races a background refresh
// goroutine against the example's return. Without an Output line
// `go test` compiles the example but does not run it. It is here to pin
// the call shape: a signature change breaks the build. Caching and
// refresh behaviour are covered by this file's tests.
func ExampleSource_CatalogFor() {
	doc := scheduleDoc("1000")
	const uri = "https://example.com/schedule.json"
	source, err := NewSource(Config{
		Book:  newFakeBook(published(1, 1000, doc, uri)),
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First call: nothing is cached yet, so this one waits on the chain.
	catalog, err := source.CatalogFor(ctx, testOperator)
	if err != nil {
		fmt.Println("catalog:", err)
		return
	}
	price, err := catalog.Price(1500, "GET", "/v1/items", 1)
	if err != nil {
		fmt.Println("price:", err)
		return
	}
	fmt.Println("charged:", price.ChargedAmount, "price version:", price.PriceVersion)

	// Later calls are served from the cache without touching the chain.
	if _, err := source.CatalogFor(ctx, testOperator); err != nil {
		fmt.Println("catalog:", err)
		return
	}
}

// ExampleSource_Refresh shows how a caller forces a reload. Refresh
// re-reads operator's catalog from the chain whatever the cache's
// freshness, stores it, and returns it. It is the same reload Run performs
// once per TTL; a caller that needs the newest published version right
// now, rather than whatever CatalogFor has cached, calls it directly.
// Unlike CatalogFor it always waits on the chain, so it belongs off the
// request path.
//
// The book and fetcher are the package's own test stubs (newFakeBook,
// published, scheduleDoc), so nothing reaches a Soroban RPC node or an
// HTTP endpoint. There is no // Output: line on purpose: in production
// the Book and Fetch behind a Source are the chain and the schedule's
// URI, so which version Refresh returns depends on what the operator has
// published by the time it runs, and no fixed output could describe
// that. Without an Output line `go test` compiles the example but does
// not run it. It is here to pin the call shape: a signature change breaks
// the build. Refresh behaviour is covered by this file's tests.
func ExampleSource_Refresh() {
	doc := scheduleDoc("1000")
	const uri = "https://example.com/schedule.json"
	source, err := NewSource(Config{
		Book:  newFakeBook(published(1, 1000, doc, uri)),
		Fetch: func(context.Context, string) ([]byte, error) { return doc, nil },
	})
	if err != nil {
		fmt.Println("new source:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	catalog, err := source.Refresh(ctx, testOperator)
	if err != nil {
		// A failed refresh leaves any cached catalog in place, so
		// CatalogFor keeps pricing against the last schedule it read.
		fmt.Println("refresh:", err)
		return
	}
	price, err := catalog.Price(1500, "GET", "/v1/items", 1)
	if err != nil {
		fmt.Println("price:", err)
		return
	}
	fmt.Println("charged:", price.ChargedAmount, "price version:", price.PriceVersion)
}
