package stellar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func ExampleDecodeU64() {
	v, err := DecodeU64(ScvU64(42))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// 42
}

func ExampleDecodeU32() {
	v, err := DecodeU32(ScvU32(42))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// 42
}

func ExampleDecodeI32() {
	i := xdr.Int32(-42)
	v, err := DecodeI32(xdr.ScVal{Type: xdr.ScValTypeScvI32, I32: &i})
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// -42
}

func ExampleDecodeI64() {
	i := xdr.Int64(9007199254740992)
	v, err := DecodeI64(xdr.ScVal{Type: xdr.ScValTypeScvI64, I64: &i})
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// 9007199254740992
}

func ExampleScvI32() {
	v, err := DecodeI32(ScvI32(-42))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// -42
}

func ExampleScvI64() {
	v, err := DecodeI64(ScvI64(9007199254740991))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// 9007199254740991
}

func ExampleScvSymbol() {
	v, err := DecodeSymbol(ScvSymbol("price_v"))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// price_v
}

func ExampleDecodeSymbol() {
	v, err := DecodeSymbol(ScvSymbol("anchor"))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// anchor
}

func ExampleDecodeString() {
	v, err := DecodeString(ScvString("price/v1.json"))
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
	// Output:
	// price/v1.json
}

func ExampleScvAddress() {
	addr := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
	v, err := ScvAddress(addr)
	if err != nil {
		panic(err)
	}
	got, err := DecodeAddress(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(got)
	// Output:
	// GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX
}

func ExampleDecodeAddress() {
	v, err := ScvAddress("GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX")
	if err != nil {
		log.Fatal(err)
	}
	addr, err := DecodeAddress(v)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(addr)
	// Output:
	// GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX
}

func ExampleDecodeBool() {
	v, err := DecodeBool(ScvBool(true))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(v)
	// Output:
	// true
}

func ExampleErrWrongScValType_Error() {
	err := &ErrWrongScValType{
		Want: xdr.ScValTypeScvU32,
		Got:  xdr.ScValTypeScvString,
	}
	fmt.Println(err.Error())
	// Output:
	// stellar: expected ScVal type ScValTypeScvU32, got ScValTypeScvString
}

func ExampleDecodeVec() {
	vecVal := ScvVec([]xdr.ScVal{ScvU32(42), ScvSymbol("hello")})
	items, err := DecodeVec(vecVal)
	if err != nil {
		panic(err)
	}
	uVal, err := DecodeU32(items[0])
	if err != nil {
		panic(err)
	}
	sVal, err := DecodeSymbol(items[1])
	if err != nil {
		panic(err)
	}
	fmt.Println(uVal, sVal)
	// Output:
	// 42 hello
}

// ExampleDecodeMap decodes the map shape every contract binding in this
// package relies on — Symbol keys, one entry per struct field — and shows
// the one thing DecodeMap refuses to do: accept an entry whose key is not
// a Symbol. That entry is an error, not a silently dropped field, so a
// struct field that never arrives on chain fails loudly here instead of
// decoding as a zero value.
func ExampleDecodeMap() {
	decoded, err := DecodeMap(ScvMap([]xdr.ScMapEntry{
		MapEntry("amount", ScvU64(1250)),
		MapEntry("price_v", ScvU32(7)),
	}))
	if err != nil {
		log.Fatal(err)
	}
	amount, err := DecodeU64(decoded["amount"])
	if err != nil {
		log.Fatal(err)
	}
	priceVersion, err := DecodeU32(decoded["price_v"])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("amount=%d price_v=%d\n", amount, priceVersion)

	// A key that isn't a Symbol is rejected, not dropped.
	_, err = DecodeMap(ScvMap([]xdr.ScMapEntry{
		{Key: ScvString("amount"), Val: ScvU64(1250)},
	}))
	fmt.Println("non-symbol key rejected:", err != nil)

	// Output:
	// amount=1250 price_v=7
	// non-symbol key rejected: true
}

func ExampleDecodeI128() {
	v, err := ScvI128(big.NewInt(-1_000_000))
	if err != nil {
		panic(err)
	}
	n, err := DecodeI128(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(n)
	// Output:
	// -1000000
}

func ExampleDecodeBytes() {
	got, err := DecodeBytes(ScvBytes([]byte{0xde, 0xad, 0xbe, 0xef}))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", got)
	// Output:
	// deadbeef
}

func TestScvU32_RoundTrip(t *testing.T) {
	for _, v := range []uint32{0, 1, 42, 1 << 31, 1<<32 - 1} {
		got, err := DecodeU32(ScvU32(v))
		if err != nil {
			t.Fatalf("DecodeU32(ScvU32(%d)) returned unexpected error: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip %d => %d", v, got)
		}
	}
}

func TestScvU64_RoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 1 << 63, 1<<64 - 1} {
		got, err := DecodeU64(ScvU64(v))
		if err != nil {
			t.Fatalf("DecodeU64(ScvU64(%d)) returned unexpected error: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip %d => %d", v, got)
		}
	}
}

