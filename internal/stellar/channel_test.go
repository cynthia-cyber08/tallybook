package stellar

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
)

// TestBuildCommitment_MatchesIndependentDerivation checks BuildCommitment's
// output against a value derived from scratch in Python directly against
// the ScVal wire format (RFC 4506 opaque/string encoding, the same
// approach internal/merkle/leaf_test.go uses) rather than through this
// package's own encoder — the closest substitute available for a live
// PrepareCommitment capture, which channel.go's doc comment explains
// wasn't obtainable.
func TestBuildCommitment_MatchesIndependentDerivation(t *testing.T) {
	contractID := "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
	passphrase := "Test SDF Network ; September 2015"

	got, err := BuildCommitment(contractID, passphrase, big.NewInt(12345))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	want := "0000001100000001000000040000000f00000006616d6f756e7400000000000a" +
		"000000000000000000000000000030390000000f000000076368616e6e656c00" +
		"0000001200000001" +
		"74823f92868c2eff9f394ed65c98b128b1773d9ce6b24f500704007658158172" +
		"0000000f00000006646f6d61696e00000000000f000000086368616e636d6d74" +
		"0000000f000000076e6574776f726b000000000d0000002" +
		"0cee0302d59844d32bdca915c8203dd44b33fbb7edc19051ea37abedf28ecd472"
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		t.Fatalf("bad test fixture hex: %v", err)
	}
	if hex.EncodeToString(got) != hex.EncodeToString(wantBytes) {
		t.Errorf("BuildCommitment =\n%s\nwant\n%s", hex.EncodeToString(got), hex.EncodeToString(wantBytes))
	}
}

func TestBuildCommitment_Deterministic(t *testing.T) {
	contractID := "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
	passphrase := "Test SDF Network ; September 2015"
	a, err := BuildCommitment(contractID, passphrase, big.NewInt(100))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	b, err := BuildCommitment(contractID, passphrase, big.NewInt(100))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	if hex.EncodeToString(a) != hex.EncodeToString(b) {
		t.Error("BuildCommitment is not deterministic for identical input")
	}
}

func TestBuildCommitment_DiffersByAmountChannelAndNetwork(t *testing.T) {
	contractID := "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
	otherContractID := "CB75TTWGP3TLKEDGA2WOLEVCNKLUX6X5KS47GMWGBHUAVES7J55LY25M"
	passphrase := "Test SDF Network ; September 2015"
	otherPassphrase := "Public Global Stellar Network ; September 2015"

	base, err := BuildCommitment(contractID, passphrase, big.NewInt(100))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	byAmount, err := BuildCommitment(contractID, passphrase, big.NewInt(200))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	byChannel, err := BuildCommitment(otherContractID, passphrase, big.NewInt(100))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}
	byNetwork, err := BuildCommitment(contractID, otherPassphrase, big.NewInt(100))
	if err != nil {
		t.Fatalf("BuildCommitment returned unexpected error: %v", err)
	}

	for name, other := range map[string][]byte{"amount": byAmount, "channel": byChannel, "network": byNetwork} {
		if hex.EncodeToString(base) == hex.EncodeToString(other) {
			t.Errorf("changing %s did not change the commitment", name)
		}
	}
}

func TestBuildCommitment_RejectsInvalidContractID(t *testing.T) {
	_, err := BuildCommitment("not-a-contract-address", "Test SDF Network ; September 2015", big.NewInt(1))
	if err == nil {
		t.Error("BuildCommitment accepted an invalid contract address")
	}
}

func TestBuildCommitment_RejectsNilAmount(t *testing.T) {
	_, err := BuildCommitment("CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW",
		"Test SDF Network ; September 2015", nil)
	if err == nil {
		t.Error("BuildCommitment accepted a nil amount")
	}
}

