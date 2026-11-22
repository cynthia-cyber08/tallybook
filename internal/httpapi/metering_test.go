package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/Tallybook-Org/tallybook/internal/merkle"
	"github.com/Tallybook-Org/tallybook/internal/meter"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ExampleRecorder shows how a caller actually obtains and uses a Recorder.
// Recorder is an interface (metering.go), so "constructing" one means
// implementing RecordRequest — internal/store's request-persistence type
// does this in production — and handing it to Config.Recorder; a config
// left holding the zero value (nil) is rejected at wiring time instead of
// silently recording nothing, which is the difference visible below.
//
// The caller sets only the fields it owns: the Config wiring (Operator,
// Method, PathTemplate, Catalogs, Ledger, Recorder, Periods) and the
// request's RequestInfo. UnitCount is left unset so the package's
// default (one unit per request) fills it in, and every field of the
// RecordedRequest — request ID, endpoint hash, price version, charged
// amount, observed ledger, period — is filled in by the middleware,
// never by the caller.
func ExampleRecorder() {
	// The wiring a caller is required to set, per Config.validate —
	// everything except the Recorder, which comes last so the zero-value
	// case below stays visible.
	sched, err := meter.ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1000"}]}`))
	if err != nil {
		fmt.Println("parse schedule:", err)
		return
	}
	catalog, err := meter.NewCatalog(meter.CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		fmt.Println("new catalog:", err)
		return
	}
	cfg := Config{
		Operator:     "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM",
		Method:       "GET",
		PathTemplate: "/v1/items",
		Catalogs:     fakeCatalogs{catalog: catalog},
		Ledger:       fakeLedgerSource{ledger: 1500},
		Periods:      fakePeriods{period: &meter.Period{ID: 7}},
	}

	// The zero value: no implementation to record with, so wiring fails
	// fast and names the problem rather than serving requests unmetered.
	if _, err := Middleware(cfg); err != nil {
		fmt.Println("zero value rejected:", err)
	}

	// A properly-constructed one: an implementation handed to Config.
	// RecordRequest is the only method to satisfy; the in-memory fake
	// stands in for internal/store's type, which needs a database.
	rec := &fakeRecorder{}
	cfg.Recorder = rec

	mw, err := Middleware(cfg)
	if err != nil {
		fmt.Println("middleware:", err)
		return
	}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	req := httptest.NewRequest("GET", "/v1/items", nil)
	info := RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolX402,
	}
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	mw(next).ServeHTTP(httptest.NewRecorder(), req)

	// Everything below was filled in by the package, not the caller.
	got := rec.recorded[0]
	fmt.Println("recorded:", len(rec.recorded))
	fmt.Println("operator:", got.Operator)
	fmt.Println("consumer:", got.Consumer)
	fmt.Println("endpoint:", got.Method, got.PathTemplate)
	fmt.Printf("endpoint hash: %x\n", got.EndpointHash)
	fmt.Println("units:", got.UnitCount, "(package default)")
	fmt.Println("price version:", got.PriceVersion)
	fmt.Println("charged amount:", got.ChargedAmount)
	fmt.Println("protocol:", got.Protocol)
	fmt.Println("observed ledger:", got.ObservedLedger)
	fmt.Println("period:", got.PeriodID)
	fmt.Println("request id set:", got.RequestID != [32]byte{})

	// Output:
	// zero value rejected: httpapi: metering config: recorder is nil
	// recorded: 1
	// operator: GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM
	// consumer: GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY
	// endpoint: GET /v1/items
	// endpoint hash: 79a8cbca8c21b745b034907bd0d84cad5dc580f9370b446c9f4030d4e4affb03
	// units: 1 (package default)
	// price version: 1
	// charged amount: 1000
	// protocol: x402
	// observed ledger: 1500
	// period: 7
	// request id set: true
}

type fakeLedgerSource struct {
	ledger uint32
	err    error
}