func TestScvI64_RoundTrip(t *testing.T) {
	for _, v := range []int64{0, 1, -1, 1<<63 - 1, -(1 << 62)} {
		got, err := DecodeI64(ScvI64(v))
		if err != nil {
			t.Fatalf("DecodeI64(ScvI64(%d)) returned unexpected error: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip %d => %d", v, got)
		}
	}
}

func TestScvI32_RoundTrip(t *testing.T) {
	for _, v := range []int32{0, 1, -1, 1<<31 - 1, -(1 << 30)} {
		got, err := DecodeI32(ScvI32(v))
		if err != nil {
			t.Fatalf("DecodeI32(ScvI32(%d)) returned unexpected error: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip %d => %d", v, got)
		}
	}
}

func TestScvBool_RoundTrip(t *testing.T) {
	for _, v := range []bool{true, false} {
		got, err := DecodeBool(ScvBool(v))
		if err != nil {
			t.Fatalf("DecodeBool returned unexpected error: %v", err)
		}
		if got != v {
			t.Errorf("round trip %v => %v", v, got)
		}
	}
}

func TestScvBytes_RoundTrip(t *testing.T) {
	tests := [][]byte{nil, {}, {0x01}, make([]byte, 32)}
	for i := range tests[3] {
		tests[3][i] = byte(i)
	}
	for _, v := range tests {
		got, err := DecodeBytes(ScvBytes(v))
		if err != nil {
			t.Fatalf("DecodeBytes returned unexpected error: %v", err)
		}
		if len(got) != len(v) {
			t.Errorf("round trip len %d => len %d", len(v), len(got))
			continue
		}
		for i := range v {
			if got[i] != v[i] {
				t.Errorf("round trip byte %d: %x => %x", i, v, got)
				break
			}
		}
	}
}

func TestScvBytes_CopiesInput(t *testing.T) {
	b := []byte{1, 2, 3}
	v := ScvBytes(b)
	b[0] = 0xFF
	got, err := DecodeBytes(v)
	if err != nil {
		t.Fatalf("DecodeBytes returned unexpected error: %v", err)
	}
	if got[0] != 1 {
		t.Error("ScvBytes did not copy its input — mutating the source slice changed the encoded value")
	}
}

func TestScvSymbol_RoundTrip(t *testing.T) {
	for _, v := range []string{"", "a", "amount", "price_v", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"} {
		got, err := DecodeSymbol(ScvSymbol(v))
		if err != nil {
			t.Fatalf("DecodeSymbol returned unexpected error: %v", err)
		}
		if got != v {
			t.Errorf("round trip %q => %q", v, got)
		}
	}
}

func TestScvString_RoundTrip(t *testing.T) {
	for _, v := range []string{"", "hello world", "https://example.com/schedule.json"} {
		got, err := DecodeString(ScvString(v))
		if err != nil {
			t.Fatalf("DecodeString returned unexpected error: %v", err)
		}
		if got != v {
			t.Errorf("round trip %q => %q", v, got)
		}
	}
}

