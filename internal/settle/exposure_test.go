package settle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCalculate_ChannelNotFound(t *testing.T) {
	pool := testSettlePool(t)
	calc := NewCalculator(pool)

	_, err := calc.Calculate(context.Background(), "CNONEXISTENT")
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("error = %v, want ErrChannelNotFound", err)
	}
}

func TestCalculate_NoCommitmentsAtAll(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	calc := NewCalculator(pool)

	exp, err := calc.Calculate(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("Calculate returned unexpected error: %v", err)
	}
	if exp.HighestVerified != nil {
		t.Errorf("HighestVerified = %v, want nil", exp.HighestVerified)
	}
	if exp.Unsettled.Sign() != 0 {
		t.Errorf("Unsettled = %s, want 0", exp.Unsettled)
	}
	if exp.OldestUnsettledSince != nil {
		t.Errorf("OldestUnsettledSince = %v, want nil", exp.OldestUnsettledSince)
	}
}

func TestCalculate_OneValidCommitmentNoSettlementYet(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	receivedAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	insertTestCommitment(t, pool, testChannel, 5000, true, receivedAt)
	calc := NewCalculator(pool)

	exp, err := calc.Calculate(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("Calculate returned unexpected error: %v", err)
	}
	if exp.HighestVerified.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("HighestVerified = %s, want 5000", exp.HighestVerified)
	}
	if exp.Unsettled.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("Unsettled = %s, want 5000", exp.Unsettled)
	}
	if exp.OldestUnsettledSince == nil {
		t.Fatal("OldestUnsettledSince is nil, want the commitment's received_at")
	}
	if !exp.OldestUnsettledSince.Equal(receivedAt) {
		t.Errorf("OldestUnsettledSince = %s, want %s", exp.OldestUnsettledSince, receivedAt)
	}
}

// TestCalculate_InvalidCommitmentsNeverCount is the correctness-critical
// case: an invalid (valid=false) commitment — even one with a higher
// cumulative_amount than any valid one, and even the most recent one —
// must never contribute to HighestVerified or OldestUnsettledSince. §6
// ordering rule 2: never settle against an unverified commitment.
func TestCalculate_InvalidCommitmentsNeverCount(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	now := time.Now().UTC()
	insertTestCommitment(t, pool, testChannel, 1000, true, now.Add(-3*time.Hour).Truncate(time.Microsecond))
	insertTestCommitment(t, pool, testChannel, 9000, false, now.Add(-1*time.Hour).Truncate(time.Microsecond)) // higher, newer, but invalid
	calc := NewCalculator(pool)

	exp, err := calc.Calculate(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("Calculate returned unexpected error: %v", err)
	}
	if exp.HighestVerified.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("HighestVerified = %s, want 1000 (the invalid 9000 commitment must not count)", exp.HighestVerified)
	}
	if exp.Unsettled.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("Unsettled = %s, want 1000", exp.Unsettled)
	}
}

func TestCalculate_FullySettledHasZeroExposure(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 5000, nil, nil)
	insertTestCommitment(t, pool, testChannel, 5000, true, time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond))
	calc := NewCalculator(pool)

	exp, err := calc.Calculate(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("Calculate returned unexpected error: %v", err)
	}
	if exp.Unsettled.Sign() != 0 {
		t.Errorf("Unsettled = %s, want 0 (fully settled)", exp.Unsettled)
	}
	if exp.OldestUnsettledSince != nil {
		t.Errorf("OldestUnsettledSince = %v, want nil (nothing left unsettled)", exp.OldestUnsettledSince)
	}
}

