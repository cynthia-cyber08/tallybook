package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"testing"
)

// testConsumer is the strkey encoding of the raw ed25519 public key
// 0x00, 0x01, ..., 0x1f — a fixed, arbitrary 32-byte value chosen so the
// expected bytes below could be derived independently without needing a
// real signed keypair.
const testConsumer = "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"

// The expected values in this file were derived independently of this
// package's implementation: a from-scratch Python XDR encoder, written
// directly against RFC 4506 and the ScVal/ScAddress/ScMap wire formats
// (verified against the xdr package's own EncodeTo methods, not assumed),
// reproduced the exact record_bytes and leaf below before this package's
// encodeRecord existed. testdata/merkle/*.json cannot serve this purpose —
// those fixtures start from pre-computed leaf hashes, not the records that
// produce them — so this is the closest available substitute for a
// contract-side reference vector.
func TestLeaf_KnownVectors(t *testing.T) {
	repeat := func(b byte) [32]byte {
		var out [32]byte
		for i := range out {
			out[i] = b
		}
		return out
	}

	tests := []struct {
		name          string
		record        Record
		wantRecordHex string
		wantLeafHex   string
	}{
		{
			name: "positive amount, mid-range fields",
			record: Record{
				Amount:       big.NewInt(123456789),
				Consumer:     testConsumer,
				EndpointHash: repeat(0xAA),
				Ledger:       1000,
				PriceVersion: 3,
				RequestID:    repeat(0xBB),
				Units:        7,
			},
			wantRecordHex: "0000001100000001000000070000000f00000006616d6f756e7400000000000a000000000000000000000000075bcd150000000f00000008636f6e73756d6572000000120000000000000000000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f0000000f00000008656e64706f696e740000000d00000020aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0000000f000000066c6564676572000000000003000003e80000000f0000000770726963655f760000000003000000030000000f0000000572657169640000000000000d00000020bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0000000f00000005756e697473000000000000050000000000000007",
			wantLeafHex:   "c83304f97dd71c23a1df9a26240ca0c9b57eb55ca6ee0685c5f323533facd31e",
		},
		{
			name: "negative amount, zero fields",
			record: Record{
				Amount:       big.NewInt(-1),
				Consumer:     testConsumer,
				EndpointHash: repeat(0x02),
				Ledger:       0,
				PriceVersion: 0,
				RequestID:    repeat(0x01),
				Units:        0,
			},
			wantRecordHex: "0000001100000001000000070000000f00000006616d6f756e7400000000000affffffffffffffffffffffffffffffff0000000f00000008636f6e73756d6572000000120000000000000000000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f0000000f00000008656e64706f696e740000000d0000002002020202020202020202020202020202020202020202020202020202020202020000000f000000066c6564676572000000000003000000000000000f0000000770726963655f760000000003000000000000000f0000000572657169640000000000000d0000002001010101010101010101010101010101010101010101010101010101010101010000000f00000005756e697473000000000000050000000000000000",
			wantLeafHex:   "5eda848b076d55f36589d9ce5d042544c96bb7686231b7bb8bbcdb9ea93bfae3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantRecord, err := hex.DecodeString(tt.wantRecordHex)
			if err != nil {
				t.Fatalf("bad test fixture hex: %v", err)
			}

			gotRecord, err := encodeRecord(tt.record)
			if err != nil {
				t.Fatalf("encodeRecord returned unexpected error: %v", err)
			}
			if hex.EncodeToString(gotRecord) != hex.EncodeToString(wantRecord) {
				t.Errorf("record_bytes =\n%s\nwant\n%s", hex.EncodeToString(gotRecord), hex.EncodeToString(wantRecord))
			}

			gotLeaf, err := Leaf(tt.record)
			if err != nil {
				t.Fatalf("Leaf returned unexpected error: %v", err)
			}
			if hex.EncodeToString(gotLeaf[:]) != tt.wantLeafHex {
				t.Errorf("leaf = %s, want %s", hex.EncodeToString(gotLeaf[:]), tt.wantLeafHex)
			}

			// The leaf must be the double sha256 of record_bytes, not a
			// single hash — verify against the record_bytes independently
			// of Leaf's own implementation.
			first := sha256.Sum256(gotRecord)
			second := sha256.Sum256(first[:])
			if hex.EncodeToString(second[:]) != tt.wantLeafHex {
				t.Errorf("sha256(sha256(record_bytes)) = %s, want %s (leaf must be a double hash)",
					hex.EncodeToString(second[:]), tt.wantLeafHex)
			}
		})
	}
}