func (f fakeLedgerSource) CurrentLedger(context.Context) (uint32, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.ledger, nil
}

// fakeCatalogs is a CatalogSource that always hands back one catalog —
// or one error, for the failure paths.
type fakeCatalogs struct {
	catalog *meter.Catalog
	err     error
}

func (f fakeCatalogs) CatalogFor(context.Context, string) (*meter.Catalog, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.catalog, nil
}

// fakePeriods is a PeriodOpener that always hands back one period — or
// one error, for the failure paths.
type fakePeriods struct {
	period *meter.Period
	err    error
}

func (f fakePeriods) EnsureOpenPeriod(context.Context, meter.Scope, uint32, *meter.Catalog) (*meter.Period, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.period, nil
}

// recordingPeriods is the same idea, but a pointer receiver, so the
// scope it was called with survives the call.
type recordingPeriods struct {
	period *meter.Period
	err    error
	scope  meter.Scope
	ledger uint32
}

func (f *recordingPeriods) EnsureOpenPeriod(_ context.Context, scope meter.Scope, ledger uint32, _ *meter.Catalog) (*meter.Period, error) {
	f.scope = scope
	f.ledger = ledger
	if f.err != nil {
		return nil, f.err
	}
	return f.period, nil
}

type fakeRecorder struct {
	recorded []RecordedRequest
	err      error
}

func (f *fakeRecorder) RecordRequest(_ context.Context, req RecordedRequest) error {
	if f.err != nil {
		return f.err
	}
	f.recorded = append(f.recorded, req)
	return nil
}

