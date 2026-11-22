package stellar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func ExampleErrSimulationFailed_Error() {
	err := &ErrSimulationFailed{Message: "contract failed"}
	fmt.Println(err.Error())
	// Output:
	// stellar: simulation failed: contract failed
}

func ExampleInvokeAndSubmitWithAuth() {
	const (
		contractID        = "CB75TTWGP3TLKEDGA2WOLEVCNKLUX6X5KS47GMWGBHUAVES7J55LY25M"
		networkPassphrase = "Test SDF Network ; September 2015"
	)
	operator, err := keypair.FromRawSeed([32]byte{1})
	if err != nil {
		log.Fatal(err)
	}
	consumer, err := keypair.FromRawSeed([32]byte{2})
	if err != nil {
		log.Fatal(err)
	}
	operatorAddress, err := ScvAddress(operator.Address())
	if err != nil {
		log.Fatal(err)
	}
	creditedAmount, err := ScvI128(big.NewInt(0))
	if err != nil {
		log.Fatal(err)
	}
	resolutionHash := [32]byte{}
	args := []xdr.ScVal{operatorAddress, ScvU64(1), ScvBytes(resolutionHash[:]), creditedAmount}
	consumerAuth, err := AuthorizeInvocation(networkPassphrase, consumer, contractID, "resolve_dispute", args, 123)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient("https://soroban-testnet.stellar.org", nil)
	_, _, err = InvokeAndSubmitWithAuth(ctx, client, networkPassphrase, operator, contractID, "resolve_dispute", args, []xdr.SorobanAuthorizationEntry{consumerAuth})
	fmt.Println(errors.Is(err, context.Canceled))
	// Output: true
}

// ExampleErrTransactionFailed_Error constructs an ErrTransactionFailed with
// a concrete literal transaction hash and result XDR, then calls its Error()
// method and prints the formatted string.
func ExampleAuthorizeInvocation() {
	const (
		contractID        = "CB75TTWGP3TLKEDGA2WOLEVCNKLUX6X5KS47GMWGBHUAVES7J55LY25M"
		networkPassphrase = "Test SDF Network ; September 2015"
	)
	signer, err := keypair.FromRawSeed([32]byte{1})
	if err != nil {
		log.Fatal(err)
	}
	entry, err := AuthorizeInvocation(networkPassphrase, signer, contractID, "close", nil, 1000)
	if err != nil {
		log.Fatal(err)
	}
	// Credentials type is deterministic; the nonce is random each call.
	fmt.Println(entry.Credentials.Type == xdr.SorobanCredentialsTypeSorobanCredentialsAddress)
	fmt.Println(string(entry.RootInvocation.Function.ContractFn.FunctionName))
	// Output:
	// true
	// close
}

func ExampleErrTransactionFailed_Error() {
	err := &ErrTransactionFailed{
		Hash:      "83806937259d185c694da8e4efd7d8e7e859c8df235f8f77895f244c999068d3",
		ResultXDR: "AAAAAAAAAGT////+",
	}
	fmt.Println(err.Error())
	// Output:
	// stellar: transaction 83806937259d185c694da8e4efd7d8e7e859c8df235f8f77895f244c999068d3 failed on chain
}

// ExampleBuildCommitment shows the deterministic XDR wire format produced for
// a concrete contract address, network passphrase, and amount. The output is
// 192 bytes (12-byte ScMap header, four keyed ScVal entries: I128 amount,
// Address channel, Symbol domain, BytesN<32> network passphrase hash).
func ExampleBuildCommitment() {
	const (
		contractID  = "CB2IEP4SQ2GC5747HFHNMXEYWEULC5Z5TTTLET2QA4CAA5SYCWAXFKAW"
		networkPass = "Test SDF Network ; September 2015"
	)
	b, err := BuildCommitment(contractID, networkPass, big.NewInt(12345))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(b))
	// Output:
	// 192
}