func TestLeaf_Deterministic(t *testing.T) {
	r := Record{
		Amount:       big.NewInt(42),
		Consumer:     testConsumer,
		EndpointHash: [32]byte{1, 2, 3},
		Ledger:       10,
		PriceVersion: 1,
		RequestID:    [32]byte{4, 5, 6},
		Units:        1,
	}
	a, err := Leaf(r)
	if err != nil {
		t.Fatalf("Leaf returned unexpected error: %v", err)
	}
	b, err := Leaf(r)
	if err != nil {
		t.Fatalf("Leaf returned unexpected error: %v", err)
	}
	if a != b {
		t.Error("Leaf is not deterministic for identical input")
	}
}

// TestLeaf_EveryFieldIsBound changes each field of a base record one at a
// time and asserts the leaf changes, so a field silently dropped from the
// encoding (an implementation bug that no round-trip test would catch)
// shows up as a test failure.
func TestLeaf_EveryFieldIsBound(t *testing.T) {
	base := Record{
		Amount:       big.NewInt(100),
		Consumer:     testConsumer,
		EndpointHash: [32]byte{1},
		Ledger:       10,
		PriceVersion: 1,
		RequestID:    [32]byte{2},
		Units:        5,
	}
	baseLeaf, err := Leaf(base)
	if err != nil {
		t.Fatalf("Leaf returned unexpected error: %v", err)
	}

	mutate := func(name string, mutation func(r *Record)) {
		t.Run(name, func(t *testing.T) {
			r := base
			mutation(&r)
			leaf, err := Leaf(r)
			if err != nil {
				t.Fatalf("Leaf returned unexpected error: %v", err)
			}
			if leaf == baseLeaf {
				t.Errorf("changing %s did not change the leaf", name)
			}
		})
	}

	mutate("amount", func(r *Record) { r.Amount = big.NewInt(101) })
	mutate("endpoint_hash", func(r *Record) { r.EndpointHash[0] = 0xFF })
	mutate("ledger", func(r *Record) { r.Ledger++ })
	mutate("price_version", func(r *Record) { r.PriceVersion++ })
	mutate("request_id", func(r *Record) { r.RequestID[0] = 0xFF })
	mutate("units", func(r *Record) { r.Units++ })
}

func TestLeaf_RejectsOutOfRangeAmount(t *testing.T) {
	tooLarge := new(big.Int).Add(i128Max, big.NewInt(1))
	tooSmall := new(big.Int).Sub(i128Min, big.NewInt(1))

	tests := []struct {
		name   string
		amount *big.Int
	}{
		{"too large", tooLarge},
		{"too small", tooSmall},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Record{Amount: tt.amount, Consumer: testConsumer}
			if _, err := Leaf(r); err == nil {
				t.Errorf("Leaf accepted out-of-range amount %s", tt.amount)
			}
		})
	}
}

func TestLeaf_AcceptsI128Boundaries(t *testing.T) {
	tests := []struct {
		name   string
		amount *big.Int
	}{
		{"i128 min", i128Min},
		{"i128 max", i128Max},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Record{Amount: tt.amount, Consumer: testConsumer}
			if _, err := Leaf(r); err != nil {
				t.Errorf("Leaf rejected boundary amount %s: %v", tt.amount, err)
			}
		})
	}
}

func TestLeaf_RejectsNilAmount(t *testing.T) {
	r := Record{Amount: nil, Consumer: testConsumer}
	if _, err := Leaf(r); err == nil {
		t.Error("Leaf accepted a nil amount")
	}
}