func mustCatalog(t testing.TB, unitPrice string) *meter.Catalog {
	t.Helper()
	sched, err := meter.ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"` + unitPrice + `"}]}`))
	if err != nil {
		t.Fatalf("ParseSchedule returned unexpected error: %v", err)
	}
	catalog, err := meter.NewCatalog(meter.CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		t.Fatalf("NewCatalog returned unexpected error: %v", err)
	}
	return catalog
}

func baseConfig(t *testing.T, ledger LedgerSource, recorder Recorder) Config {
	t.Helper()
	return Config{
		Operator:     "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM",
		Method:       "GET",
		PathTemplate: "/v1/items",
		Catalogs:     fakeCatalogs{catalog: mustCatalog(t, "1000")},
		Ledger:       ledger,
		Recorder:     recorder,
		Periods:      fakePeriods{period: &meter.Period{ID: 42}},
	}
}

func TestMiddleware_RecordsAndCallsNext(t *testing.T) {
	recorder := &fakeRecorder{}
	cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)

	mw, err := Middleware(cfg)
	if err != nil {
		t.Fatalf("Middleware returned unexpected error: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/items", nil)
	info := RequestInfo{Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY", Protocol: ProtocolX402}
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("next was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(recorder.recorded) != 1 {
		t.Fatalf("recorded %d requests, want 1", len(recorder.recorded))
	}

	got := recorder.recorded[0]
	if got.Operator != cfg.Operator {
		t.Errorf("Operator = %q, want %q", got.Operator, cfg.Operator)
	}
	if got.Consumer != info.Consumer {
		t.Errorf("Consumer = %q, want %q", got.Consumer, info.Consumer)
	}
	if got.Method != "GET" || got.PathTemplate != "/v1/items" {
		t.Errorf("Method/PathTemplate = %q/%q, want GET//v1/items", got.Method, got.PathTemplate)
	}
	if got.UnitCount != 1 {
		t.Errorf("UnitCount = %d, want 1 (default)", got.UnitCount)
	}
	if got.PriceVersion != 1 {
		t.Errorf("PriceVersion = %d, want 1", got.PriceVersion)
	}
	if got.ChargedAmount.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("ChargedAmount = %s, want 1000", got.ChargedAmount)
	}
	if got.Protocol != ProtocolX402 {
		t.Errorf("Protocol = %q, want %q", got.Protocol, ProtocolX402)
	}
	if got.Channel != "" {
		t.Errorf("Channel = %q, want empty for x402", got.Channel)
	}
	if got.ObservedLedger != 1500 {
		t.Errorf("ObservedLedger = %d, want 1500", got.ObservedLedger)
	}
	if got.PeriodID != 42 {
		t.Errorf("PeriodID = %d, want 42", got.PeriodID)
	}
	if got.RequestID == ([32]byte{}) {
		t.Error("RequestID is all zeros, want a random value")
	}
}

func TestMiddleware_MissingRequestInfoFailsClosed(t *testing.T) {
	recorder := &fakeRecorder{}
	cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)
	mw, err := Middleware(cfg)
	if err != nil {
		t.Fatalf("Middleware returned unexpected error: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { nextCalled = true })

	req := httptest.NewRequest("GET", "/v1/items", nil) // no RequestInfo attached
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if nextCalled {
		t.Error("next was called despite missing RequestInfo")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if len(recorder.recorded) != 0 {
		t.Error("a request was recorded despite missing RequestInfo")
	}
}

// TestMiddleware_OpensThePeriodTheRequestBelongsTo covers the billing
// side of metering: every request is filed under a period, and the
// period is the one for the request's own scope — operator, consumer,
// protocol and channel — so two consumers never share a period and an
// mpp_session request is not filed under a charge-mode period.
func TestMiddleware_OpensThePeriodTheRequestBelongsTo(t *testing.T) {
	tests := []struct {
		name string
		info RequestInfo
	}{
		{"x402", RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}},
		{"mpp_charge", RequestInfo{Consumer: "GABC", Protocol: ProtocolMPPCharge}},
		{"mpp_session", RequestInfo{Consumer: "GABC", Protocol: ProtocolMPPSession, Channel: "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &fakeRecorder{}
			periods := &recordingPeriods{period: &meter.Period{ID: 99}}
			cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)
			cfg.Periods = periods

			mw, err := Middleware(cfg)
			if err != nil {
				t.Fatalf("Middleware returned unexpected error: %v", err)
			}
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

			req := httptest.NewRequest("GET", "/v1/items", nil)
			req = req.WithContext(WithRequestInfo(req.Context(), tt.info))
			mw(next).ServeHTTP(httptest.NewRecorder(), req)

			want := meter.Scope{
				Operator: cfg.Operator,
				Consumer: tt.info.Consumer,
				Protocol: tt.info.Protocol,
				Channel:  tt.info.Channel,
			}
			if periods.scope != want {
				t.Errorf("scope = %+v, want %+v", periods.scope, want)
			}
			if periods.ledger != 1500 {
				t.Errorf("ledger = %d, want 1500", periods.ledger)
			}
			if len(recorder.recorded) != 1 {
				t.Fatalf("recorded %d requests, want 1", len(recorder.recorded))
			}
			if got := recorder.recorded[0].PeriodID; got != 99 {
				t.Errorf("PeriodID = %d, want 99", got)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	valid := baseConfig(t, fakeLedgerSource{ledger: 1500}, &fakeRecorder{})

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"empty operator", func(c *Config) { c.Operator = "" }},
		{"empty method", func(c *Config) { c.Method = "" }},
		{"empty path template", func(c *Config) { c.PathTemplate = "" }},
		{"nil catalog source", func(c *Config) { c.Catalogs = nil }},
		{"nil ledger source", func(c *Config) { c.Ledger = nil }},
		{"nil recorder", func(c *Config) { c.Recorder = nil }},
		{"nil period opener", func(c *Config) { c.Periods = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if _, err := Middleware(cfg); err == nil {
				t.Error("Middleware accepted an invalid config")
			}
		})
	}
}

// --- failure-path coverage: every error source must fail closed ---
//
// Each of these fails for a different reason (bad RequestInfo, a ledger
// fetch error, a pricing error, a recorder error) but must produce the same
// externally observable behaviour: a 500, next never called, and nothing
// recorded. A table keeps that invariant checked uniformly rather than
// duplicated per failure kind.

var errLedgerUnavailable = errors.New("rpc: ledger unavailable")

var errRecordFailed = errors.New("store: connection refused")

var errCatalogUnavailable = errors.New("price book unavailable")

var errPeriodUnavailable = errors.New("database: cannot open period")

func TestMiddleware_FailureSourcesAllFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		cfg      func(t *testing.T, recorder *fakeRecorder) Config
		req      func() *http.Request
		wantCode int
	}{
		{
			name: "invalid RequestInfo (bad protocol)",
			cfg:  func(t *testing.T, r *fakeRecorder) Config { return baseConfig(t, fakeLedgerSource{ledger: 1500}, r) },
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: "not-a-real-protocol"}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "ledger source error",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				return baseConfig(t, fakeLedgerSource{err: errLedgerUnavailable}, r)
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "catalog source error",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, r)
				cfg.Catalogs = fakeCatalogs{err: errCatalogUnavailable}
				return cfg
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "period opener error",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, r)
				cfg.Periods = fakePeriods{err: errPeriodUnavailable}
				return cfg
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "pricing error (unpriced endpoint)",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, r)
				cfg.PathTemplate = "/v1/unpriced" // no rule for this path in the schedule
				return cfg
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/unpriced", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "pricing error (ledger before any published version)",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				return baseConfig(t, fakeLedgerSource{ledger: 1}, r) // schedule is effective from ledger 1000
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "recorder error",
			cfg: func(t *testing.T, r *fakeRecorder) Config {
				r.err = errRecordFailed
				return baseConfig(t, fakeLedgerSource{ledger: 1500}, r)
			},
			req: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/items", nil)
				info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
				return req.WithContext(WithRequestInfo(req.Context(), info))
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &fakeRecorder{}
			cfg := tt.cfg(t, recorder)
			mw, err := Middleware(cfg)
			if err != nil {
				t.Fatalf("Middleware returned unexpected error: %v", err)
			}

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { nextCalled = true })

			rec := httptest.NewRecorder()
			mw(next).ServeHTTP(rec, tt.req())

			if nextCalled {
				t.Error("next was called on a failure path")
			}
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if len(recorder.recorded) != 0 {
				t.Error("a request was recorded on a failure path")
			}
			// The response body must never leak the internal error text —
			// only the generic message http.Error writes.
			if body := rec.Body.String(); body != "internal server error\n" {
				t.Errorf("response body = %q, want the generic \"internal server error\" message only", body)
			}
		})
	}
}

func TestMiddleware_CustomUnitCount(t *testing.T) {
	recorder := &fakeRecorder{}
	cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)
	cfg.UnitCount = func(r *http.Request) uint64 { return 7 }

	mw, err := Middleware(cfg)
	if err != nil {
		t.Fatalf("Middleware returned unexpected error: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/v1/items", nil)
	info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	mw(next).ServeHTTP(httptest.NewRecorder(), req)

	if len(recorder.recorded) != 1 {
		t.Fatalf("recorded %d requests, want 1", len(recorder.recorded))
	}
	got := recorder.recorded[0]
	if got.UnitCount != 7 {
		t.Errorf("UnitCount = %d, want 7", got.UnitCount)
	}
	// unit_price 1000 * 7 units = 7000.
	if got.ChargedAmount.Cmp(big.NewInt(7000)) != 0 {
		t.Errorf("ChargedAmount = %s, want 7000", got.ChargedAmount)
	}
}

func TestMiddleware_RequestIDsAreUniquePerCall(t *testing.T) {
	recorder := &fakeRecorder{}
	cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)
	mw, err := Middleware(cfg)
	if err != nil {
		t.Fatalf("Middleware returned unexpected error: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	info := RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}
	const calls = 20
	for i := 0; i < calls; i++ {
		req := httptest.NewRequest("GET", "/v1/items", nil)
		req = req.WithContext(WithRequestInfo(req.Context(), info))
		mw(next).ServeHTTP(httptest.NewRecorder(), req)
	}

	if len(recorder.recorded) != calls {
		t.Fatalf("recorded %d requests, want %d", len(recorder.recorded), calls)
	}
	seen := make(map[[32]byte]bool, calls)
	for _, rec := range recorder.recorded {
		if seen[rec.RequestID] {
			t.Fatalf("duplicate RequestID %x across %d calls", rec.RequestID, calls)
		}
		seen[rec.RequestID] = true
	}
}

func TestMiddleware_MPPSessionChannelIsRecorded(t *testing.T) {
	recorder := &fakeRecorder{}
	cfg := baseConfig(t, fakeLedgerSource{ledger: 1500}, recorder)
	mw, err := Middleware(cfg)
	if err != nil {
		t.Fatalf("Middleware returned unexpected error: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/v1/items", nil)
	info := RequestInfo{
		Consumer: "GABC",
		Protocol: ProtocolMPPSession,
		Channel:  "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW",
	}
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	mw(next).ServeHTTP(httptest.NewRecorder(), req)

	if len(recorder.recorded) != 1 {
		t.Fatalf("recorded %d requests, want 1", len(recorder.recorded))
	}
	if got := recorder.recorded[0].Channel; got != info.Channel {
		t.Errorf("Channel = %q, want %q", got, info.Channel)
	}
	if got := recorder.recorded[0].Protocol; got != ProtocolMPPSession {
		t.Errorf("Protocol = %q, want %q", got, ProtocolMPPSession)
	}
}

func TestRequestInfo_Validate(t *testing.T) {
	tests := []struct {
		name    string
		info    RequestInfo
		wantErr bool
	}{
		{"valid x402", RequestInfo{Consumer: "GABC", Protocol: ProtocolX402}, false},
		{"valid mpp_charge", RequestInfo{Consumer: "GABC", Protocol: ProtocolMPPCharge}, false},
		{"valid mpp_session with channel", RequestInfo{Consumer: "GABC", Protocol: ProtocolMPPSession, Channel: "CDEF"}, false},
		{"empty consumer", RequestInfo{Consumer: "", Protocol: ProtocolX402}, true},
		{"unknown protocol", RequestInfo{Consumer: "GABC", Protocol: "carrier_pigeon"}, true},
		{"mpp_session without channel", RequestInfo{Consumer: "GABC", Protocol: ProtocolMPPSession}, true},
		{"x402 with channel", RequestInfo{Consumer: "GABC", Protocol: ProtocolX402, Channel: "CDEF"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ExampleRequestInfo_Validate shows both halves of Validate's contract: a
// well-formed x402 request is accepted, and an mpp_session request with no
// channel is rejected with ErrChannelMismatch, identified with errors.Is
// rather than by comparing the message. The remaining rejection classes —
// ErrConsumerEmpty and ErrUnknownProtocol — are covered by
// TestRequestInfo_Validate.
func ExampleRequestInfo_Validate() {
	accepted := RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolX402,
	}
	fmt.Println("accepted:", accepted.Validate() == nil)

	rejected := RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolMPPSession, // Channel deliberately left unset.
	}
	err := rejected.Validate()
	fmt.Println("rejected:", errors.Is(err, ErrChannelMismatch))

	// Output:
	// accepted: true
	// rejected: true
}

// BenchmarkMiddleware measures the per-request cost of metering one request
// end to end: RequestInfo validation, the ledger read, pricing against the
// catalog, request-ID generation, endpoint hashing, and recording. Both
// external interfaces are the existing test stubs, so this touches neither
// Postgres nor the network and runs in CI without the service container.
// Every iteration takes the all-success hot path — an error from any stub
// would have the middleware fail the request closed instead.
//
// recorder.recorded is truncated each iteration rather than allowed to
// grow: the fake appends one RecordedRequest per call, and without the
// truncation a run of b.N iterations would hold b.N of them. After the
// first append the backing array is reused, so the append itself does not
// allocate and the allocation count reflects the middleware, not the fake.
func BenchmarkMiddleware(b *testing.B) {
	recorder := &fakeRecorder{}
	cfg := Config{
		Operator:     "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM",
		Method:       "GET",
		PathTemplate: "/v1/items",
		Catalogs:     fakeCatalogs{catalog: mustCatalog(b, "1000")},
		Ledger:       fakeLedgerSource{ledger: 1500},
		Recorder:     recorder,
		Periods:      fakePeriods{period: &meter.Period{ID: 42}},
	}

	handler, err := Middleware(cfg)
	if err != nil {
		b.Fatalf("Middleware returned unexpected error: %v", err)
	}

	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	metered := handler(next)

	info := RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolX402,
	}
	req := httptest.NewRequest("GET", "/v1/items", nil)
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder.recorded = recorder.recorded[:0]
		metered.ServeHTTP(w, req)
	}
	b.StopTimer()

	if len(recorder.recorded) != 1 {
		b.Fatalf("recorded %d requests on the last iteration, want 1", len(recorder.recorded))
	}
}

// BenchmarkEndpointHashAndRequestID isolates the two primitives the
// middleware performs per request besides pricing — hashing the endpoint
// (merkle.EndpointHash: sha256 over the method and path template) and
// drawing a fresh 32-byte request ID from crypto/rand — so a regression in
// BenchmarkMiddleware can be attributed to one of them rather than leaving
// both as suspects. The two are sub-benchmarks rather than one loop because
// they are independent costs and this repository's guidance is to measure
// crypto/rand separately before concluding anything about the rest.
func BenchmarkEndpointHashAndRequestID(b *testing.B) {
	b.Run("EndpointHash", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = merkle.EndpointHash("GET", "/v1/items")
		}
	})

	b.Run("RequestID", func(b *testing.B) {
		var requestID [32]byte
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := rand.Read(requestID[:]); err != nil {
				b.Fatalf("rand.Read returned unexpected error: %v", err)
			}
		}
	})
}

// ExampleMiddleware shows how a caller configures and runs Middleware to
// meter a paid endpoint, record the request, and invoke the wrapped handler.
func ExampleMiddleware() {
	// A deterministic fake ledger source returning ledger 1500.
	ledger := fakeLedgerSource{ledger: 1500}
	// A fake recorder that prints the recorded request details.
	recorder := &printRecorder{}

	sched, err := meter.ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1000"}]}`))
	if err != nil {
		fmt.Println("ParseSchedule error:", err)
		return
	}
	catalog, err := meter.NewCatalog(meter.CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		fmt.Println("NewCatalog error:", err)
		return
	}

	cfg := Config{
		Operator:     "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM",
		Method:       "GET",
		PathTemplate: "/v1/items",
		Catalogs:     fakeCatalogs{catalog: catalog},
		Ledger:       ledger,
		Recorder:     recorder,
		Periods:      fakePeriods{period: &meter.Period{ID: 42}},
	}

	mw, err := Middleware(cfg)
	if err != nil {
		fmt.Println("Middleware error:", err)
		return
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/v1/items", nil)
	info := RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolX402,
	}
	req = req.WithContext(WithRequestInfo(req.Context(), info))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	fmt.Printf("response status: %d, body: %s\n", rec.Code, rec.Body.String())

	// Output:
	// recorded: GET /v1/items consumer GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY protocol x402
	// 1 unit(s) charged 1000 stroops price v1 observed ledger 1500
	// response status: 200, body: ok
}

