package stellar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// fixtureServer returns an httptest.Server that replays the recorded
// response in testdata/stellar/<name>.json verbatim, and a func returning
// the last request body it received (for asserting what the client sent).
func fixtureServer(t *testing.T, name string) (*httptest.Server, func() []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "stellar", name+".json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var lastRequest []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRequest, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []byte { return lastRequest }
}

// ExampleNewClient shows one accepted input and one rejected input. NewClient
// itself never fails — it has no error return and validates nothing at
// construction — so the rejection half of its contract is enforced on the
// first call, where the error wraps a sentinel a caller matches with
// errors.Is rather than a message to compare as a string.
func ExampleNewClient() {
	// Accepted input: a well-formed endpoint URL and a nil *http.Client.
	// NewClient substitutes one bounded by defaultTimeout, observable on
	// the returned Client from this in-package example. Nothing is dialed
	// here — constructing is the whole call.
	c := NewClient("https://soroban-testnet.stellar.org", nil)
	fmt.Println("accepted: default timeout =", c.httpClient.Timeout)

	// Rejected input: a context that is already cancelled. The first call
	// refuses it before any request is sent, and the error identifies
	// itself by wrapping the context.Canceled sentinel — matched with
	// errors.Is, as CONTRIBUTING requires, never by its string.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetLatestLedger(ctx)
	fmt.Println("rejected: errors.Is(err, context.Canceled) =", errors.Is(err, context.Canceled))

	// Output:
	// accepted: default timeout = 30s
	// rejected: errors.Is(err, context.Canceled) = true
}

// ExampleRPCError_Error demonstrates the stable protocol error message
// callers see when Soroban RPC returns a JSON-RPC error.
func ExampleRPCError_Error() {
	err := &RPCError{Code: -32602, Message: "invalid_xdr"}
	fmt.Println(err.Error())
	// Output:
	// soroban rpc: code -32602: invalid_xdr
}

// exampleFixtureClient returns a Client pointed at a loopback httptest.Server
// that replays the recorded response in testdata/stellar/<name>.json, so an
// example runs offline against a real testnet response rather than a
// hand-invented one. The returned func stops the server.
func exampleFixtureClient(name string) (*Client, func()) {
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "stellar", name+".json"))
	if err != nil {
		log.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	return NewClient(srv.URL, nil), srv.Close
}

