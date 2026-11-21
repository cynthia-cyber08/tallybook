package stellar

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// baseFee is the classic (non-resource) fee, in stroops, for a
// single-operation transaction. A real submission pays this plus the
// resource fee simulation reports.
const baseFee = 100

// pollInterval and pollTimeout bound how long InvokeAndSubmit waits for a
// submitted transaction to settle before giving up.
const (
	pollInterval = 2 * time.Second
	pollTimeout  = 30 * time.Second
)

// ErrSimulationFailed wraps simulateTransaction's own Error field — a
// simulation that ran but rejected the call, distinct from an RPCError
// (which means the request itself was malformed).
type ErrSimulationFailed struct{ Message string }

func (e *ErrSimulationFailed) Error() string {
	return fmt.Sprintf("stellar: simulation failed: %s", e.Message)
}

// ErrTransactionFailed indicates a transaction was accepted and settled,
// but with FAILED status rather than SUCCESS.
type ErrTransactionFailed struct {
	Hash      string
	ResultXDR string
}

func (e *ErrTransactionFailed) Error() string {
	return fmt.Sprintf("stellar: transaction %s failed on chain", e.Hash)
}

// SimulateCall builds a placeholder read-only transaction invoking
// function on the contract at contractID with args, and returns the
// decoded ScVal it returns. sourceAccount need not be funded or even
// exist — the RPC node's simulateTransaction never touches real account
// state for a call shaped like this, so callers may pass any well-formed
// G... address for a pure read (verified against a live testnet node; see
// pricebook_test.go).
func SimulateCall(ctx context.Context, client *Client, sourceAccount, contractID, function string, args []xdr.ScVal) (xdr.ScVal, error) {
	muxed, err := muxedAccount(sourceAccount)
	if err != nil {
		return xdr.ScVal{}, err
	}
	op, err := buildContractCallOperation(contractID, function, args, nil)
	if err != nil {
		return xdr.ScVal{}, err
	}

	tx := xdr.Transaction{
		SourceAccount: muxed,
		Fee:           baseFee,
		SeqNum:        1,
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{op},
		Ext:           xdr.TransactionExt{V: 0},
	}
	envelopeB64, err := marshalUnsignedEnvelope(tx)
	if err != nil {
		return xdr.ScVal{}, err
	}

	result, err := client.SimulateTransaction(ctx, SimulateTransactionParams{Transaction: envelopeB64})
	if err != nil {
		return xdr.ScVal{}, fmt.Errorf("stellar: simulate %s: %w", function, err)
	}
	if result.Error != "" {
		return xdr.ScVal{}, &ErrSimulationFailed{Message: result.Error}
	}
	if len(result.Results) == 0 {
		return xdr.ScVal{}, fmt.Errorf("stellar: simulate %s: no results", function)
	}
	return UnmarshalScValBase64(result.Results[0].XDR)
}

// InvokeAndSubmit performs a full write-path contract call authorized by
// signer: fetches signer's account sequence number, simulates to obtain
// the resource footprint, fee, and the authorization entries the call
// needs, signs, submits, and polls until the transaction settles. Returns
// the contract call's real on-chain return value — decoded from the
// settled transaction's own result metadata, not the value simulation
// predicted, since ledger state (and so the result) can differ between
// simulating and actually landing — and the transaction hash.
//
// This covers every write in §4 authorized by a single party who is also
// the transaction's source account (publish, anchor, open_dispute,
// extend_statement_ttl, and every one-way-channel mutator). resolve_dispute
// needs a second party's authorization too; use InvokeAndSubmitWithAuth for
// that (registry.go's ResolveDispute does).
func InvokeAndSubmit(ctx context.Context, client *Client, networkPassphrase string, signer *keypair.Full, contractID, function string, args []xdr.ScVal) (xdr.ScVal, string, error) {
	return invokeAndSubmit(ctx, client, networkPassphrase, signer, contractID, function, args, nil)
}