func TestScvI128_RoundTrip(t *testing.T) {
	tests := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(-1),
		big.NewInt(123456789),
		i128Min,
		i128Max,
		new(big.Int).Add(i128Min, big.NewInt(1)),
		new(big.Int).Sub(i128Max, big.NewInt(1)),
	}
	for _, v := range tests {
		scv, err := ScvI128(v)
		if err != nil {
			t.Fatalf("ScvI128(%s) returned unexpected error: %v", v, err)
		}
		got, err := DecodeI128(scv)
		if err != nil {
			t.Fatalf("DecodeI128 returned unexpected error: %v", err)
		}
		if got.Cmp(v) != 0 {
			t.Errorf("round trip %s => %s", v, got)
		}
	}
}

func TestScvI128_RejectsOutOfRange(t *testing.T) {
	tooLarge := new(big.Int).Add(i128Max, big.NewInt(1))
	tooSmall := new(big.Int).Sub(i128Min, big.NewInt(1))
	for _, v := range []*big.Int{tooLarge, tooSmall, nil} {
		if _, err := ScvI128(v); err == nil {
			t.Errorf("ScvI128(%v) returned nil error, want out-of-range error", v)
		}
	}
}

func TestScvAddress_RoundTrip_Account(t *testing.T) {
	// The strkey encoding of the raw ed25519 key 0x00..0x1f — see
	// internal/merkle/leaf_test.go, same derivation.
	addr := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
	scv, err := ScvAddress(addr)
	if err != nil {
		t.Fatalf("ScvAddress returned unexpected error: %v", err)
	}
	got, err := DecodeAddress(scv)
	if err != nil {
		t.Fatalf("DecodeAddress returned unexpected error: %v", err)
	}
	if got != addr {
		t.Errorf("round trip %q => %q", addr, got)
	}
}

func TestScvAddress_RoundTrip_Contract(t *testing.T) {
	// A real deployed contract address (statement_registry, testnet — see
	// CLAUDE.md §4), to exercise the C... branch against a genuine strkey.
	addr := "CB75TTWGP3TLKEDGA2WOLEVCNKLUX6X5KS47GMWGBHUAVES7J55LY25M"
	scv, err := ScvAddress(addr)
	if err != nil {
		t.Fatalf("ScvAddress returned unexpected error: %v", err)
	}
	got, err := DecodeAddress(scv)
	if err != nil {
		t.Fatalf("DecodeAddress returned unexpected error: %v", err)
	}
	if got != addr {
		t.Errorf("round trip %q => %q", addr, got)
	}
}

func TestScvAddress_RejectsInvalid(t *testing.T) {
	for _, addr := range []string{"", "not-a-strkey", "S" + strings.Repeat("A", 55)} {
		if _, err := ScvAddress(addr); err == nil {
			t.Errorf("ScvAddress(%q) returned nil error", addr)
		}
	}
}

