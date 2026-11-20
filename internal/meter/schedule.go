// Package meter turns metered requests into charges: loading and validating
// an operator's published price schedule, and pricing individual requests
// against it. It has no dependency on internal/stellar or internal/store —
// it consumes plain Go values (a schedule document's bytes, a ledger
// number, a method and path template) and produces plain Go values (a
// charged amount, a price version), so it can be tested without a network
// or a database and reused by both the collector and, later, anything that
// needs to re-derive a historical charge.
package meter

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Sentinel errors for ParseSchedule. Wrapped with %w alongside the specific
// rule and reason, so a caller can match with errors.Is while a human still
// gets a useful message.
var (
	ErrEmptySchedule       = errors.New("meter: schedule has no rules")
	ErrDuplicateRule       = errors.New("meter: schedule has two rules for the same method and path_template")
	ErrInvalidMethod       = errors.New("meter: rule method is empty")
	ErrInvalidPathTemplate = errors.New("meter: rule path_template is empty or does not start with /")
	ErrInvalidUnitPrice    = errors.New("meter: rule unit_price is not a valid non-negative integer")
	ErrUnitPriceOutOfRange = errors.New("meter: rule unit_price exceeds the i128 range statement_registry uses for amounts")
)

// i128Max bounds a rule's unit price: a charged amount is unit_price times a
// request's unit count, and that amount ultimately has to fit in the i128
// range statement_registry's amount_billed field uses (§4) — better to
// reject an oversized price at schedule-load time than to have it silently
// overflow the first time a request actually prices against it.
var i128Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))

// Rule is one priced endpoint: unit_price stroops per unit of Method+PathTemplate.
type Rule struct {
	Method       string
	PathTemplate string
	UnitPrice    *big.Int
}

// Schedule is one validated, published price schedule — the document an
// operator's price_book.publish URI points to, parsed and checked. Method
// and PathTemplate together identify an endpoint the same way
// merkle.EndpointHash does (sha256(method || " " || path_template)); a
// Schedule rule and the merkle leaf a billed request eventually produces
// must agree on that identity, which is why both key on the exact same
// tuple rather than, say, a path pattern merged with the method separately.
type Schedule struct {
	rules []Rule
	byKey map[string]Rule
}

// scheduleDocument is the on-the-wire JSON shape ParseSchedule reads.
// unit_price is a string, not a JSON number: money is integer everywhere in
// this codebase (§8), and a JSON number big enough to matter here would
// already have lost precision by the time encoding/json parsed it.
type scheduleDocument struct {
	Rules []scheduleRule `json:"rules"`
}

type scheduleRule struct {
	Method       string `json:"method"`
	PathTemplate string `json:"path_template"`
	UnitPrice    string `json:"unit_price"`
}

// ParseSchedule parses and validates raw as a price schedule document.
// Validation rejects: no rules; a rule with an empty method; a rule whose
// path_template is empty or doesn't start with "/"; two rules for the same
// (method, path_template) pair, case-insensitively on method; a unit_price
// that isn't a non-negative base-10 integer, or that exceeds the i128
// range. Method is normalized to uppercase (an operator's document can
// write "get" or "GET" and mean the same rule); path_template is taken
// verbatim, since Soroban's endpoint hash and this package's own Lookup
// both compare it byte for byte.
func ParseSchedule(raw []byte) (*Schedule, error) {
	var doc scheduleDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("meter: parse schedule JSON with no usable rules: %w: %w", ErrEmptySchedule, err)
	}
	if len(doc.Rules) == 0 {
		return nil, ErrEmptySchedule
	}

	rules := make([]Rule, 0, len(doc.Rules))
	byKey := make(map[string]Rule, len(doc.Rules))
	for i, rr := range doc.Rules {
		method := strings.ToUpper(strings.TrimSpace(rr.Method))
		if method == "" {
			return nil, fmt.Errorf("meter: rule %d: %w", i, ErrInvalidMethod)
		}

		path := strings.TrimSpace(rr.PathTemplate)
		if path == "" || !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("meter: rule %d (%s): %w: %q", i, method, ErrInvalidPathTemplate, rr.PathTemplate)
		}

		key := scheduleKey(method, path)
		if _, exists := byKey[key]; exists {
			return nil, fmt.Errorf("meter: rule %d: %w: %s %s", i, ErrDuplicateRule, method, path)
		}

		priceRaw := strings.TrimSpace(rr.UnitPrice)
		price, ok := new(big.Int).SetString(priceRaw, 10)
		if !ok || price.Sign() < 0 {
			return nil, fmt.Errorf("meter: rule %d (%s %s): %w: %q", i, method, path, ErrInvalidUnitPrice, rr.UnitPrice)
		}
		if price.Cmp(i128Max) > 0 {
			return nil, fmt.Errorf("meter: rule %d (%s %s): %w: %s", i, method, path, ErrUnitPriceOutOfRange, price)
		}

		rule := Rule{Method: method, PathTemplate: path, UnitPrice: price}
		rules = append(rules, rule)
		byKey[key] = rule
	}

	return &Schedule{rules: rules, byKey: byKey}, nil
}

// Lookup returns the unit price for method and pathTemplate, and whether a
// rule for that pair exists at all. method is compared case-insensitively;
// pathTemplate is compared exactly, matching how it was stored.
func (s *Schedule) Lookup(method, pathTemplate string) (*big.Int, bool) {
	rule, ok := s.byKey[scheduleKey(strings.ToUpper(method), pathTemplate)]
	if !ok {
		return nil, false
	}
	return rule.UnitPrice, true
}

// Rules returns every rule in the schedule, in document order. The result
// is a copy; mutating it does not affect the Schedule.
func (s *Schedule) Rules() []Rule {
	return append([]Rule(nil), s.rules...)
}

func scheduleKey(method, pathTemplate string) string {
	return method + " " + pathTemplate
}

// ScheduleHash returns sha256(raw) — the value that must equal the
// schedule_hash argument price_book.publish was called with (§4). It
// hashes the document's raw bytes as published, not a reserialization of
// the parsed Schedule, so there is no risk of this package's JSON decoder
// and whatever produced the original file disagreeing on canonical form.
func ScheduleHash(raw []byte) [32]byte {
	return sha256.Sum256(raw)
}