// InvokeAndSubmitWithAuth is InvokeAndSubmit, plus extraAuth: authorization
// entries for parties other than signer that the call also needs — built
// with AuthorizeInvocation. Any authorization simulation records for the
// same address as an entry in extraAuth is superseded by that entry rather
// than submitted alongside it: a second, unsigned template entry for an
// address that's already properly authorized is not merely redundant, it
// makes the whole call fail — the host authenticates the first entry it
// finds for that address, and simulation's own template entries carry no
// signature at all. Verified live: submitting both entries for one
// address traps with diagnostic topics
// [error ScErrorTypeSceAuth(ScErrorCodeScecInvalidAction)] and data
// "failed account authentication with error <address>
// ScErrorTypeSceValue(ScErrorCodeScecUnexpectedType)" — the host tried to
// decode the unsigned entry's Void signature as the expected Vec. If you
// see that exact error, check for a duplicate entry first; it is not the
// footprint bug documented on invokeAndSubmit below, and not a credential
// type problem (see AuthorizeInvocation's doc comment).
func InvokeAndSubmitWithAuth(
	ctx context.Context,
	client *Client,
	networkPassphrase string,
	signer *keypair.Full,
	contractID, function string,
	args []xdr.ScVal,
	extraAuth []xdr.SorobanAuthorizationEntry,
) (xdr.ScVal, string, error) {
	return invokeAndSubmit(ctx, client, networkPassphrase, signer, contractID, function, args, extraAuth)
}

// AuthorizeInvocation builds a signed SorobanAuthorizationEntry authorizing
// signer for exactly the invocation of function on the contract at
// contractID with args — for a party other than the transaction's source
// account whose own authorization a call requires (resolve_dispute's
// consumer, per §4). currentLedger should be a recently fetched ledger
// sequence; the entry is valid until currentLedger+authValidityLedgers.
//
// Uses classic SorobanCredentialsTypeSorobanCredentialsAddress credentials
// (the format simulateTransaction's own recorded template entries use for
// a plain G... account on this network), signing a
// HashIdPreimageSorobanAuthorization payload the way js-stellar-base's
// authorizeEntry does: a one-element Vec containing a Map of
// {public_key, signature}, both raw bytes, both Symbol-keyed.
//
// Classic Address vs. CAP-71 AddressV2 credentials made no difference in
// testing — both authenticate identically; this function uses classic
// Address simply because it's what simulateTransaction's own template
// already uses, not because AddressV2 is broken or unsupported. An
// earlier revision of this doc comment claimed AddressV2 was required and
// that classic Address was the fix; that was wrong, and traced to
// conflating three separate bugs hit in the same debugging session (only
// one of which this function's design addresses):
//
//  1. Two authorization entries for the same address on one operation —
//     the unsigned template entry simulateTransaction itself returns,
//     left in place, plus a separately-built signed one appended
//     alongside it rather than in place of it. The host authenticates
//     whichever entry it finds first for that address; when that's the
//     unsigned one (Signature: Void), decoding it as the expected Vec
//     fails with "ScErrorCodeScecUnexpectedType". Fixed by
//     mergeAuthEntries, not by this function — de-duplicate by address,
//     never concatenate.
//  2. The transaction's resource footprint computed from a simulation
//     that never saw the real, final authorization entries — so the
//     footprint doesn't reserve access to the second party's nonce ledger
//     entry, and execution traps with "trying to access nonce outside of
//     the footprint" the instant require_auth() touches that nonce. Fixed
//     by invokeAndSubmit's second simulate pass, not by this function —
//     see its comment below. This was the actual, sole cause of the
//     original resolve_dispute failure.
//  3. Credential type (this function's choice of Address over AddressV2).
//     Isolated with a clean 2×2 matrix (credential type × footprint
//     consistency, a fresh dispute per cell) once bugs 1 and 2 above were
//     already fixed: AddressV2 succeeded exactly as often as Address did
//     (both cells with a consistent footprint succeeded; both cells
//     without one failed, with the byte-identical diagnostic in both
//     cases). Credential type predicts nothing. Full reproduction,
//     including live transaction hashes for all four cells, is the
//     session artifact at /tmp/cap71-finding.md.
//
// Multi-party authorization support across subsequent stellar-cli releases
// does not coordinate independent multi-signer payloads or the dual-pass
// footprint simulation required here; programmatic Go construction remains
// the definitive, permanent architecture for Tallybook's multi-auth calls.
func AuthorizeInvocation(
	networkPassphrase string,
	signer *keypair.Full,
	contractID, function string,
	args []xdr.ScVal,
	currentLedger uint32,
) (xdr.SorobanAuthorizationEntry, error) {
	contractAddr, err := scAddress(contractID)
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, fmt.Errorf("stellar: authorize invocation for contract %s: %w", contractID, err)
	}
	invocation := xdr.SorobanAuthorizedInvocation{
		Function: xdr.SorobanAuthorizedFunction{
			Type: xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
			ContractFn: &xdr.InvokeContractArgs{
				ContractAddress: contractAddr,
				FunctionName:    xdr.ScSymbol(function),
				Args:            args,
			},
		},
	}

	nonce, err := randomNonce()
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, fmt.Errorf("stellar: generate nonce: %w", err)
	}
	expirationLedger := currentLedger + authValidityLedgers

	networkID := network.ID(networkPassphrase)
	preimage := xdr.HashIdPreimage{
		Type: xdr.EnvelopeTypeEnvelopeTypeSorobanAuthorization,
		SorobanAuthorization: &xdr.HashIdPreimageSorobanAuthorization{
			NetworkId:                 xdr.Hash(networkID),
			Nonce:                     xdr.Int64(nonce),
			SignatureExpirationLedger: xdr.Uint32(expirationLedger),
			Invocation:                invocation,
		},
	}
	preimageBytes, err := preimage.MarshalBinary()
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, fmt.Errorf("stellar: marshal auth preimage: %w", err)
	}
	payloadHash := sha256.Sum256(preimageBytes)

	sig, err := signer.Sign(payloadHash[:])
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, fmt.Errorf("stellar: sign auth payload: %w", err)
	}

	signerAddr, err := scAddress(signer.Address())
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, err
	}
	_, rawPub, err := strkey.DecodeAny(signer.Address())
	if err != nil {
		return xdr.SorobanAuthorizationEntry{}, fmt.Errorf("stellar: decode signer address: %w", err)
	}
	pubKeyBytes := xdr.ScBytes(rawPub)
	sigBytes := xdr.ScBytes(sig)
	signatureVal := ScvVec([]xdr.ScVal{
		ScvMap([]xdr.ScMapEntry{
			MapEntry("public_key", xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &pubKeyBytes}),
			MapEntry("signature", xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &sigBytes}),
		}),
	})

	creds := xdr.SorobanAddressCredentials{
		Address:                   signerAddr,
		Nonce:                     xdr.Int64(nonce),
		SignatureExpirationLedger: xdr.Uint32(expirationLedger),
		Signature:                 signatureVal,
	}
	return xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type:    xdr.SorobanCredentialsTypeSorobanCredentialsAddress,
			Address: &creds,
		},
		RootInvocation: invocation,
	}, nil
}

