package pricebook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// ExampleHTTPFetcher_Fetch shows the call shape of HTTPFetcher.Fetch: build
// the fetcher with NewHTTPFetcher, give the call a context with its own
// deadline, and read the schedule document back as raw bytes. The bytes are
// deliberately unverified at this layer — Source compares them against the
// schedule hash the chain recorded before pricing anything against them.
//
// Per Fetch's doc comment, its errors name the failure rather than the URI,
// so the caller adds which version it was fetching; a URI whose scheme is
// not http or https is refused with ErrUnsupportedScheme before any request
// is built.
//
// No // Output: is asserted, so `go test` compiles this example but never
// runs it. Fetch performs network I/O, and a published example must not
// depend on a listener being reachable. Compiling it still breaks the build
// if Fetch's signature changes, which is the part worth protecting. The
// server below is the same loopback httptest stub TestHTTPFetcher_Fetch
// uses, serving the package's scheduleDoc fixture.
func ExampleHTTPFetcher_Fetch() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(scheduleDoc("1000"))
	}))
	defer server.Close()

	fetcher := NewHTTPFetcher(server.Client())

	// Every external call gets its own timeout (CLAUDE.md §8);
	// DefaultHTTPTimeout is the bound the collector uses for this fetch.
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout)
	defer cancel()

	doc, err := fetcher.Fetch(ctx, server.URL+"/schedule.json")
	if err != nil {
		fmt.Println("fetch schedule version 1:", err)
		return
	}
	fmt.Printf("fetched %d bytes\n", len(doc))

	// A non-network scheme is refused outright: "published at this URI"
	// never means a local file read.
	_, err = fetcher.Fetch(ctx, "file:///etc/passwd")
	fmt.Println("file scheme refused:", errors.Is(err, ErrUnsupportedScheme))

	// A shorter deadline than DefaultHTTPTimeout is honoured too: Fetch
	// builds its request with ctx, so cancellation reaches the transport.
	short, cancelShort := context.WithTimeout(ctx, time.Millisecond)
	defer cancelShort()
	if _, err := fetcher.Fetch(short, server.URL+"/schedule.json"); err != nil {
		fmt.Println("fetch under short deadline:", err)
	}
}