type printRecorder struct{}

func (printRecorder) RecordRequest(_ context.Context, req RecordedRequest) error {
	fmt.Printf("recorded: %s %s consumer %s protocol %s\n", req.Method, req.PathTemplate, req.Consumer, req.Protocol)
	fmt.Printf("%d unit(s) charged %s stroops price v%d observed ledger %d\n", req.UnitCount, req.ChargedAmount, req.PriceVersion, req.ObservedLedger)
	return nil
}

// ExampleWithRequestInfo shows where WithRequestInfo sits: in the
// middleware upstream of metering that has established who is paying
// (in the collector, cmd/collector's header-reading middleware). It
// attaches the caller's RequestInfo to the request context, and the
// metering Middleware reads it back. RequestInfo is never derived by the
// metering layer itself — verifying payment is a non-goal of this
// package — so a request reaching Middleware without one fails closed
// with a 500 instead of being recorded against nobody.
//
// The ledger, catalog, period, and recorder are the package's own test
// stubs, so nothing touches Postgres or the network. There is no
// // Output: line: the example is compiled by `go test` but deliberately
// not run, so it pins the call shape of WithRequestInfo without
// duplicating assertions — the round trip and the missing-RequestInfo
// failure are asserted by TestMiddleware_RecordsAndCallsNext and
// TestMiddleware_MissingRequestInfoFailsClosed.
func ExampleWithRequestInfo() {
	sched, err := meter.ParseSchedule([]byte(`{"rules":[{"method":"GET","path_template":"/v1/items","unit_price":"1000"}]}`))
	if err != nil {
		fmt.Println("ParseSchedule:", err)
		return
	}
	catalog, err := meter.NewCatalog(meter.CatalogVersion{Version: 1, EffectiveLedger: 1000, Schedule: sched})
	if err != nil {
		fmt.Println("NewCatalog:", err)
		return
	}

	metering, err := Middleware(Config{
		Operator:     "GDOLCHAOYP63BEHGAUJJS5IVQNUXLPBWCHO2HZRBTZRU52XBMW2TRJLM",
		Method:       "GET",
		PathTemplate: "/v1/items",
		Catalogs:     fakeCatalogs{catalog: catalog},
		Ledger:       fakeLedgerSource{ledger: 1500},
		Recorder:     &fakeRecorder{},
		Periods:      fakePeriods{period: &meter.Period{ID: 42}},
	})
	if err != nil {
		fmt.Println("Middleware:", err)
		return
	}

	// identify stands in for whatever verified the payment upstream.
	identify := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := RequestInfo{
				Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
				Protocol: ProtocolX402,
			}
			next.ServeHTTP(w, r.WithContext(WithRequestInfo(r.Context(), info)))
		})
	}
	paid := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/items", nil)
	rec := httptest.NewRecorder()
	identify(metering(paid)).ServeHTTP(rec, req)
}