// authValidityLedgers is how many ledgers past currentLedger an
// AuthorizeInvocation entry stays valid for — long enough to cover
// submission and confirmation latency, short enough to bound how long a
// leaked entry remains replayable.
const authValidityLedgers = 100

func randomNonce() (int64, error) {
	// 8 random bytes reinterpreted as int64, so the result is uniformly
	// distributed over the full signed 64-bit range and roughly half of all
	// draws are negative. The XDR field is an int64, so a negative value is
	// representable, and the host's own check is on nonce uniqueness rather
	// than sign.
	//
	// Whether the live host accepts a negative nonce has NOT been verified
	// from this repository, and CONTRIBUTING is explicit that a claim about
	// this package has to be checked against the live RPC surface or the real
	// contract source rather than asserted. #17 stays open on that point. An
	// earlier revision of this comment cited a Rust soroban-sdk test using
	// "a negative nonce (0xC0FFEE_i64)" as evidence; 0xC0FFEE is 12648430,
	// which is positive, so it did not show what it claimed to.
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b[:])), nil
}

// mergeAuthEntries combines recorded (simulation-predicted) authorization
// entries with extraAuth, letting extraAuth entries supersede a recorded
// entry for the same address rather than sit alongside it.
func mergeAuthEntries(recorded, extra []xdr.SorobanAuthorizationEntry) []xdr.SorobanAuthorizationEntry {
	superseded := make(map[string]bool, len(extra))
	for _, e := range extra {
		if key, ok := authCredentialsAddressKey(e.Credentials); ok {
			superseded[key] = true
		}
	}
	merged := make([]xdr.SorobanAuthorizationEntry, 0, len(recorded)+len(extra))
	for _, e := range recorded {
		if key, ok := authCredentialsAddressKey(e.Credentials); ok && superseded[key] {
			continue
		}
		merged = append(merged, e)
	}
	return append(merged, extra...)
}