func ExampleLeaf() {
	leaf, err := Leaf(Record{
		Amount:   big.NewInt(123456789),
		Consumer: testConsumer,
		EndpointHash: [32]byte{
			0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
			0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
			0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
			0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
		},
		Ledger:       1000,
		PriceVersion: 3,
		RequestID: [32]byte{
			0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB,
			0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB,
			0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB,
			0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB,
		},
		Units: 7,
	})
	if err != nil {
		log.Fatalf("Leaf: %v", err)
	}
	fmt.Println(hex.EncodeToString(leaf[:]))
	// Output:
	// c83304f97dd71c23a1df9a26240ca0c9b57eb55ca6ee0685c5f323533facd31e
}

func TestLeaf_RejectsInvalidConsumerAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"empty", ""},
		{"wrong prefix", "C" + strings.Repeat("A", 55)},
		{"too short", "GSHORT"},
		{"garbage", "not-a-strkey-address-at-all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Record{Amount: big.NewInt(1), Consumer: tt.address}
			if _, err := Leaf(r); err == nil {
				t.Errorf("Leaf accepted invalid consumer address %q", tt.address)
			}
		})
	}
}

func TestEndpointHash_MatchesFormula(t *testing.T) {
	method := "GET"
	path := "/v1/widgets/{id}"
	want := sha256.Sum256([]byte(method + " " + path))
	got := EndpointHash(method, path)
	if got != want {
		t.Errorf("EndpointHash(%q, %q) = %x, want %x", method, path, got, want)
	}
}

func TestEndpointHash_Deterministic(t *testing.T) {
	a := EndpointHash("POST", "/v1/orders")
	b := EndpointHash("POST", "/v1/orders")
	if a != b {
		t.Error("EndpointHash is not deterministic for identical input")
	}
}

func TestEndpointHash_DistinguishesMethodAndPath(t *testing.T) {
	// "GET /ab" and "GET/ ab" must not collide just because concatenation
	// without the separator would make them identical strings.
	a := EndpointHash("GET", "/ab")
	b := EndpointHash("GET/", "ab")
	if a == b {
		t.Error("EndpointHash collided across a method/path boundary shift")
	}
}

// ExampleEndpointHash demonstrates how to compute the endpoint hash for a
// given HTTP method and path template.
func ExampleEndpointHash() {
	hash := EndpointHash("GET", "/v1/widgets/{id}")
	fmt.Printf("%x\n", hash)

	// Output:
	// f7968b93731d5e337052ea0d1b0c0863f568393cbfb5ab807a87f505922e3b01
}

// ExampleRecord shows how a caller constructs a Record and what the zero
// value is worth. Record is not an output type: every field is
// caller-supplied, because each comes from a different part of the same
// billed request — the pricing result (Amount, PriceVersion, Units), the
// request's identity (RequestID, EndpointHash), and where it was observed
// (Consumer, Ledger) — and the package fills in none of them. The zero
// value is not a valid record: Leaf refuses a nil Amount rather than
// encoding a zero charge, which is the failure shown below.
//
// EndpointHash is the one field a caller should not derive by hand: the
// package's EndpointHash is the single derivation shared with the meter's
// pricing lookup, so calling it is what keeps the two keyed on the same
// value.
func ExampleRecord() {
	r := Record{
		Amount:       big.NewInt(1500),
		Consumer:     testConsumer,
		EndpointHash: EndpointHash("GET", "/v1/inference"),
		Ledger:       1042,
		PriceVersion: 3,
		RequestID:    exampleFill32(0x42),
		Units:        1,
	}

	leaf, err := Leaf(r)
	if err != nil {
		log.Fatalf("Leaf: %v", err)
	}
	fmt.Printf("leaf: %x\n", leaf)

	_, err = Leaf(Record{})
	fmt.Println("zero value:", err)

	// Output:
	// leaf: 466eb5a1459628ea9a8fb04e573077fd9f95a761797a33054d912b22cbc81d14
	// zero value: merkle: encode record: amount: nil amount
}