func TestCalculate_PartialSettlementPicksOldestUnsettled(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 3000, nil, nil)
	now := time.Now().UTC()
	// Settled up to 3000. 1000 and 2000 are already covered; 4000 and 6000
	// (cumulative) are still unsettled, 4000 being the older of the two.
	insertTestCommitment(t, pool, testChannel, 1000, true, now.Add(-5*time.Hour).Truncate(time.Microsecond))
	insertTestCommitment(t, pool, testChannel, 2000, true, now.Add(-4*time.Hour).Truncate(time.Microsecond))
	fourK := now.Add(-3 * time.Hour).Truncate(time.Microsecond)
	insertTestCommitment(t, pool, testChannel, 4000, true, fourK)
	insertTestCommitment(t, pool, testChannel, 6000, true, now.Add(-1*time.Hour).Truncate(time.Microsecond))
	calc := NewCalculator(pool)

	exp, err := calc.Calculate(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("Calculate returned unexpected error: %v", err)
	}
	if exp.HighestVerified.Cmp(big.NewInt(6000)) != 0 {
		t.Errorf("HighestVerified = %s, want 6000", exp.HighestVerified)
	}
	if exp.Unsettled.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("Unsettled = %s, want 3000 (6000 - 3000)", exp.Unsettled)
	}
	if exp.OldestUnsettledSince == nil || !exp.OldestUnsettledSince.Equal(fourK) {
		t.Errorf("OldestUnsettledSince = %v, want %s (the 4000 commitment, the first one > last_settled_amount)",
			exp.OldestUnsettledSince, fourK)
	}
}

func TestExposure_Age(t *testing.T) {
	now := time.Now()
	since := now.Add(-90 * time.Minute)
	exp := &Exposure{OldestUnsettledSince: &since}
	if got := exp.Age(now); got != 90*time.Minute {
		t.Errorf("Age = %s, want 90m", got)
	}

	empty := &Exposure{}
	if got := empty.Age(now); got != 0 {
		t.Errorf("Age with no OldestUnsettledSince = %s, want 0", got)
	}
}

func ExampleExposure_Age() {
	since := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)
	exp := &Exposure{OldestUnsettledSince: &since}
	fmt.Println(exp.Age(now))

	empty := &Exposure{}
	fmt.Println(empty.Age(now))
	// Output:
	// 2h30m0s
	// 0s
}

// ExampleNewCalculator shows both halves of NewCalculator's contract.
// NewCalculator itself never fails — it has no error return and stores pool
// verbatim (see its doc comment) — so the rejection half only exists at the
// first call that reads through the pool. A cancelled context is refused by
// the pool's own acquire step before any connection is opened, keeping this
// example runnable with or without Postgres; the returned error wraps
// context.Canceled, identified with errors.Is rather than by comparing its
// message. The sentinel specific to this file, ErrChannelNotFound, is
// Calculate's answer for a channel with no channels row — that path needs a
// real database to reach and is covered by TestCalculate_ChannelNotFound.
func ExampleNewCalculator() {
	// pgxpool.New opens no connection itself — connections are created
	// lazily on first use — so no database needs to be reachable here.
	pool, err := pgxpool.New(context.Background(), "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// Accepted input: a pool to read existing channels and commitments
	// through.
	calc := NewCalculator(pool)
	fmt.Println("accepted:", calc != nil)

	// Rejected input: a context that is already cancelled. Calculate's
	// first action is to read the channel row through pool, which refuses
	// the cancelled context before ever connecting.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = calc.Calculate(ctx, testChannel)
	fmt.Println("rejected:", errors.Is(err, context.Canceled))

	// Output:
	// accepted: true
	// rejected: true
}

// ExampleCalculator_Calculate shows how the settler reads one channel's
// exposure before deciding whether to sweep. Calculate only reads: the
// channels row comes from the indexer and the commitments from custody, so
// calling it never changes what gets settled. Only valid (verified)
// commitments count, which keeps a commitment that failed prepare_commitment
// out of the unsettled amount. A channel with no channels row comes back as
// ErrChannelNotFound, matched with errors.Is.
//
// The pool is a zero-value stub and testChannel is the package's own test
// channel. There is no // Output: line on purpose: Calculate's result
// depends on the channels and commitments rows in Postgres, which an
// example cannot set up without a live database. Without an Output line
// `go test` compiles the example but does not run it, so the pool is never
// dialed. It is here to pin the call shape: a signature change breaks the
// build. The behaviour is covered by the TestCalculate_* tests above.
func ExampleCalculator_Calculate() {
	calc := NewCalculator(&pgxpool.Pool{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exp, err := calc.Calculate(ctx, testChannel)
	if errors.Is(err, ErrChannelNotFound) {
		fmt.Println("channel not indexed yet:", testChannel)
		return
	}
	if err != nil {
		fmt.Println("calculate exposure:", err)
		return
	}
	fmt.Println("unsettled:", exp.Unsettled, "age:", exp.Age(time.Now()))
}