// authCredentialsAddressKey returns a comparable key for the address an
// Address or AddressV2 credential authorizes, and false for
// SourceAccount (which has no address of its own to key by).
func authCredentialsAddressKey(creds xdr.SorobanCredentials) (string, bool) {
	var addr *xdr.ScAddress
	switch creds.Type {
	case xdr.SorobanCredentialsTypeSorobanCredentialsAddress:
		if creds.Address == nil {
			return "", false
		}
		addr = &creds.Address.Address
	case xdr.SorobanCredentialsTypeSorobanCredentialsAddressV2:
		if creds.AddressV2 == nil {
			return "", false
		}
		addr = &creds.AddressV2.Address
	default:
		return "", false
	}
	b, err := addr.MarshalBinary()
	if err != nil {
		return "", false
	}
	return string(b), true
}

func invokeAndSubmit(
	ctx context.Context,
	client *Client,
	networkPassphrase string,
	signer *keypair.Full,
	contractID, function string,
	args []xdr.ScVal,
	extraAuth []xdr.SorobanAuthorizationEntry,
) (xdr.ScVal, string, error) {
	source := signer.Address()

	seqNum, err := fetchSequenceNumber(ctx, client, source)
	if err != nil {
		return xdr.ScVal{}, "", err
	}

	muxed, err := muxedAccount(source)
	if err != nil {
		return xdr.ScVal{}, "", err
	}

	// First pass: simulate with no auth attached, to learn what
	// authorization and resources the call needs.
	unsignedOp, err := buildContractCallOperation(contractID, function, args, nil)
	if err != nil {
		return xdr.ScVal{}, "", err
	}
	simTx := xdr.Transaction{
		SourceAccount: muxed,
		Fee:           baseFee,
		SeqNum:        xdr.SequenceNumber(seqNum + 1),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{unsignedOp},
		Ext:           xdr.TransactionExt{V: 0},
	}
	envelopeB64, err := marshalUnsignedEnvelope(simTx)
	if err != nil {
		return xdr.ScVal{}, "", err
	}

	simResult, err := client.SimulateTransaction(ctx, SimulateTransactionParams{Transaction: envelopeB64})
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: simulate %s: %w", function, err)
	}
	if simResult.Error != "" {
		return xdr.ScVal{}, "", &ErrSimulationFailed{Message: simResult.Error}
	}
	if len(simResult.Results) == 0 {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: simulate %s: no results", function)
	}

	recordedAuth, err := decodeAuthEntries(simResult.Results[0].Auth)
	if err != nil {
		return xdr.ScVal{}, "", err
	}
	allAuth := mergeAuthEntries(recordedAuth, extraAuth)

	finalOp, err := buildContractCallOperation(contractID, function, args, allAuth)
	if err != nil {
		return xdr.ScVal{}, "", err
	}

	// When extraAuth adds an authorizer the first, auth-less simulation
	// never saw, that simulation's footprint doesn't reserve access to
	// that authorizer's nonce ledger entry — the transaction would trap
	// with "trying to access nonce outside of the footprint" despite a
	// perfectly valid signature. Re-simulate with the real, final auth
	// list attached whenever extraAuth is non-empty, so the footprint
	// this call actually submits accounts for every authorizer, not just
	// signer.
	//
	// This — not credential type — is the actual, sole cause of the
	// resolve_dispute failure an earlier revision of this codebase
	// misattributed to CAP-71 AddressV2 credentials. Verified with a
	// clean 2×2 matrix (credential type × whether this re-simulation
	// happens, a fresh dispute per cell, on the real statement_registry
	// testnet contract): both credential types succeeded whenever this
	// re-simulation ran, and both failed identically — same diagnostic,
	// same "trying to access nonce outside of the footprint" — whenever
	// it didn't. See /tmp/cap71-finding.md (session artifact, not
	// committed) for the full reproduction with live transaction hashes
	// for all four cells.
	if len(extraAuth) > 0 {
		finalSimTx := simTx
		finalSimTx.Operations = []xdr.Operation{finalOp}
		finalEnvelopeB64, err := marshalUnsignedEnvelope(finalSimTx)
		if err != nil {
			return xdr.ScVal{}, "", err
		}
		simResult, err = client.SimulateTransaction(ctx, SimulateTransactionParams{Transaction: finalEnvelopeB64})
		if err != nil {
			return xdr.ScVal{}, "", fmt.Errorf("stellar: re-simulate %s with full auth: %w", function, err)
		}
		if simResult.Error != "" {
			return xdr.ScVal{}, "", &ErrSimulationFailed{Message: simResult.Error}
		}
		if len(simResult.Results) == 0 {
			return xdr.ScVal{}, "", fmt.Errorf("stellar: re-simulate %s with full auth: no results", function)
		}
	}

	var sorobanData xdr.SorobanTransactionData
	sorobanDataBytes, err := base64.StdEncoding.DecodeString(simResult.TransactionData)
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: decode transaction data: %w", err)
	}
	if err := sorobanData.UnmarshalBinary(sorobanDataBytes); err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: unmarshal transaction data: %w", err)
	}

	var minResourceFee int64
	if simResult.MinResourceFee != "" {
		if _, err := fmt.Sscanf(simResult.MinResourceFee, "%d", &minResourceFee); err != nil {
			return xdr.ScVal{}, "", fmt.Errorf("stellar: parse minResourceFee %q: %w", simResult.MinResourceFee, err)
		}
	}

	finalTx := xdr.Transaction{
		SourceAccount: muxed,
		Fee:           xdr.Uint32(baseFee + minResourceFee),
		SeqNum:        xdr.SequenceNumber(seqNum + 1),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{finalOp},
		Ext:           xdr.TransactionExt{V: 1, SorobanData: &sorobanData},
	}

	hash, err := network.HashTransaction(finalTx, networkPassphrase)
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: hash transaction: %w", err)
	}
	sig, err := signer.SignDecorated(hash[:])
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: sign transaction: %w", err)
	}

	v1 := xdr.TransactionV1Envelope{Tx: finalTx, Signatures: []xdr.DecoratedSignature{sig}}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &v1}
	envBytes, err := env.MarshalBinary()
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: marshal signed envelope: %w", err)
	}

	sendResult, err := client.SendTransaction(ctx, base64.StdEncoding.EncodeToString(envBytes))
	if err != nil {
		return xdr.ScVal{}, "", fmt.Errorf("stellar: submit %s: %w", function, err)
	}
	switch sendResult.Status {
	case SendTransactionStatusError:
		return xdr.ScVal{}, sendResult.Hash, fmt.Errorf("stellar: submit %s: rejected: %s", function, sendResult.ErrorResultXDR)
	case SendTransactionStatusPending, SendTransactionStatusDuplicate:
		// Accepted for inclusion; poll for the outcome below.
	default:
		return xdr.ScVal{}, sendResult.Hash, fmt.Errorf("stellar: submit %s: unexpected status %s", function, sendResult.Status)
	}

	txResult, err := pollTransaction(ctx, client, sendResult.Hash)
	if err != nil {
		return xdr.ScVal{}, sendResult.Hash, err
	}
	if txResult.Status != GetTransactionStatusSuccess {
		return xdr.ScVal{}, sendResult.Hash, &ErrTransactionFailed{Hash: sendResult.Hash, ResultXDR: txResult.ResultXDR}
	}

	returnVal, err := decodeReturnValue(txResult.ResultMetaXDR)
	if err != nil {
		return xdr.ScVal{}, sendResult.Hash, fmt.Errorf("stellar: %s succeeded but its return value could not be decoded: %w", function, err)
	}
	return returnVal, sendResult.Hash, nil
}

