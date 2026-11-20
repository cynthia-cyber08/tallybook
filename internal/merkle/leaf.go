// Package merkle builds the usage-root merkle tree the collector anchors on
// chain and the leaf/proof format the statement_registry contract verifies
// against. Every formula here must match the Rust contract byte for byte —
// see CLAUDE.md §4 — because a divergence makes every proof this package
// produces invalid on chain, not just locally wrong.
package merkle

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// i128 bounds: [-2^127, 2^127 - 1].
var (
	i128Min = new(big.Int).Lsh(big.NewInt(-1), 127)
	i128Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
)

// Record is one billed request in the form the statement_registry
// contract's merkle leaf format requires. Field names mirror the contract's
// map keys (§4); this is the only place that formula is implemented.
type Record struct {
	Amount       *big.Int // charged amount, i128 range (integer stroops)
	Consumer     string   // G... account or C... contract strkey address
	EndpointHash [32]byte // sha256(method || " " || path_template); see EndpointHash
	Ledger       uint32   // settlement or observation ledger
	PriceVersion uint32
	RequestID    [32]byte
	Units        uint64
}

// EndpointHash computes the "endpoint" field of a merkle leaf record:
// sha256(method || " " || path_template). The meter package's pricing
// lookup and this package's leaf encoding must derive the same value from
// the same request, so it lives here as the single source of truth for
// both.
func EndpointHash(method, pathTemplate string) [32]byte {
	return sha256.Sum256([]byte(method + " " + pathTemplate))
}

// Leaf encodes r as the contract's ScVal::Map record and returns
// sha256(sha256(record_bytes)) — the leaf hash the contract expects, which
// is a double sha256 of the XDR-encoded record, not a single one.
func Leaf(r Record) ([32]byte, error) {
	recordBytes, err := encodeRecord(r)
	if err != nil {
		return [32]byte{}, fmt.Errorf("merkle: encode record: %w", err)
	}
	first := sha256.Sum256(recordBytes)
	return sha256.Sum256(first[:]), nil
}

// encodeRecord XDR-encodes r as ScVal::Map, with entries in the exact key
// order the contract requires: alphabetical by map key ("amount",
// "consumer", "endpoint", "ledger", "price_v", "reqid", "units"). Soroban's
// host requires map keys in this sorted order; an out-of-order map is not
// just a style mismatch, it fails to decode on chain.
func encodeRecord(r Record) ([]byte, error) {
	amount, err := scvI128(r.Amount)
	if err != nil {
		return nil, fmt.Errorf("amount: %w", err)
	}
	consumer, err := scvAddress(r.Consumer)
	if err != nil {
		return nil, fmt.Errorf("consumer: %w", err)
	}

	m := xdr.ScMap{
		mapEntry("amount", amount),
		mapEntry("consumer", consumer),
		mapEntry("endpoint", scvBytesN32(r.EndpointHash)),
		mapEntry("ledger", scvU32(r.Ledger)),
		mapEntry("price_v", scvU32(r.PriceVersion)),
		mapEntry("reqid", scvBytesN32(r.RequestID)),
		mapEntry("units", scvU64(r.Units)),
	}
	mapPtr := &m
	val := xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &mapPtr}

	encoded, err := val.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal xdr: %w", err)
	}
	return encoded, nil
}

func mapEntry(key string, val xdr.ScVal) xdr.ScMapEntry {
	sym := xdr.ScSymbol(key)
	return xdr.ScMapEntry{
		Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym},
		Val: val,
	}
}

func scvU32(v uint32) xdr.ScVal {
	u := xdr.Uint32(v)
	return xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: &u}
}

func scvU64(v uint64) xdr.ScVal {
	u := xdr.Uint64(v)
	return xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &u}
}

func scvBytesN32(v [32]byte) xdr.ScVal {
	b := xdr.ScBytes(v[:])
	return xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &b}
}

// scvI128 converts a signed arbitrary-precision integer to ScVal::I128,
// two's-complement encoded across the Hi/Lo 64-bit halves as the contract
// expects.
func scvI128(v *big.Int) (xdr.ScVal, error) {
	if v == nil {
		return xdr.ScVal{}, fmt.Errorf("nil amount")
	}
	if v.Cmp(i128Min) < 0 || v.Cmp(i128Max) > 0 {
		return xdr.ScVal{}, fmt.Errorf("value %s out of i128 range", v)
	}

	unsigned := new(big.Int).Set(v)
	if v.Sign() < 0 {
		modulus := new(big.Int).Lsh(big.NewInt(1), 128)
		unsigned.Add(modulus, v)
	}

	var buf [16]byte
	unsigned.FillBytes(buf[:])
	hi := xdr.Int64(uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
		uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7]))
	lo := xdr.Uint64(uint64(buf[8])<<56 | uint64(buf[9])<<48 | uint64(buf[10])<<40 | uint64(buf[11])<<32 |
		uint64(buf[12])<<24 | uint64(buf[13])<<16 | uint64(buf[14])<<8 | uint64(buf[15]))

	parts := xdr.Int128Parts{Hi: hi, Lo: lo}
	return xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &parts}, nil
}

// scvAddress converts a G... account or C... contract strkey address to
// ScVal::Address. The wire format differs by type:
//   - account: ScAddressType=0, then PublicKeyType=0, then 32-byte ed25519 key (36 bytes)
//   - contract: ScAddressType=1, then 32-byte contract id — no inner type word (32 bytes)
//
// A contract-consumer record is 4 bytes shorter than the account equivalent.
func scvAddress(address string) (xdr.ScVal, error) {
	if len(address) == 0 {
		return xdr.ScVal{}, fmt.Errorf("consumer: empty address")
	}
	switch address[0] {
	case 'G':
		raw, err := strkey.Decode(strkey.VersionByteAccountID, address)
		if err != nil {
			return xdr.ScVal{}, fmt.Errorf("decode strkey address %q: %w", address, err)
		}
		if len(raw) != 32 {
			return xdr.ScVal{}, fmt.Errorf("decoded address %q is %d bytes, want 32", address, len(raw))
		}
		var raw32 xdr.Uint256
		copy(raw32[:], raw)
		accountID := xdr.AccountId(xdr.PublicKey{
			Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
			Ed25519: &raw32,
		})
		scAddr := xdr.ScAddress{
			Type:      xdr.ScAddressTypeScAddressTypeAccount,
			AccountId: &accountID,
		}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &scAddr}, nil
	case 'C':
		raw, err := strkey.Decode(strkey.VersionByteContract, address)
		if err != nil {
			return xdr.ScVal{}, fmt.Errorf("decode strkey address %q: %w", address, err)
		}
		if len(raw) != 32 {
			return xdr.ScVal{}, fmt.Errorf("decoded address %q is %d bytes, want 32", address, len(raw))
		}
		var contractID xdr.ContractId
		copy(contractID[:], raw)
		scAddr := xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &contractID,
		}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &scAddr}, nil
	default:
		return xdr.ScVal{}, fmt.Errorf("address %q: unrecognized strkey prefix, want G... account or C... contract", address)
	}
}
