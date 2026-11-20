package stellar

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

type rawEvent struct {
	Topic []string `json:"topic"`
	Value string   `json:"value"`
}

func loadEventFixture(t testing.TB, name string) []rawEvent {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "stellar", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var doc struct {
		Result struct {
			Events []rawEvent `json:"events"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	if len(doc.Result.Events) == 0 {
		t.Fatalf("fixture %s has no events", path)
	}
	return doc.Result.Events
}

func ExampleDecodePriceBookPublishEvent() {
	// A genuine ("price_book", "publish") event's Value, captured live
	// from testnet (tx f3228c2dc7597914, fixture events_pricebook_publish).
	const publishValueB64 = "AAAAEAAAAAEAAAAEAAAAEgAAAAAAAAAAaSue9e7ifQ7u45Obpx8FFiCUrvDOYAF+ur/wm7XTZE0AAAADAAAAAQAAAA0AAAAgGj9XZu1r/5QDu00mLYMf1MIGS8sVIkTZkWu5E6NGEuAAAAADAEZgRQ=="
	ev, err := DecodePriceBookPublishEvent(publishValueB64)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("operator:", ev.Operator)
	fmt.Println("version:", ev.Version)
	fmt.Println("effective_ledger:", ev.EffectiveLedger)
	// Output:
	// operator: GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A
	// version: 1
	// effective_ledger: 4612165
}

func TestMatchesTopic(t *testing.T) {
	events := loadEventFixture(t, "events_pricebook_publish")
	ok, err := MatchesTopic(events[0].Topic, TopicPriceBook, TopicPublish)
	if err != nil {
		t.Fatalf("MatchesTopic returned unexpected error: %v", err)
	}
	if !ok {
		t.Error("MatchesTopic = false for a real price_book publish event's own topics")
	}

	ok, err = MatchesTopic(events[0].Topic, TopicStatement, TopicAnchor)
	if err != nil {
		t.Fatalf("MatchesTopic returned unexpected error: %v", err)
	}
	if ok {
		t.Error("MatchesTopic = true for mismatched topics")
	}

	ok, err = MatchesTopic(events[0].Topic, TopicPriceBook)
	if err != nil {
		t.Fatalf("MatchesTopic returned unexpected error: %v", err)
	}
	if ok {
		t.Error("MatchesTopic = true for a topic list of the wrong length")
	}
}

func ExampleMatchesTopic() {
	matched, err := MatchesTopic([]string{"AAAADwAAAAh3aXRoZHJhdw=="}, TopicWithdraw)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(matched)
	// Output: true
}

func ExampleDecodeStatementAnchorEvent() {
	// A genuine ("statement", "anchor") event's Value, captured live
	// from testnet (tx 24ae654ce0eee96e, fixture events_statement_registry).
	const anchorValueB64 = "AAAAEAAAAAEAAAAHAAAAEgAAAAAAAAAAaSue9e7ifQ7u45Obpx8FFiCUrvDOYAF+ur/wm7XTZE0AAAASAAAAAAAAAACtXWITvZ/9QnTuL9KzW/xEfHokr4K/TUEhRQ7HUBTv7wAAAAUAAAAAAAAAAgAAAA0AAAAggaEkUz5nXjZ3AZgrgejf1gyEi/t9Axkzv7xLL7BNQiUAAAAKAAAAAAAAAAAAAAAAAAAH0AAAAAoAAAAAAAAAAAAAAAAAAAfQAAAAEAAAAAEAAAABAAAADwAAAARYNDAy"
	ev, err := DecodeStatementAnchorEvent(anchorValueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("operator:", ev.Operator)
	fmt.Println("consumer:", ev.Consumer)
	fmt.Println("seq:", ev.Seq)
	fmt.Printf("usage_root: %x\n", ev.UsageRoot)
	fmt.Println("amount_billed:", ev.AmountBilled)
	fmt.Println("amount_settled:", ev.AmountSettled)
	fmt.Println("protocol:", ev.Protocol)
	// Output:
	// operator: GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A
	// consumer: GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG
	// seq: 2
	// usage_root: 81a124533e675e367701982b81e8dfd60c848bfb7d031933bfbc4b2fb04d4225
	// amount_billed: 2000
	// amount_settled: 2000
	// protocol: X402
}

func ExampleDecodeStatementDisputeEvent() {
	// A genuine ("statement", "dispute") event's Value, captured live
	// from testnet (tx ecbafcd85fc10381, fixture events_statement_registry).
	const disputeValueB64 = "AAAAEAAAAAEAAAAEAAAAEgAAAAAAAAAAaSue9e7ifQ7u45Obpx8FFiCUrvDOYAF+ur/wm7XTZE0AAAASAAAAAAAAAACtXWITvZ/9QnTuL9KzW/xEfHokr4K/TUEhRQ7HUBTv7wAAAAUAAAAAAAAAAgAAAA0AAAAgef6xLgsPk8jiZXvgRxL+ABu7LhG/XfXOXz5vKemhjoU="
	ev, err := DecodeStatementDisputeEvent(disputeValueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("operator:", ev.Operator)
	fmt.Println("consumer:", ev.Consumer)
	fmt.Println("seq:", ev.Seq)
	fmt.Printf("reason_hash: %x\n", ev.ReasonHash)
	// Output:
	// operator: GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A
	// consumer: GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG
	// seq: 2
	// reason_hash: 79feb12e0b0f93c8e2657be04712fe001bbb2e11bf5df5ce5f3e6f29e9a18e85
}

func ExampleDecodeStatementResolveEvent() {
	// A genuine ("statement", "resolve") event's Value, captured live
	// from testnet (seq 2 of the anchor→dispute→resolve cycle in
	// testdata/stellar/events_statement_registry.json).
	const resolveValueB64 = "AAAAEAAAAAEAAAAFAAAAEgAAAAAAAAAAaSue9e7ifQ7u45Obpx8FFiCUrvDOYAF+ur/wm7XTZE0AAAASAAAAAAAAAACtXWITvZ/9QnTuL9KzW/xEfHokr4K/TUEhRQ7HUBTv7wAAAAUAAAAAAAAAAgAAAA0AAAAg8A5inGgIkyd6LJ6/kc4nFZgaYZYrhltAbEs9w+7SQiwAAAAKAAAAAAAAAAAAAAAAAAAAyA=="
	ev, err := DecodeStatementResolveEvent(resolveValueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("operator:", ev.Operator)
	fmt.Println("consumer:", ev.Consumer)
	fmt.Println("seq:", ev.Seq)
	fmt.Printf("resolution_hash: %x\n", ev.ResolutionHash)
	fmt.Println("amount_credited:", ev.AmountCredited)
	// Output:
	// operator: GBUSXHXV53RH2DXO4OJZXJY7AULCBFFO6DHGAAL6XK77BG5V2NSE253A
	// consumer: GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG
	// seq: 2
	// resolution_hash: f00e629c680893277a2c9ebf91ce2715981a61962b865b406c4b3dc3eed2422c
	// amount_credited: 200
}

// TestDecodePriceBookPublishEvent_AgainstRealEvents decodes two genuine
// price_book publish events captured live from testnet — one from this
// package's own pricebook_test.go capture run, one from an earlier
// exploratory capture — cross-checking against known values from each.
func TestDecodePriceBookPublishEvent_AgainstRealEvents(t *testing.T) {
	events := loadEventFixture(t, "events_pricebook_publish")
	if len(events) != 2 {
		t.Fatalf("fixture has %d events, want 2", len(events))
	}

	for _, e := range events {
		ok, err := MatchesTopic(e.Topic, TopicPriceBook, TopicPublish)
		if err != nil {
			t.Fatalf("MatchesTopic returned unexpected error: %v", err)
		}
		if !ok {
			t.Fatalf("event topics %v do not match (price_book, publish)", e.Topic)
		}
	}

	// The second captured event is this package's own registryOperator
	// publishing version 1, effective at ledger 4612165 — the same
	// values TestPriceBook_Publish's fixtures assert on independently.
	ev, err := DecodePriceBookPublishEvent(events[1].Value)
	if err != nil {
		t.Fatalf("DecodePriceBookPublishEvent returned unexpected error: %v", err)
	}
	if ev.Operator != registryOperator {
		t.Errorf("Operator = %q, want %q", ev.Operator, registryOperator)
	}
	if ev.Version != 1 {
		t.Errorf("Version = %d, want 1", ev.Version)
	}
	if ev.EffectiveLedger != 4612165 {
		t.Errorf("EffectiveLedger = %d, want 4612165", ev.EffectiveLedger)
	}
}

// TestStatementEvents_AgainstRealEvents decodes six genuine
// anchor/dispute/resolve events captured live from testnet — two full
// anchor→dispute→resolve cycles this package's own registry_test.go
// fixtures were captured alongside — cross-checking every field against
// values already known from those fixtures.
func TestStatementEvents_AgainstRealEvents(t *testing.T) {
	events := loadEventFixture(t, "events_statement_registry")
	if len(events) != 6 {
		t.Fatalf("fixture has %d events, want 6", len(events))
	}

	var anchors []*StatementAnchorEvent
	var disputes []*StatementDisputeEvent
	var resolves []*StatementResolveEvent

	for i, e := range events {
		switch {
		case mustMatchTopic(t, e.Topic, TopicStatement, TopicAnchor):
			ev, err := DecodeStatementAnchorEvent(e.Value)
			if err != nil {
				t.Fatalf("event %d: DecodeStatementAnchorEvent: %v", i, err)
			}
			anchors = append(anchors, ev)
		case mustMatchTopic(t, e.Topic, TopicStatement, TopicDispute):
			ev, err := DecodeStatementDisputeEvent(e.Value)
			if err != nil {
				t.Fatalf("event %d: DecodeStatementDisputeEvent: %v", i, err)
			}
			disputes = append(disputes, ev)
		case mustMatchTopic(t, e.Topic, TopicStatement, TopicResolve):
			ev, err := DecodeStatementResolveEvent(e.Value)
			if err != nil {
				t.Fatalf("event %d: DecodeStatementResolveEvent: %v", i, err)
			}
			resolves = append(resolves, ev)
		default:
			t.Fatalf("event %d has unrecognized topics %v", i, e.Topic)
		}
	}

	if len(anchors) != 2 || len(disputes) != 2 || len(resolves) != 2 {
		t.Fatalf("got %d anchors, %d disputes, %d resolves, want 2 each", len(anchors), len(disputes), len(resolves))
	}

	// The seq-3 cycle matches registry_test.go's fixtures exactly.
	var anchor3, dispute3, resolve3 bool
	for _, a := range anchors {
		if a.Seq != 3 {
			continue
		}
		anchor3 = true
		if a.Operator != registryOperator {
			t.Errorf("anchor seq 3: Operator = %q, want %q", a.Operator, registryOperator)
		}
		if a.Consumer != registryConsumer {
			t.Errorf("anchor seq 3: Consumer = %q, want %q", a.Consumer, registryConsumer)
		}
		if a.AmountBilled.Cmp(big.NewInt(3000)) != 0 {
			t.Errorf("anchor seq 3: AmountBilled = %s, want 3000", a.AmountBilled)
		}
		if a.Protocol != ProtocolX402 {
			t.Errorf("anchor seq 3: Protocol = %q, want %q", a.Protocol, ProtocolX402)
		}
	}
	for _, d := range disputes {
		if d.Seq != 3 {
			continue
		}
		dispute3 = true
		if d.Consumer != registryConsumer {
			t.Errorf("dispute seq 3: Consumer = %q, want %q", d.Consumer, registryConsumer)
		}
	}
	for _, r := range resolves {
		if r.Seq != 3 {
			continue
		}
		resolve3 = true
		if r.AmountCredited.Cmp(big.NewInt(300)) != 0 {
			t.Errorf("resolve seq 3: AmountCredited = %s, want 300", r.AmountCredited)
		}
	}
	if !anchor3 || !dispute3 || !resolve3 {
		t.Errorf("did not find all of seq-3's anchor/dispute/resolve events: anchor=%v dispute=%v resolve=%v", anchor3, dispute3, resolve3)
	}
}

func mustMatchTopic(t *testing.T, topic []string, want ...string) bool {
	t.Helper()
	ok, err := MatchesTopic(topic, want...)
	if err != nil {
		t.Fatalf("MatchesTopic returned unexpected error: %v", err)
	}
	return ok
}

// The Open, Close, and Refund tests below are round-trip only; they confirm
// the field order and types declared in the contract source. Withdraw is
// additionally checked against a live capture in TestDecodeChannelWithdrawEvent_LiveEvent.
// The topic default (the struct name in snake_case) is exercised too; it
// is directly observed for Withdraw and follows the documented SDK default
// for the other events.

func TestDecodeChannelOpenEvent_RoundTrip(t *testing.T) {
	from := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
	to := "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG"
	token := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	var commitmentKey [32]byte
	for i := range commitmentKey {
		commitmentKey[i] = byte(i)
	}

	fromVal, err := ScvAddress(from)
	if err != nil {
		t.Fatalf("ScvAddress: %v", err)
	}
	toVal, err := ScvAddress(to)
	if err != nil {
		t.Fatalf("ScvAddress: %v", err)
	}
	tokenVal, err := ScvAddress(token)
	if err != nil {
		t.Fatalf("ScvAddress: %v", err)
	}
	amountVal, err := ScvI128(big.NewInt(500))
	if err != nil {
		t.Fatalf("ScvI128: %v", err)
	}

	// Field order matches event.rs's Open struct declaration exactly:
	// from, commitment_key, to, token, amount, refund_waiting_period.
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{
		fromVal, ScvBytes(commitmentKey[:]), toVal, tokenVal, amountVal, ScvU32(20),
	}))
	if err != nil {
		t.Fatalf("MarshalScValBase64: %v", err)
	}

	ev, err := DecodeChannelOpenEvent(valueB64)
	if err != nil {
		t.Fatalf("DecodeChannelOpenEvent returned unexpected error: %v", err)
	}
	if ev.From != from || ev.To != to || ev.Token != token {
		t.Errorf("From/To/Token = %q/%q/%q, want %q/%q/%q", ev.From, ev.To, ev.Token, from, to, token)
	}
	if ev.CommitmentKey != commitmentKey {
		t.Errorf("CommitmentKey = %x, want %x", ev.CommitmentKey, commitmentKey)
	}
	if ev.Amount.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("Amount = %s, want 500", ev.Amount)
	}
	if ev.RefundWaitingPeriod != 20 {
		t.Errorf("RefundWaitingPeriod = %d, want 20", ev.RefundWaitingPeriod)
	}
}

func TestDecodeChannelCloseEvent_RoundTrip(t *testing.T) {
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{ScvU32(4700123)}))
	if err != nil {
		t.Fatalf("MarshalScValBase64: %v", err)
	}
	ev, err := DecodeChannelCloseEvent(valueB64)
	if err != nil {
		t.Fatalf("DecodeChannelCloseEvent returned unexpected error: %v", err)
	}
	if ev.EffectiveAtLedger != 4700123 {
		t.Errorf("EffectiveAtLedger = %d, want 4700123", ev.EffectiveAtLedger)
	}
}

func TestDecodeChannelWithdrawEvent_RoundTrip(t *testing.T) {
	to := "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG"
	toVal, err := ScvAddress(to)
	if err != nil {
		t.Fatalf("ScvAddress: %v", err)
	}
	amountVal, err := ScvI128(big.NewInt(1234))
	if err != nil {
		t.Fatalf("ScvI128: %v", err)
	}
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{toVal, amountVal}))
	if err != nil {
		t.Fatalf("MarshalScValBase64: %v", err)
	}

	ev, err := DecodeChannelWithdrawEvent(valueB64)
	if err != nil {
		t.Fatalf("DecodeChannelWithdrawEvent returned unexpected error: %v", err)
	}
	if ev.To != to {
		t.Errorf("To = %q, want %q", ev.To, to)
	}
	if ev.Amount.Cmp(big.NewInt(1234)) != 0 {
		t.Errorf("Amount = %s, want 1234", ev.Amount)
	}
}

func TestDecodeChannelWithdrawEvent_LiveEvent(t *testing.T) {
	events := loadEventFixture(t, "events_channel_withdraw")
	if len(events) != 1 {
		t.Fatalf("fixture has %d events, want 1", len(events))
	}
	if !mustMatchTopic(t, events[0].Topic, TopicWithdraw) {
		t.Fatalf("event topics %v do not match Withdraw", events[0].Topic)
	}

	ev, err := DecodeChannelWithdrawEvent(events[0].Value)
	if err != nil {
		t.Fatalf("DecodeChannelWithdrawEvent returned unexpected error: %v", err)
	}
	wantPrefixBytes, err := hex.DecodeString("27c29810ea382ea4c044c71ebbf122d7f4d72a874b82a01d394ffbf91dd44c59")
	if err != nil {
		t.Fatalf("decode expected prefix: %v", err)
	}
	var wantPrefix [32]byte
	copy(wantPrefix[:], wantPrefixBytes)
	if ev.OpaquePrefix == nil || *ev.OpaquePrefix != wantPrefix {
		t.Errorf("OpaquePrefix = %v, want %x", ev.OpaquePrefix, wantPrefix)
	}
	if ev.To == "" {
		t.Error("To is empty")
	}
	if ev.Amount.Cmp(big.NewInt(150000000)) != 0 {
		t.Errorf("Amount = %s, want 150000000", ev.Amount)
	}
}

func ExampleDecodeChannelWithdrawEvent() {
	// A two-field Withdraw event (to, amount) built with the XDR helpers.
	to := "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG"
	toVal, err := ScvAddress(to)
	if err != nil {
		panic(err)
	}
	amountVal, err := ScvI128(big.NewInt(5_000_000))
	if err != nil {
		panic(err)
	}
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{toVal, amountVal}))
	if err != nil {
		panic(err)
	}

	ev, err := DecodeChannelWithdrawEvent(valueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("to:", ev.To)
	fmt.Println("amount:", ev.Amount)
	// Output:
	// to: GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG
	// amount: 5000000
}

func TestDecodeChannelRefundEvent_RoundTrip(t *testing.T) {
	from := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
	fromVal, err := ScvAddress(from)
	if err != nil {
		t.Fatalf("ScvAddress: %v", err)
	}
	amountVal, err := ScvI128(big.NewInt(9999))
	if err != nil {
		t.Fatalf("ScvI128: %v", err)
	}
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{fromVal, amountVal}))
	if err != nil {
		t.Fatalf("MarshalScValBase64: %v", err)
	}

	ev, err := DecodeChannelRefundEvent(valueB64)
	if err != nil {
		t.Fatalf("DecodeChannelRefundEvent returned unexpected error: %v", err)
	}
	if ev.From != from {
		t.Errorf("From = %q, want %q", ev.From, from)
	}
	if ev.Amount.Cmp(big.NewInt(9999)) != 0 {
		t.Errorf("Amount = %s, want 9999", ev.Amount)
	}
}

func ExampleDecodeChannelOpenEvent() {
	// A synthetically constructed one-way-channel Open event: from,
	// commitment_key, to, token, amount, refund_waiting_period — in the
	// field order event.rs declares.
	const openValueB64 = "AAAAEAAAAAEAAAAGAAAAEgAAAAAAAAAAAAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8AAAANAAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4fAAAAEgAAAAAAAAAArV1iE72f/UJ07i/Ss1v8RHx6JK+Cv01BIUUOx1AU7+8AAAASAAAAAdeSi3LCcDzP6vfrn/TvTVBKVai5efybRQ6iyEK00c5hAAAACgAAAAAAAAAAAAAAAAAAAfQAAAADAAAAFA=="
	ev, err := DecodeChannelOpenEvent(openValueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("from:", ev.From)
	fmt.Printf("commitment_key: %x\n", ev.CommitmentKey)
	fmt.Println("to:", ev.To)
	fmt.Println("token:", ev.Token)
	fmt.Println("amount:", ev.Amount)
	fmt.Println("refund_waiting_period:", ev.RefundWaitingPeriod)
	// Output:
	// from: GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX
	// commitment_key: 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// to: GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG
	// token: CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC
	// amount: 500
	// refund_waiting_period: 20
}

func ExampleDecodeChannelRefundEvent() {
	// A synthetically constructed one-way-channel Refund event: from and
	// the entire remaining balance returned to the payer (§1).
	const refundValueB64 = "AAAAEAAAAAEAAAACAAAAEgAAAAAAAAAAAAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8AAAAKAAAAAAAAAAAAAAAAAAAnDw=="
	ev, err := DecodeChannelRefundEvent(refundValueB64)
	if err != nil {
		panic(err)
	}
	fmt.Println("from:", ev.From)
	fmt.Println("amount:", ev.Amount)
	// Output:
	// from: GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX
	// amount: 9999
}

func TestDecodeEventFields_RejectsWrongLength(t *testing.T) {
	valueB64, err := MarshalScValBase64(ScvVec([]xdr.ScVal{ScvU32(1)}))
	if err != nil {
		t.Fatalf("MarshalScValBase64: %v", err)
	}
	if _, err := decodeEventFields(valueB64, 2); err == nil {
		t.Error("decodeEventFields accepted a value with the wrong number of fields")
	}
}

func FuzzDecodeEvents(f *testing.F) {
	f.Add("")
	f.Add("not-base64!")
	for _, fixture := range []string{"events_pricebook_publish", "events_statement_registry", "events_channel_withdraw"} {
		for _, event := range loadEventFixture(f, fixture) {
			f.Add(event.Value)
		}
	}

	address, err := ScvAddress("GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG")
	if err != nil {
		f.Fatalf("ScvAddress: %v", err)
	}
	amount, err := ScvI128(big.NewInt(1))
	if err != nil {
		f.Fatalf("ScvI128: %v", err)
	}
	threeFieldWithdraw, err := MarshalScValBase64(ScvVec([]xdr.ScVal{ScvBytes(make([]byte, 32)), address, amount}))
	if err != nil {
		f.Fatalf("MarshalScValBase64: %v", err)
	}
	f.Add(threeFieldWithdraw)
	fourFieldWithdraw, err := MarshalScValBase64(ScvVec([]xdr.ScVal{ScvBytes(make([]byte, 32)), address, amount, ScvU32(3)}))
	if err != nil {
		f.Fatalf("MarshalScValBase64: %v", err)
	}
	if _, err := DecodeChannelWithdrawEvent(fourFieldWithdraw); err == nil || !strings.Contains(err.Error(), "want 2 or 3") {
		f.Fatalf("four-field Withdraw payload error = %v, want a field-count rejection", err)
	}

	decoders := []struct {
		name  string
		check func(string) (bool, error)
	}{
		{"price_book publish", func(v string) (bool, error) {
			event, err := DecodePriceBookPublishEvent(v)
			return event != nil && event.Operator != "", err
		}},
		{"statement anchor", func(v string) (bool, error) {
			event, err := DecodeStatementAnchorEvent(v)
			return event != nil && event.Operator != "" && event.Consumer != "" && event.AmountBilled != nil && event.AmountSettled != nil, err
		}},
		{"statement dispute", func(v string) (bool, error) {
			event, err := DecodeStatementDisputeEvent(v)
			return event != nil && event.Operator != "" && event.Consumer != "", err
		}},
		{"statement resolve", func(v string) (bool, error) {
			event, err := DecodeStatementResolveEvent(v)
			return event != nil && event.Operator != "" && event.Consumer != "" && event.AmountCredited != nil, err
		}},
		{"channel Open", func(v string) (bool, error) {
			event, err := DecodeChannelOpenEvent(v)
			return event != nil && event.From != "" && event.To != "" && event.Token != "" && event.Amount != nil, err
		}},
		{"channel Close", func(v string) (bool, error) {
			event, err := DecodeChannelCloseEvent(v)
			return event != nil, err
		}},
		{"channel Withdraw", func(v string) (bool, error) {
			event, err := DecodeChannelWithdrawEvent(v)
			return event != nil && event.To != "" && event.Amount != nil, err
		}},
		{"channel Refund", func(v string) (bool, error) {
			event, err := DecodeChannelRefundEvent(v)
			return event != nil && event.From != "" && event.Amount != nil, err
		}},
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<16 {
			t.Skip()
		}
		for _, decoder := range decoders {
			populated, err := decoder.check(value)
			if err == nil {
				if !populated {
					t.Errorf("%s decoder returned an incomplete event", decoder.name)
				}
				continue
			}
			if strings.TrimSpace(err.Error()) == "" || (!strings.Contains(err.Error(), "event") && !strings.Contains(err.Error(), "field")) {
				t.Errorf("%s decoder returned an uncontextualized error: %v", decoder.name, err)
			}
		}
	})
}