// --- shared helpers ---

func buildContractCallOperation(contractID, function string, args []xdr.ScVal, auth []xdr.SorobanAuthorizationEntry) (xdr.Operation, error) {
	contractAddr, err := scAddress(contractID)
	if err != nil {
		return xdr.Operation{}, err
	}
	invokeArgs := xdr.InvokeContractArgs{
		ContractAddress: contractAddr,
		FunctionName:    xdr.ScSymbol(function),
		Args:            args,
	}
	hf := xdr.HostFunction{Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract, InvokeContract: &invokeArgs}
	op := xdr.InvokeHostFunctionOp{HostFunction: hf, Auth: auth}
	return xdr.Operation{Body: xdr.OperationBody{Type: xdr.OperationTypeInvokeHostFunction, InvokeHostFunctionOp: &op}}, nil
}

func muxedAccount(address string) (xdr.MuxedAccount, error) {
	accountID, err := accountIDFromAddress(address)
	if err != nil {
		return xdr.MuxedAccount{}, err
	}
	pk := xdr.PublicKey(accountID)
	return xdr.MuxedAccount{Type: xdr.CryptoKeyTypeKeyTypeEd25519, Ed25519: pk.Ed25519}, nil
}

func accountIDFromAddress(address string) (xdr.AccountId, error) {
	version, raw, err := strkey.DecodeAny(address)
	if err != nil {
		return xdr.AccountId{}, fmt.Errorf("stellar: decode strkey address %q: %w", address, err)
	}
	if version != strkey.VersionByteAccountID || len(raw) != 32 {
		return xdr.AccountId{}, fmt.Errorf("stellar: %q is not a valid G... account address", address)
	}
	var raw32 xdr.Uint256
	copy(raw32[:], raw)
	return xdr.AccountId(xdr.PublicKey{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &raw32}), nil
}