func TestScvVec_RoundTrip(t *testing.T) {
	items := []xdr.ScVal{ScvU32(1), ScvSymbol("two"), ScvBool(true)}
	scv := ScvVec(items)
	got, err := DecodeVec(scv)
	if err != nil {
		t.Fatalf("DecodeVec returned unexpected error: %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("got %d items, want %d", len(got), len(items))
	}
	if v, err := DecodeU32(got[0]); err != nil || v != 1 {
		t.Errorf("items[0] = %v, %v", v, err)
	}
	if v, err := DecodeSymbol(got[1]); err != nil || v != "two" {
		t.Errorf("items[1] = %v, %v", v, err)
	}
	if v, err := DecodeBool(got[2]); err != nil || v != true {
		t.Errorf("items[2] = %v, %v", v, err)
	}
}

func TestScvVec_Empty(t *testing.T) {
	got, err := DecodeVec(ScvVec(nil))
	if err != nil {
		t.Fatalf("DecodeVec returned unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d items, want 0", len(got))
	}
}

func TestScvMap_RoundTrip(t *testing.T) {
	entries := []xdr.ScMapEntry{
		MapEntry("amount", ScvU64(100)),
		MapEntry("consumer", ScvSymbol("placeholder")),
	}
	scv := ScvMap(entries)
	got, err := DecodeMap(scv)
	if err != nil {
		t.Fatalf("DecodeMap returned unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if v, err := DecodeU64(got["amount"]); err != nil || v != 100 {
		t.Errorf("map[amount] = %v, %v", v, err)
	}
	if v, err := DecodeSymbol(got["consumer"]); err != nil || v != "placeholder" {
		t.Errorf("map[consumer] = %v, %v", v, err)
	}
}

func ExampleScvBool() {
	value := ScvBool(true)
	fmt.Println(*value.B)
	// Output: true
}

func ExampleScvI128() {
	value, err := ScvI128(big.NewInt(123456789))
	if err != nil {
		log.Fatal(err)
	}
	decoded, err := DecodeI128(value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(decoded)
	// Output: 123456789
}

func ExampleScvMap() {
	value := ScvMap([]xdr.ScMapEntry{
		MapEntry("amount", ScvU64(42)),
		MapEntry("units", ScvU64(3)),
	})
	decoded, err := DecodeMap(value)
	if err != nil {
		log.Fatal(err)
	}
	amount, err := DecodeU64(decoded["amount"])
	if err != nil {
		log.Fatal(err)
	}
	units, err := DecodeU64(decoded["units"])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("amount: %d units: %d\n", amount, units)
	// Output: amount: 42 units: 3
}

func ExampleScvString() {
	value := ScvString("monthly")
	fmt.Println(*value.Str)
	// Output: monthly
}

func TestDecode_WrongTypeReturnsTypedError(t *testing.T) {
	_, err := DecodeU32(ScvBool(true))
	if err == nil {
		t.Fatal("DecodeU32 returned nil error for a Bool value")
	}
	var typeErr *ErrWrongScValType
	if !errors.As(err, &typeErr) {
		t.Errorf("error %v is not *ErrWrongScValType", err)
	}
}

func FuzzDecodeScValHelpers(f *testing.F) {
	f.Add(make([]byte, 16))
	minimum := make([]byte, 16)
	minimum[0] = 0x80
	f.Add(minimum)
	maximum := bytes.Repeat([]byte{0xff}, 16)
	maximum[0] = 0x7f
	f.Add(maximum)
	f.Add([]byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var valueBytes [16]byte
		copy(valueBytes[:], data)
		wantI128 := new(big.Int).SetBytes(valueBytes[:])
		if valueBytes[0]&0x80 != 0 {
			wantI128.Sub(wantI128, new(big.Int).Lsh(big.NewInt(1), 128))
		}
		i128Val, err := ScvI128(wantI128)
		if err != nil {
			t.Fatalf("ScvI128(%s): %v", wantI128, err)
		}
		address := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
		addressVal, err := ScvAddress(address)
		if err != nil {
			t.Fatalf("ScvAddress: %v", err)
		}
		vecVal := ScvVec([]xdr.ScVal{ScvU32(uint32(len(data)))})
		mapVal := ScvMap([]xdr.ScMapEntry{MapEntry("value", ScvBytes(data))})
		values := []xdr.ScVal{
			ScvBool(len(data)%2 == 0),
			ScvU32(uint32(len(data))),
			ScvI32(int32(len(data))),
			ScvU64(uint64(len(data))),
			ScvI64(int64(len(data))),
			i128Val,
			ScvBytes(data),
			ScvSymbol("fuzz"),
			ScvString(string(data)),
			addressVal,
			vecVal,
			mapVal,
		}

		decodedBool, err := DecodeBool(values[0])
		if err != nil || decodedBool != (len(data)%2 == 0) {
			t.Fatalf("DecodeBool matching value = %v, %v", decodedBool, err)
		}
		decodedU32, err := DecodeU32(values[1])
		if err != nil || decodedU32 != uint32(len(data)) {
			t.Fatalf("DecodeU32 matching value = %d, %v", decodedU32, err)
		}
		decodedI32, err := DecodeI32(values[2])
		if err != nil || decodedI32 != int32(len(data)) {
			t.Fatalf("DecodeI32 matching value = %d, %v", decodedI32, err)
		}
		decodedU64, err := DecodeU64(values[3])
		if err != nil || decodedU64 != uint64(len(data)) {
			t.Fatalf("DecodeU64 matching value = %d, %v", decodedU64, err)
		}
		decodedI64, err := DecodeI64(values[4])
		if err != nil || decodedI64 != int64(len(data)) {
			t.Fatalf("DecodeI64 matching value = %d, %v", decodedI64, err)
		}
		decodedI128, err := DecodeI128(values[5])
		if err != nil || decodedI128.Cmp(wantI128) != 0 {
			t.Fatalf("DecodeI128 matching value = %v, %v; want %s", decodedI128, err, wantI128)
		}
		decodedBytes, err := DecodeBytes(values[6])
		if err != nil || !bytes.Equal(decodedBytes, data) {
			t.Fatalf("DecodeBytes matching value differs: %v", err)
		}
		decodedSymbol, err := DecodeSymbol(values[7])
		if err != nil || decodedSymbol != "fuzz" {
			t.Fatalf("DecodeSymbol matching value = %q, %v", decodedSymbol, err)
		}
		decodedString, err := DecodeString(values[8])
		if err != nil || decodedString != string(data) {
			t.Fatalf("DecodeString matching value differs: %v", err)
		}
		decodedAddress, err := DecodeAddress(values[9])
		if err != nil || decodedAddress != address {
			t.Fatalf("DecodeAddress matching value = %q, %v", decodedAddress, err)
		}
		decodedVec, err := DecodeVec(values[10])
		if err != nil || len(decodedVec) != 1 {
			t.Fatalf("DecodeVec matching value has %d elements: %v", len(decodedVec), err)
		}
		decodedMap, err := DecodeMap(values[11])
		if err != nil || len(decodedMap) != 1 {
			t.Fatalf("DecodeMap matching value has %d entries: %v", len(decodedMap), err)
		}

		mismatches := []struct {
			name   string
			decode func(xdr.ScVal) error
		}{
			{"Bool", func(v xdr.ScVal) error { _, err := DecodeBool(v); return err }},
			{"U32", func(v xdr.ScVal) error { _, err := DecodeU32(v); return err }},
			{"I32", func(v xdr.ScVal) error { _, err := DecodeI32(v); return err }},
			{"U64", func(v xdr.ScVal) error { _, err := DecodeU64(v); return err }},
			{"I64", func(v xdr.ScVal) error { _, err := DecodeI64(v); return err }},
			{"I128", func(v xdr.ScVal) error { _, err := DecodeI128(v); return err }},
			{"Bytes", func(v xdr.ScVal) error { _, err := DecodeBytes(v); return err }},
			{"Symbol", func(v xdr.ScVal) error { _, err := DecodeSymbol(v); return err }},
			{"String", func(v xdr.ScVal) error { _, err := DecodeString(v); return err }},
			{"Address", func(v xdr.ScVal) error { _, err := DecodeAddress(v); return err }},
			{"Vec", func(v xdr.ScVal) error { _, err := DecodeVec(v); return err }},
			{"Map", func(v xdr.ScVal) error { _, err := DecodeMap(v); return err }},
		}
		for _, mismatch := range mismatches {
			err := mismatch.decode(ScvVoid())
			var typeErr *ErrWrongScValType
			if !errors.As(err, &typeErr) {
				t.Errorf("Decode%s mismatch error = %v, want *ErrWrongScValType", mismatch.name, err)
			}
		}
	})
}

func TestMarshalScValBase64_RoundTrip(t *testing.T) {
	scv := ScvU32(42)
	encoded, err := MarshalScValBase64(scv)
	if err != nil {
		t.Fatalf("MarshalScValBase64 returned unexpected error: %v", err)
	}
	decoded, err := UnmarshalScValBase64(encoded)
	if err != nil {
		t.Fatalf("UnmarshalScValBase64 returned unexpected error: %v", err)
	}
	got, err := DecodeU32(decoded)
	if err != nil || got != 42 {
		t.Errorf("round trip via base64: got %d, %v", got, err)
	}
}

func TestUnmarshalScValBase64_RejectsGarbage(t *testing.T) {
	if _, err := UnmarshalScValBase64("not-valid-base64!!"); err == nil {
		t.Error("UnmarshalScValBase64 returned nil error for invalid base64")
	}
	if _, err := UnmarshalScValBase64("AAAA"); err == nil {
		t.Error("UnmarshalScValBase64 returned nil error for valid base64 that isn't a complete ScVal")
	}
}

func ExampleScvBytes() {
	value := ScvBytes([]byte{0x01, 0x02, 0x03})
	decoded, err := DecodeBytes(value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%x\n", decoded)
	// Output: 010203
}

func ExampleMapField() {
	value, err := MapField(map[string]xdr.ScVal{"amount": ScvU64(1250)}, "amount")
	if err != nil {
		log.Fatal(err)
	}
	amount, err := DecodeU64(value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(amount)
	// Output: 1250
}

func ExampleMapEntry() {
	entry := MapEntry("amount", ScvU64(1250))
	key, err := DecodeSymbol(entry.Key)
	if err != nil {
		log.Fatal(err)
	}
	amount, err := DecodeU64(entry.Val)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d\n", key, amount)
	// Output: amount: 1250
}

func ExampleMarshalScValBase64() {
	encoded, err := MarshalScValBase64(ScvU32(42))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(encoded == "AAAAAwAAACo=")
	// Output: true
}

// ExampleUnmarshalScValBase64 decodes "AAAAAwAAACo=" — the exact base64 a
// Soroban RPC response carries for ScVal::U32(42), and the exact value
// MarshalScValBase64 returns for ScvU32(42) — then reads it back with
// DecodeU32, the second half of any real RPC round trip.
func ExampleUnmarshalScValBase64() {
	val, err := UnmarshalScValBase64("AAAAAwAAACo=")
	if err != nil {
		log.Fatal(err)
	}
	n, err := DecodeU32(val)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(n)
	// Output: 42
}

// TestDecode_AgainstRealOnChainEvent decodes the exact base64 ScVal blobs
// captured live in testdata/stellar/get_events.json — a real Symbol
// ("fee"), a real Address, and a real I128 amount from a genuine testnet
// event — rather than only synthetic values this package generated itself.
func TestDecode_AgainstRealOnChainEvent(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "stellar", "get_events.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc struct {
		Result struct {
			Events []struct {
				Topic []string `json:"topic"`
				Value string   `json:"value"`
			} `json:"events"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(doc.Result.Events) == 0 {
		t.Fatal("fixture has no events")
	}
	event := doc.Result.Events[0]
	if len(event.Topic) < 2 {
		t.Fatalf("event has %d topics, want at least 2", len(event.Topic))
	}

	topic0, err := UnmarshalScValBase64(event.Topic[0])
	if err != nil {
		t.Fatalf("decode topic[0]: %v", err)
	}
	sym, err := DecodeSymbol(topic0)
	if err != nil {
		t.Fatalf("DecodeSymbol(topic[0]): %v", err)
	}
	if sym != "fee" {
		t.Errorf("topic[0] = %q, want %q", sym, "fee")
	}

	topic1, err := UnmarshalScValBase64(event.Topic[1])
	if err != nil {
		t.Fatalf("decode topic[1]: %v", err)
	}
	addr, err := DecodeAddress(topic1)
	if err != nil {
		t.Fatalf("DecodeAddress(topic[1]): %v", err)
	}
	if !strings.HasPrefix(addr, "G") || len(addr) != 56 {
		t.Errorf("topic[1] decoded to %q, doesn't look like a G... account address", addr)
	}

	value, err := UnmarshalScValBase64(event.Value)
	if err != nil {
		t.Fatalf("decode value: %v", err)
	}
	amount, err := DecodeI128(value)
	if err != nil {
		t.Fatalf("DecodeI128(value): %v", err)
	}
	if amount.Sign() < 0 {
		t.Errorf("amount = %s, want non-negative (this is a fee charge)", amount)
	}
}