// The remaining tests in this file exercise Channel's RPC-calling paths
// against constructed-but-schema-accurate responses, not a genuine chain
// capture. That's not because the contract is broken — see channel.go's
// doc comment: it deploys and works fine once you know to give
// commitment_key as hex — it's simply that no live-deployed instance's
// fixtures were captured for this test file specifically. What's
// actually being tested here is this package's own request-building and
// response-decoding — the RPC round trip itself (SimulateCall,
// InvokeAndSubmit) is the same code already exercised live by
// pricebook_test.go and registry_test.go.

func TestChannel_Getters(t *testing.T) {
	from := "GAAACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB7JZX"
	to := "GCWV2YQTXWP72QTU5YX5FM237RCHY6REV6BL6TKBEFCQ5R2QCTX67VUG"
	token := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	tests := []struct {
		name     string
		function func(*Channel) (any, error)
		respXDR  func() (string, error)
		want     any
	}{
		{
			name:     "Token",
			function: func(c *Channel) (any, error) { return c.Token(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvAddress(token)
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: token,
		},
		{
			name:     "From",
			function: func(c *Channel) (any, error) { return c.From(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvAddress(from)
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: from,
		},
		{
			name:     "To",
			function: func(c *Channel) (any, error) { return c.To(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvAddress(to)
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: to,
		},
		{
			name:     "RefundWaitingPeriod",
			function: func(c *Channel) (any, error) { return c.RefundWaitingPeriod(context.Background()) },
			respXDR:  func() (string, error) { return MarshalScValBase64(ScvU32(50)) },
			want:     uint32(50),
		},
		{
			name:     "Deposited",
			function: func(c *Channel) (any, error) { return c.Deposited(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvI128(big.NewInt(1000))
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: big.NewInt(1000),
		},
		{
			name:     "Balance",
			function: func(c *Channel) (any, error) { return c.Balance(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvI128(big.NewInt(700))
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: big.NewInt(700),
		},
		{
			name:     "Withdrawn",
			function: func(c *Channel) (any, error) { return c.Withdrawn(context.Background()) },
			respXDR: func() (string, error) {
				v, err := ScvI128(big.NewInt(300))
				if err != nil {
					return "", err
				}
				return MarshalScValBase64(v)
			},
			want: big.NewInt(300),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xdrVal, err := tt.respXDR()
			if err != nil {
				t.Fatalf("build response xdr: %v", err)
			}
			srv := simulateResultServer(t, xdrVal)
			channel := NewChannel(NewClient(srv.URL, nil), "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW", "")

			got, err := tt.function(channel)
			if err != nil {
				t.Fatalf("%s returned unexpected error: %v", tt.name, err)
			}
			switch want := tt.want.(type) {
			case *big.Int:
				gotInt, ok := got.(*big.Int)
				if !ok || gotInt.Cmp(want) != 0 {
					t.Errorf("%s = %v, want %v", tt.name, got, want)
				}
			default:
				if got != tt.want {
					t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
				}
			}
		})
	}
}

func TestChannel_PrepareCommitment(t *testing.T) {
	commitment := []byte("fake-commitment-bytes-for-plumbing-test")
	xdrVal, err := MarshalScValBase64(ScvBytes(commitment))
	if err != nil {
		t.Fatalf("build response xdr: %v", err)
	}
	srv := simulateResultServer(t, xdrVal)
	channel := NewChannel(NewClient(srv.URL, nil), "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW", "")

	got, err := channel.PrepareCommitment(context.Background(), big.NewInt(500))
	if err != nil {
		t.Fatalf("PrepareCommitment returned unexpected error: %v", err)
	}
	if string(got) != string(commitment) {
		t.Errorf("PrepareCommitment = %q, want %q", got, commitment)
	}
}

// simulateResultServer returns an httptest.Server that answers any
// simulateTransaction call with a minimal success response whose single
// result is resultXDR (already base64-encoded ScVal), and getLatestLedger
// with a fixed sequence — enough for SimulateCall-based reads (the
// getters and PrepareCommitment above), not for a full InvokeAndSubmit
// write path.
func simulateResultServer(t *testing.T, resultXDR string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "simulateTransaction":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"latestLedger":1000,"minResourceFee":"100","results":[{"auth":[],"xdr":%q}]}}`, resultXDR)
		default:
			http.Error(w, "unexpected method "+req.Method, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ExampleChannel_Balance demonstrates how to query the current token balance
// of a payment channel.
//
// There is deliberately no // Output: comment. Balance requires a live Soroban
// RPC endpoint to simulate the read call on chain. Without an Output comment,
// go test compiles this example to protect against signature drift without
// executing it or opening a live network connection.
func ExampleChannel_Balance() {
	const (
		contractID        = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
		networkPassphrase = "Test SDF Network ; September 2015"
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	channel := NewChannel(NewClient("https://soroban-testnet.stellar.org", nil), contractID, networkPassphrase)
	balance, err := channel.Balance(ctx)
	if err != nil {
		fmt.Println("balance:", err)
		return
	}
	fmt.Println("channel balance:", balance)
}

// ExampleChannel_Close shows the call shape of Channel.Close: authorized by
// the recipient, it closes the channel using a signed cumulative commitment,
// withdrawing the unsettled difference to the recipient and attempting to
// refund the funder.
//
// There is deliberately no // Output: comment. Close submits a transaction
// to a live Soroban RPC endpoint and requires an active channel, a funded
// recipient signer, and a valid funder signature. Without an Output comment,
// go test compiles this example to protect against signature drift without
// executing it or opening a live network connection.
func ExampleChannel_Close() {
	const (
		contractID        = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
		networkPassphrase = "Test SDF Network ; September 2015"
	)
	recipient, err := keypair.FromRawSeed([32]byte{1})
	if err != nil {
		fmt.Println("recipient key:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	channel := NewChannel(NewClient("https://soroban-testnet.stellar.org", nil), contractID, networkPassphrase)
	amount := big.NewInt(10_000)
	var sig [64]byte

	hash, err := channel.Close(ctx, recipient, amount, sig)
	if err != nil {
		fmt.Println("close:", err)
		return
	}
	fmt.Println("close submitted:", hash)
}

// ExampleChannel_CloseStart shows the call shape of Channel.CloseStart: the
// funder initiates closing the channel, which becomes effective after
// RefundWaitingPeriod ledgers.
//
// There is deliberately no // Output: comment. CloseStart submits an on-chain
// transaction via Soroban RPC and requires a live network, an active channel,
// and a funded funder account. Without an Output comment, go test compiles
// this example to protect against signature drift without executing it or
// opening a live network connection.
func ExampleChannel_CloseStart() {
	const (
		contractID        = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
		networkPassphrase = "Test SDF Network ; September 2015"
	)
	funder, err := keypair.FromRawSeed([32]byte{1})
	if err != nil {
		fmt.Println("funder key:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	channel := NewChannel(NewClient("https://soroban-testnet.stellar.org", nil), contractID, networkPassphrase)
	hash, err := channel.CloseStart(ctx, funder)
	if err != nil {
		fmt.Println("close_start:", err)
		return
	}
	fmt.Println("close_start submitted:", hash)
}

// ExampleChannel_Deposited demonstrates how to query the cumulative total
// amount ever deposited into a payment channel.
//
// There is deliberately no // Output: comment. Deposited requires a live
// Soroban RPC endpoint to simulate the read call on chain. Without an Output
// comment, go test compiles this example to protect against signature drift
// without executing it or opening a live network connection.
func ExampleChannel_Deposited() {
	const (
		contractID        = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
		networkPassphrase = "Test SDF Network ; September 2015"
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	channel := NewChannel(NewClient("https://soroban-testnet.stellar.org", nil), contractID, networkPassphrase)
	deposited, err := channel.Deposited(ctx)
	if err != nil {
		fmt.Println("deposited:", err)
		return
	}
	fmt.Println("total deposited:", deposited)
}