// ExampleGetLatestLedgerResult shows that a caller never fills in a
// GetLatestLedgerResult itself: it has no request-side fields, and every
// field is set by GetLatestLedger from the RPC node's response. The zero
// value's Sequence of 0 is not a ledger, so a caller only trusts a result
// that came back with a nil error. Sequence is what the collector and
// settler read as "the current ledger"; CloseTime is a unix timestamp in
// seconds, carried as a string.
func ExampleGetLatestLedgerResult() {
	var zero GetLatestLedgerResult
	fmt.Printf("zero value: sequence=%d closeTime=%q\n", zero.Sequence, zero.CloseTime)

	// Replays the recorded testnet getLatestLedger response
	// (testdata/stellar/get_latest_ledger.json).
	client, stop := exampleFixtureClient("get_latest_ledger")
	defer stop()

	latest, err := client.GetLatestLedger(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("sequence:", latest.Sequence)
	fmt.Println("protocol:", latest.ProtocolVersion)

	closeUnix, err := strconv.ParseInt(latest.CloseTime, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("closed at:", time.Unix(closeUnix, 0).UTC().Format(time.RFC3339))

	// Output:
	// zero value: sequence=0 closeTime=""
	// sequence: 4611229
	// protocol: 28
	// closed at: 2026-09-10T22:35:32Z
}

// ExampleGetLedgerEntriesResult shows a GetLedgerEntriesResult as
// GetLedgerEntries fills it in: the caller supplies only the keys, and the
// RPC node supplies every field of the result. A key with no live entry is
// simply absent from Entries rather than an error, so the zero value's empty
// Entries reads the same as "not found" (fetchSequenceNumber in invoke.go
// relies on exactly that); a caller matches entries by Key rather than by
// position. LatestLedger is the ledger the node answered at, which is what a
// LiveUntilLedgerSeq is measured against.
func ExampleGetLedgerEntriesResult() {
	var zero GetLedgerEntriesResult
	fmt.Println("zero value: entries =", len(zero.Entries), "latestLedger =", zero.LatestLedger)

	// Replays the recorded testnet getLedgerEntries response for contract
	// CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC's instance
	// key (testdata/stellar/get_ledger_entries.json).
	client, stop := exampleFixtureClient("get_ledger_entries")
	defer stop()

	const instanceKey = "AAAABgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAABQAAAAB"
	result, err := client.GetLedgerEntries(context.Background(), []string{instanceKey})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("entries:", len(result.Entries))
	fmt.Println("latestLedger:", result.LatestLedger)

	for _, entry := range result.Entries {
		if entry.Key != instanceKey {
			continue
		}
		fmt.Println("ledgers until archival:", entry.LiveUntilLedgerSeq-result.LatestLedger)
	}

	// Output:
	// zero value: entries = 0 latestLedger = 0
	// entries: 1
	// latestLedger: 4611230
	// ledgers until archival: 2950337
}

// ExampleLedgerEntryChange reads the StateChanges of a real simulation:
// the recorded testnet simulateTransaction response for a price_book
// publish call (testdata/stellar/pricebook_publish_02_simulate.json),
// decoded with the same JSON tags SimulateTransaction uses. A caller never
// builds a LedgerEntryChange; the RPC node reports one per ledger entry the
// invocation would write. Before is nil for an entry the call creates and
// After is nil for one it deletes, which is why both are pointers: the zero
// value, with an empty Type and two nil pointers, is not a change at all.
// Key is a base64 LedgerKey; Before and After are base64 LedgerEntry XDR.
func ExampleLedgerEntryChange() {
	var zero LedgerEntryChange
	fmt.Printf("zero value: type=%q before=%t after=%t\n", zero.Type, zero.Before != nil, zero.After != nil)

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "stellar", "pricebook_publish_02_simulate.json"))
	if err != nil {
		log.Fatal(err)
	}
	var resp struct {
		Result SimulateTransactionResult `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		log.Fatal(err)
	}

	for _, change := range resp.Result.StateChanges {
		var key xdr.LedgerKey
		if err := xdr.SafeUnmarshalBase64(change.Key, &key); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s %s: before=%t after=%t\n", change.Type, key.Type, change.Before != nil, change.After != nil)
	}

	// Output:
	// zero value: type="" before=false after=false
	// updated LedgerEntryTypeContractData: before=true after=true
	// updated LedgerEntryTypeContractData: before=true after=true
	// created LedgerEntryTypeContractData: before=false after=true
}

// ExampleLedgerEntryResult reads two LedgerEntryResults as GetLedgerEntries
// fills them in from recorded testnet responses. Every field is set by the
// RPC node, never by the caller. XDR decodes as LedgerEntryData, the same
// way fetchSequenceNumber in invoke.go reads an account's sequence number.
// LiveUntilLedgerSeq only applies to Soroban entries, which carry a TTL: a
// classic account entry has none, the node omits the field, and it decodes
// to 0, the same as the zero value. So 0 here means "no TTL", not "expired".
func ExampleLedgerEntryResult() {
	var zero LedgerEntryResult
	fmt.Println("zero value: lastModified =", zero.LastModifiedLedgerSeq, "liveUntil =", zero.LiveUntilLedgerSeq)

	// A contract instance entry (testdata/stellar/get_ledger_entries.json)
	// and a classic account entry fetched for a sequence number
	// (testdata/stellar/pricebook_publish_01_getledgerentries_account.json).
	fixtures := []struct{ name, key string }{
		{"get_ledger_entries", "AAAABgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAABQAAAAB"},
		{"pricebook_publish_01_getledgerentries_account", "AAAAAAAAAACxo2Y62NJF7xUghukoiQ0XteQyuJfdetk9gOOSFFdt4A=="},
	}
	for _, f := range fixtures {
		client, stop := exampleFixtureClient(f.name)
		result, err := client.GetLedgerEntries(context.Background(), []string{f.key})
		stop()
		if err != nil {
			log.Fatal(err)
		}
		entry := result.Entries[0]

		var data xdr.LedgerEntryData
		if err := xdr.SafeUnmarshalBase64(entry.XDR, &data); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: lastModified = %d liveUntil = %d\n", data.Type, entry.LastModifiedLedgerSeq, entry.LiveUntilLedgerSeq)
	}

	// Output:
	// zero value: lastModified = 0 liveUntil = 0
	// LedgerEntryTypeContractData: lastModified = 4609662 liveUntil = 7561567
	// LedgerEntryTypeAccount: lastModified = 4611396 liveUntil = 0
}

func TestClient_GetLatestLedger(t *testing.T) {
	srv, lastRequest := fixtureServer(t, "get_latest_ledger")
	c := NewClient(srv.URL, nil)

	result, err := c.GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("GetLatestLedger returned unexpected error: %v", err)
	}
	if result.ID != "ebf0cf5947dd21719178d9f34bef992e6e5e197db9debc54c5761d1937772321" {
		t.Errorf("ID = %q", result.ID)
	}
	if result.ProtocolVersion != 28 {
		t.Errorf("ProtocolVersion = %d, want 28", result.ProtocolVersion)
	}
	if result.Sequence != 4611229 {
		t.Errorf("Sequence = %d, want 4611229", result.Sequence)
	}
	if result.CloseTime != "1789079732" {
		t.Errorf("CloseTime = %q, want 1789079732", result.CloseTime)
	}
	if result.HeaderXDR == "" || result.MetadataXDR == "" {
		t.Error("HeaderXDR/MetadataXDR should not be empty")
	}

	req := string(lastRequest())
	if !containsAll(req, `"method":"getLatestLedger"`, `"jsonrpc":"2.0"`) {
		t.Errorf("request body %s does not look like a getLatestLedger call", req)
	}
	if containsAll(req, `"params"`) {
		t.Errorf("request body %s should omit params for a no-parameter method", req)
	}
}

func TestClient_GetEvents(t *testing.T) {
	srv, _ := fixtureServer(t, "get_events")
	c := NewClient(srv.URL, nil)

	result, err := c.GetEvents(context.Background(), GetEventsParams{
		StartLedger: 4611150,
		Pagination:  &EventsPagination{Limit: 3},
	})
	if err != nil {
		t.Fatalf("GetEvents returned unexpected error: %v", err)
	}
	if len(result.Events) != 3 {
		t.Fatalf("got %d events, want 3", len(result.Events))
	}
	first := result.Events[0]
	if first.Type != "contract" {
		t.Errorf("Events[0].Type = %q, want contract", first.Type)
	}
	if first.ContractID != "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC" {
		t.Errorf("Events[0].ContractID = %q", first.ContractID)
	}
	if first.ID != "0019804738446950400-0000000000" {
		t.Errorf("Events[0].ID = %q", first.ID)
	}
	if len(first.Topic) != 2 {
		t.Errorf("Events[0].Topic has %d entries, want 2", len(first.Topic))
	}
	if result.Cursor == "" {
		t.Error("Cursor should not be empty")
	}
	if result.LatestLedgerCloseTime == "" {
		t.Error("LatestLedgerCloseTime should not be empty")
	}
}

func TestClient_GetLedgerEntries(t *testing.T) {
	srv, lastRequest := fixtureServer(t, "get_ledger_entries")
	c := NewClient(srv.URL, nil)

	key := "AAAABgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAABQAAAAB"
	result, err := c.GetLedgerEntries(context.Background(), []string{key})
	if err != nil {
		t.Fatalf("GetLedgerEntries returned unexpected error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(result.Entries))
	}
	entry := result.Entries[0]
	if entry.Key != key {
		t.Errorf("Entries[0].Key = %q, want %q", entry.Key, key)
	}
	if entry.LastModifiedLedgerSeq != 4609662 {
		t.Errorf("LastModifiedLedgerSeq = %d, want 4609662", entry.LastModifiedLedgerSeq)
	}
	if entry.LiveUntilLedgerSeq != 7561567 {
		t.Errorf("LiveUntilLedgerSeq = %d, want 7561567", entry.LiveUntilLedgerSeq)
	}
	if entry.ExtXDR == "" {
		t.Error("ExtXDR should not be empty — the live response included it")
	}

	req := string(lastRequest())
	if !containsAll(req, `"method":"getLedgerEntries"`, key) {
		t.Errorf("request body %s does not carry the requested key", req)
	}
}

func TestClient_GetTransaction_Success(t *testing.T) {
	srv, _ := fixtureServer(t, "get_transaction_success")
	c := NewClient(srv.URL, nil)

	result, err := c.GetTransaction(context.Background(), "a55f146e117abdd29fb6643168e10f03fca79a761091355ea03effff3d44739a")
	if err != nil {
		t.Fatalf("GetTransaction returned unexpected error: %v", err)
	}
	if result.Status != GetTransactionStatusSuccess {
		t.Errorf("Status = %q, want %q", result.Status, GetTransactionStatusSuccess)
	}
	if result.ApplicationOrder != 2 {
		t.Errorf("ApplicationOrder = %d, want 2", result.ApplicationOrder)
	}
	if result.EnvelopeXDR == "" || result.ResultXDR == "" || result.ResultMetaXDR == "" {
		t.Error("EnvelopeXDR/ResultXDR/ResultMetaXDR should not be empty for a SUCCESS transaction")
	}
	if len(result.DiagnosticEventsXDR) == 0 {
		t.Error("DiagnosticEventsXDR should not be empty — the live response included several")
	}
}

func TestClient_GetTransaction_NotFound(t *testing.T) {
	srv, _ := fixtureServer(t, "get_transaction_not_found")
	c := NewClient(srv.URL, nil)

	result, err := c.GetTransaction(context.Background(), "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GetTransaction returned unexpected error: %v", err)
	}
	if result.Status != GetTransactionStatusNotFound {
		t.Errorf("Status = %q, want %q", result.Status, GetTransactionStatusNotFound)
	}
	// The live node returns these present-but-zero for NOT_FOUND rather than
	// omitting them; decoding must not choke on that, and callers must gate
	// on Status rather than treat a zero Ledger as "unknown".
	if result.Ledger != 0 || result.ApplicationOrder != 0 {
		t.Errorf("expected zero-valued Ledger/ApplicationOrder for NOT_FOUND, got %d/%d", result.Ledger, result.ApplicationOrder)
	}
}

func TestClient_SimulateTransaction_Success(t *testing.T) {
	srv, _ := fixtureServer(t, "simulate_transaction_success")
	c := NewClient(srv.URL, nil)

	result, err := c.SimulateTransaction(context.Background(), SimulateTransactionParams{Transaction: "AAAA"})
	if err != nil {
		t.Fatalf("SimulateTransaction returned unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty for a successful simulation", result.Error)
	}
	if result.MinResourceFee != "90353" {
		t.Errorf("MinResourceFee = %q, want 90353", result.MinResourceFee)
	}
	if len(result.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(result.Results))
	}
	if result.Cost == nil || result.Cost.CPUInsns != "1635562" {
		t.Errorf("Cost = %+v, want CPUInsns 1635562", result.Cost)
	}
}

// TestClient_SimulateTransaction_SimulationError covers the case CLAUDE.md
// calls out for custody's commitment verification: a simulation failure is
// carried in the result's own Error field with HTTP 200 and no top-level
// JSON-RPC error, not raised as a Go error from the call itself — the
// caller must check Error explicitly.
func TestClient_SimulateTransaction_SimulationError(t *testing.T) {
	srv, _ := fixtureServer(t, "simulate_transaction_error")
	c := NewClient(srv.URL, nil)

	result, err := c.SimulateTransaction(context.Background(), SimulateTransactionParams{Transaction: "not-valid-base64-xdr"})
	if err != nil {
		t.Fatalf("SimulateTransaction returned a Go error for a simulation-level failure: %v", err)
	}
	if result.Error == "" {
		t.Error("Error should be set for a failed simulation")
	}
}

func TestClient_SendTransaction_Pending(t *testing.T) {
	srv, _ := fixtureServer(t, "send_transaction_pending")
	c := NewClient(srv.URL, nil)

	result, err := c.SendTransaction(context.Background(), "AAAA")
	if err != nil {
		t.Fatalf("SendTransaction returned unexpected error: %v", err)
	}
	if result.Status != SendTransactionStatusPending {
		t.Errorf("Status = %q, want %q", result.Status, SendTransactionStatusPending)
	}
	if result.Hash != "d8ec9b68780314ffdfdfc2194b1b35dd27d7303c3bceaef6447e31631a1419dc" {
		t.Errorf("Hash = %q", result.Hash)
	}
}

// TestClient_SendTransaction_RPCError covers a protocol-level JSON-RPC
// error (malformed XDR): this must surface as a Go error, unlike
// simulateTransaction's own result.Error.
func TestClient_SendTransaction_RPCError(t *testing.T) {
	srv, _ := fixtureServer(t, "send_transaction_error")
	c := NewClient(srv.URL, nil)

	_, err := c.SendTransaction(context.Background(), "not-valid-base64-xdr")
	if err == nil {
		t.Fatal("SendTransaction returned nil error for an RPC-level error response")
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error %v does not wrap *RPCError", err)
	}
	if rpcErr.Code != -32602 {
		t.Errorf("Code = %d, want -32602", rpcErr.Code)
	}
	if rpcErr.Message != "invalid_xdr" {
		t.Errorf("Message = %q, want invalid_xdr", rpcErr.Message)
	}
}

func TestClient_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, nil)
	_, err := c.GetLatestLedger(context.Background())
	if err == nil {
		t.Fatal("GetLatestLedger returned nil error for a 503 response")
	}
	if !containsAll(err.Error(), "503") {
		t.Errorf("error %q does not mention the HTTP status", err.Error())
	}
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, nil)
	_, err := c.GetLatestLedger(context.Background())
	if err == nil {
		t.Fatal("GetLatestLedger returned nil error for a malformed JSON body")
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetLatestLedger(ctx)
	if err == nil {
		t.Fatal("GetLatestLedger returned nil error for an already-cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v does not wrap context.Canceled", err)
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
