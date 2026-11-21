package stellar

import (
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestAuthorizeInvocation_MalformedContractID asserts that a malformed contract
// ID results in an error rather than a panic, satisfying CLAUDE.md §8 ("No
// panics outside main startup validation. A library function returns an error").
func TestAuthorizeInvocation_MalformedContractID(t *testing.T) {
	kp, err := keypair.Random()
	if err != nil {
		t.Fatalf("keypair.Random: %v", err)
	}

	tests := []struct {
		name       string
		contractID string
	}{
		{"empty string", ""},
		{"not base32", "NOTACONTRACTID!!!"},
		{"bad checksum", "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB"},
		{"too short", "CAAA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AuthorizeInvocation("Test SDF Network ; September 2015", kp, tt.contractID, "fn", []xdr.ScVal{}, 100)
			if err == nil {
				t.Fatalf("AuthorizeInvocation(%q) returned nil error, want error for malformed contract ID", tt.contractID)
			}
			if !strings.Contains(err.Error(), "stellar:") {
				t.Errorf("error %q does not carry expected stellar: prefix", err.Error())
			}
		})
	}
}