// ExampleRequestInfoFromContext shows the read side: a handler behind the
// upstream middleware that called WithRequestInfo retrieves the caller's
// RequestInfo, and the boolean reports whether one was attached at all.
// The metering Middleware treats a missing RequestInfo as a wiring bug
// and fails closed, and any other reader should do the same rather than
// proceed with the zero value — an empty Consumer would otherwise be
// metered and billed to nobody.
//
// There is no // Output: line: the example is compiled by `go test` but
// deliberately not run, so it pins the call shape of
// RequestInfoFromContext without duplicating assertions — both the
// present and the missing case are asserted by
// TestMiddleware_RecordsAndCallsNext and
// TestMiddleware_MissingRequestInfoFailsClosed.
// It opens no network or database connection.
func ExampleRequestInfoFromContext() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := RequestInfoFromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "consumer %s via %s", info.Consumer, info.Protocol)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Present: attached upstream with WithRequestInfo.
	ctx = WithRequestInfo(ctx, RequestInfo{
		Consumer: "GAJ46LDZSSYAB4YY6VMPM763P652ZROT7TBG3YYE3BZBAUZDXYDDK6OY",
		Protocol: ProtocolX402,
	})
	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/items", nil))

	// Missing: a bare context carries no RequestInfo, so ok is false.
	if _, ok := RequestInfoFromContext(context.Background()); !ok {
		fmt.Println("no RequestInfo attached")
	}
}