func marshalUnsignedEnvelope(tx xdr.Transaction) (string, error) {
	v1 := xdr.TransactionV1Envelope{Tx: tx}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &v1}
	b, err := env.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("stellar: marshal envelope: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// fetchSequenceNumber returns account's current sequence number, read
// directly from its ledger entry.
func fetchSequenceNumber(ctx context.Context, client *Client, account string) (int64, error) {
	accountID, err := accountIDFromAddress(account)
	if err != nil {
		return 0, err
	}
	key := xdr.LedgerKey{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.LedgerKeyAccount{AccountId: accountID}}
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("stellar: marshal account ledger key: %w", err)
	}

	result, err := client.GetLedgerEntries(ctx, []string{base64.StdEncoding.EncodeToString(keyBytes)})
	if err != nil {
		return 0, fmt.Errorf("stellar: fetch account %s: %w", account, err)
	}
	if len(result.Entries) == 0 {
		return 0, fmt.Errorf("stellar: account %s not found (never funded?)", account)
	}

	entryBytes, err := base64.StdEncoding.DecodeString(result.Entries[0].XDR)
	if err != nil {
		return 0, fmt.Errorf("stellar: decode account ledger entry: %w", err)
	}
	var entryData xdr.LedgerEntryData
	if err := entryData.UnmarshalBinary(entryBytes); err != nil {
		return 0, fmt.Errorf("stellar: unmarshal account ledger entry: %w", err)
	}
	if entryData.Account == nil {
		return 0, fmt.Errorf("stellar: ledger entry for %s is not an account entry", account)
	}
	return int64(entryData.Account.SeqNum), nil
}

func decodeAuthEntries(entries []string) ([]xdr.SorobanAuthorizationEntry, error) {
	out := make([]xdr.SorobanAuthorizationEntry, 0, len(entries))
	for _, b64 := range entries {
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("stellar: decode auth entry: %w", err)
		}
		var entry xdr.SorobanAuthorizationEntry
		if err := entry.UnmarshalBinary(raw); err != nil {
			return nil, fmt.Errorf("stellar: unmarshal auth entry: %w", err)
		}
		out = append(out, entry)
	}
	return out, nil
}

// decodeReturnValue extracts a settled transaction's real return value
// from its base64 TransactionMeta. Soroban's meta version has changed
// across protocol upgrades (V3 vs V4 carry the Soroban return value in
// different fields) — both are handled since either can be live depending
// on the network's current protocol version.
func decodeReturnValue(resultMetaXDR string) (xdr.ScVal, error) {
	raw, err := base64.StdEncoding.DecodeString(resultMetaXDR)
	if err != nil {
		return xdr.ScVal{}, fmt.Errorf("stellar: decode result meta: %w", err)
	}
	var meta xdr.TransactionMeta
	if err := meta.UnmarshalBinary(raw); err != nil {
		return xdr.ScVal{}, fmt.Errorf("stellar: unmarshal result meta: %w", err)
	}
	switch {
	case meta.V3 != nil && meta.V3.SorobanMeta != nil:
		return meta.V3.SorobanMeta.ReturnValue, nil
	case meta.V4 != nil && meta.V4.SorobanMeta != nil && meta.V4.SorobanMeta.ReturnValue != nil:
		return *meta.V4.SorobanMeta.ReturnValue, nil
	default:
		return xdr.ScVal{}, fmt.Errorf("stellar: result meta (version %d) carries no Soroban return value", meta.V)
	}
}

func pollTransaction(ctx context.Context, client *Client, hash string) (*GetTransactionResult, error) {
	deadline := time.Now().Add(pollTimeout)
	for {
		result, err := client.GetTransaction(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("stellar: poll transaction %s: %w", hash, err)
		}
		if result.Status != GetTransactionStatusNotFound {
			return result, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("stellar: transaction %s did not settle within %s", hash, pollTimeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
