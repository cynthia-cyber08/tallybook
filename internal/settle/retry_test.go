package settle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"

	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

// settleFunc adapts a plain function to ChannelSettler, for tests that
// need a sequence of different outcomes across calls rather than the one
// fixed (hash, err) pair fakeChannelSettler supports.
type settleFunc func(ctx context.Context, amount *big.Int, sig [64]byte) (string, error)

func (f settleFunc) Settle(ctx context.Context, _ *keypair.Full, amount *big.Int, sig [64]byte) (string, error) {
	return f(ctx, amount, sig)
}

func identityJitter(d time.Duration) time.Duration { return d }

// fakeClock and fakeSleeper make retry timing deterministic and instant:
// Sleep never actually waits, it just records the duration it was asked
// to wait and advances the fake clock by exactly that much, so
// opts.Now() inside SubmitWithRetry sees time having "passed" without the
// test taking any real wall-clock time.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

type fakeSleeper struct {
	clock *fakeClock
	calls []time.Duration
}

func (s *fakeSleeper) Sleep(_ context.Context, d time.Duration) error {
	s.calls = append(s.calls, d)
	s.clock.Advance(d)
	return nil
}

func ExampleIsPermanent() {
	// ErrTransactionFailed means the chain itself rejected the settlement;
	// retrying the same call cannot change that outcome.
	fmt.Println(IsPermanent(&stellar.ErrTransactionFailed{Hash: "abc"}))
	// A network error is transient; SubmitWithRetry will back off and retry.
	fmt.Println(IsPermanent(errors.New("connection reset by peer")))
	// Output:
	// true
	// false
}

// ExampleDefaultRetryOptions shows the concrete backoff parameters that
// DefaultRetryOptions sets for a given deadline. All fields except Now,
// Sleep, and Jitter (which default to nil, resolved at call time) are
// deterministic given the deadline.
func ExampleDefaultRetryOptions() {
	deadline := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := DefaultRetryOptions(deadline)
	fmt.Println(opts.InitialDelay)
	fmt.Println(opts.MaxDelay)
	fmt.Println(opts.Multiplier)
	fmt.Println(opts.TightenAfter)
	fmt.Println(opts.TightenedDelay)
	fmt.Println(opts.Deadline.Equal(deadline))
	// Output:
	// 2s
	// 2m0s
	// 2
	// 30m0s
	// 15s
	// true
}

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"simulation failed", &stellar.ErrSimulationFailed{Message: "boom"}, true},
		{"transaction failed", &stellar.ErrTransactionFailed{Hash: "abc"}, true},
		{"wrapped simulation failed", fmt.Errorf("settle: submit: %w", &stellar.ErrSimulationFailed{Message: "boom"}), true},
		{"commitment not found", ErrCommitmentNotFound, true},
		{"invalid signature length", ErrInvalidSignatureLength, true},
		{"settlement in flight", ErrSettlementInFlight, true},
		{"plain network error", errors.New("connection reset by peer"), false},
		{"wrapped plain error", fmt.Errorf("stellar: send: %w", errors.New("timeout")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.want {
				t.Errorf("IsPermanent(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestSubmitWithRetry_SucceedsFirstAttempt(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	fake := &fakeChannelSettler{hash: "hash1"}
	clock := &fakeClock{now: time.Now()}
	sleeper := &fakeSleeper{clock: clock}
	opts := DefaultRetryOptions(clock.now.Add(time.Hour))
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, fake, nil, testChannel, big.NewInt(5000), opts)
	if err != nil {
		t.Fatalf("SubmitWithRetry returned unexpected error: %v", err)
	}
	if rec.Status != "confirmed" {
		t.Errorf("Status = %q, want confirmed", rec.Status)
	}
	if len(fake.calls) != 1 {
		t.Errorf("Settle called %d times, want 1", len(fake.calls))
	}
	if len(sleeper.calls) != 0 {
		t.Errorf("Sleep called %d times, want 0 (no retry needed)", len(sleeper.calls))
	}
}

func TestSubmitWithRetry_PermanentErrorStopsImmediately(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	fake := &fakeChannelSettler{hash: "h", err: &stellar.ErrTransactionFailed{Hash: "h"}}
	clock := &fakeClock{now: time.Now()}
	sleeper := &fakeSleeper{clock: clock}
	opts := DefaultRetryOptions(clock.now.Add(time.Hour))
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, fake, nil, testChannel, big.NewInt(5000), opts)
	var txErr *stellar.ErrTransactionFailed
	if !errors.As(err, &txErr) {
		t.Fatalf("error = %v, want it to wrap *stellar.ErrTransactionFailed", err)
	}
	if rec.Status != "failed" {
		t.Errorf("Status = %q, want failed", rec.Status)
	}
	if len(fake.calls) != 1 {
		t.Errorf("Settle called %d times, want 1 (a permanent error must not be retried)", len(fake.calls))
	}
	if len(sleeper.calls) != 0 {
		t.Errorf("Sleep called %d times, want 0", len(sleeper.calls))
	}

	status, exists := settlementStatus(t, pool, testChannel, 5000)
	if !exists || status != "failed" {
		t.Errorf("settlements row status = %q (exists=%v), want failed", status, exists)
	}
}

func TestSubmitWithRetry_TransientErrorsRetryWithGrowingBackoffThenSucceed(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	attempts := 0
	seq := []error{errors.New("rpc: timeout"), errors.New("rpc: connection reset"), nil}
	settler := settleFunc(func(_ context.Context, amount *big.Int, sig [64]byte) (string, error) {
		defer func() { attempts++ }()
		err := seq[attempts]
		if err != nil {
			return "", err
		}
		return "final-hash", nil
	})

	clock := &fakeClock{now: time.Now()}
	sleeper := &fakeSleeper{clock: clock}
	opts := DefaultRetryOptions(clock.now.Add(time.Hour))
	opts.InitialDelay = 2 * time.Second
	opts.Multiplier = 2
	opts.MaxDelay = time.Minute
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, settler, nil, testChannel, big.NewInt(5000), opts)
	if err != nil {
		t.Fatalf("SubmitWithRetry returned unexpected error: %v", err)
	}
	if rec.Status != "confirmed" || rec.TxHash != "final-hash" {
		t.Errorf("rec = %+v, want confirmed with tx hash final-hash", rec)
	}
	if attempts != 3 {
		t.Errorf("Settle called %d times, want 3", attempts)
	}
	if len(sleeper.calls) != 2 {
		t.Fatalf("Sleep called %d times, want 2", len(sleeper.calls))
	}
	if sleeper.calls[0] != 2*time.Second {
		t.Errorf("first sleep = %s, want 2s (InitialDelay)", sleeper.calls[0])
	}
	if sleeper.calls[1] != 4*time.Second {
		t.Errorf("second sleep = %s, want 4s (InitialDelay * Multiplier)", sleeper.calls[1])
	}
}

func TestSubmitWithRetry_TightensIntervalNearDeadline(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	start := time.Now()
	clock := &fakeClock{now: start}
	sleeper := &fakeSleeper{clock: clock}

	// Deadline is 50s out. TightenAfter is 40s (i.e. tighten once inside
	// the final 40s). InitialDelay 10s, Multiplier 2: first sleep 10s
	// (clock -> +10s, 40s from deadline: right at the tighten boundary,
	// not yet inside it), second sleep would normally be 20s but by then
	// we're within TightenAfter, so it must be TightenedDelay instead.
	deadline := start.Add(50 * time.Second)
	opts := RetryOptions{
		InitialDelay: 10 * time.Second, Multiplier: 2, MaxDelay: time.Minute,
		Deadline: deadline, TightenAfter: 45 * time.Second, TightenedDelay: 3 * time.Second,
		Now: clock.Now, Sleep: sleeper.Sleep, Jitter: identityJitter,
	}

	attempts := 0
	seq := []error{errors.New("blip"), errors.New("blip"), nil}
	settler := settleFunc(func(_ context.Context, amount *big.Int, sig [64]byte) (string, error) {
		defer func() { attempts++ }()
		if seq[attempts] != nil {
			return "", seq[attempts]
		}
		return "hash", nil
	})

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, settler, nil, testChannel, big.NewInt(5000), opts)
	if err != nil {
		t.Fatalf("SubmitWithRetry returned unexpected error: %v", err)
	}
	if rec.Status != "confirmed" {
		t.Errorf("Status = %q, want confirmed", rec.Status)
	}
	if len(sleeper.calls) != 2 {
		t.Fatalf("Sleep called %d times, want 2: %v", len(sleeper.calls), sleeper.calls)
	}
	if sleeper.calls[1] != opts.TightenedDelay {
		t.Errorf("second sleep = %s, want TightenedDelay %s once within TightenAfter of the deadline",
			sleeper.calls[1], opts.TightenedDelay)
	}
}

func TestSubmitWithRetry_NeverGivesUpBeforeDeadline(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	start := time.Now()
	clock := &fakeClock{now: start}
	sleeper := &fakeSleeper{clock: clock}
	// Five transient failures in a row, succeeding on the sixth — even
	// though the accumulated (fake) elapsed time is well past what a
	// single tick might normally allow, the deadline (2 hours out) is
	// never reached, so every one of the five failures must be retried.
	opts := RetryOptions{
		InitialDelay: 5 * time.Second, Multiplier: 2, MaxDelay: 30 * time.Second,
		Deadline: start.Add(2 * time.Hour), TightenAfter: time.Minute, TightenedDelay: time.Second,
		Now: clock.Now, Sleep: sleeper.Sleep, Jitter: identityJitter,
	}

	attempts := 0
	seq := []error{
		errors.New("1"), errors.New("2"), errors.New("3"), errors.New("4"), errors.New("5"), nil,
	}
	settler := settleFunc(func(_ context.Context, amount *big.Int, sig [64]byte) (string, error) {
		defer func() { attempts++ }()
		if seq[attempts] != nil {
			return "", seq[attempts]
		}
		return "hash", nil
	})

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, settler, nil, testChannel, big.NewInt(5000), opts)
	if err != nil {
		t.Fatalf("SubmitWithRetry returned unexpected error: %v", err)
	}
	if rec.Status != "confirmed" {
		t.Errorf("Status = %q, want confirmed", rec.Status)
	}
	if attempts != 6 {
		t.Errorf("Settle called %d times, want 6 (five failures, all retried, then success)", attempts)
	}
}

// TestSubmitWithRetry_StopsAtDeadlineLeavesRowSubmitting is the crash-safe
// half of deadline escalation: when the deadline passes with the
// settlement still failing, SubmitWithRetry must stop — but it must NOT
// guess at an outcome. The row stays 'submitting', for ReconcileSubmitting
// to resolve later against the chain's real state.
func TestSubmitWithRetry_StopsAtDeadlineLeavesRowSubmitting(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)

	start := time.Now()
	clock := &fakeClock{now: start}
	sleeper := &fakeSleeper{clock: clock}
	// Deadline is already now — the very first failure must find the
	// deadline already passed.
	opts := DefaultRetryOptions(start)
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	fake := &fakeChannelSettler{err: errors.New("still down")}
	s := NewSubmitter(pool)

	rec, err := SubmitWithRetry(context.Background(), s, fake, nil, testChannel, big.NewInt(5000), opts)
	if err == nil {
		t.Fatal("SubmitWithRetry returned nil error, want a deadline-passed error")
	}
	if rec == nil || rec.Status != "submitting" {
		t.Errorf("rec = %+v, want status still 'submitting'", rec)
	}
	if len(sleeper.calls) != 0 {
		t.Errorf("Sleep called %d times, want 0 (deadline was already passed on the first failure)", len(sleeper.calls))
	}

	status, exists := settlementStatus(t, pool, testChannel, 5000)
	if !exists || status != "submitting" {
		t.Errorf("settlements row status = %q (exists=%v), want submitting — must not be guessed as failed", status, exists)
	}
}

func TestSubmitWithRetry_ExistingConfirmedRowIsIdempotent(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)
	fake := &fakeChannelSettler{hash: "h1"}
	s := NewSubmitter(pool)
	if _, err := s.Submit(context.Background(), fake, nil, testChannel, big.NewInt(5000)); err != nil {
		t.Fatalf("initial Submit returned unexpected error: %v", err)
	}

	clock := &fakeClock{now: time.Now()}
	sleeper := &fakeSleeper{clock: clock}
	opts := DefaultRetryOptions(clock.now.Add(time.Hour))
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	rec, err := SubmitWithRetry(context.Background(), s, fake, nil, testChannel, big.NewInt(5000), opts)
	if err != nil {
		t.Fatalf("SubmitWithRetry returned unexpected error: %v", err)
	}
	if rec.Status != "confirmed" {
		t.Errorf("Status = %q, want confirmed", rec.Status)
	}
	if len(fake.calls) != 1 {
		t.Errorf("Settle called %d times total, want 1 (SubmitWithRetry must not call it again for an already-confirmed row)", len(fake.calls))
	}
}

func TestSubmitWithRetry_ExistingSubmittingRowRefusesWithoutCallingSettle(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 5000)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO settlements (channel, cumulative_amount, status) VALUES ($1, $2, 'submitting')`,
		testChannel, int64(5000),
	); err != nil {
		t.Fatalf("insert pre-existing submitting row: %v", err)
	}

	fake := &fakeChannelSettler{hash: "should-not-be-used"}
	clock := &fakeClock{now: time.Now()}
	sleeper := &fakeSleeper{clock: clock}
	opts := DefaultRetryOptions(clock.now.Add(time.Hour))
	opts.Now, opts.Sleep, opts.Jitter = clock.Now, sleeper.Sleep, identityJitter

	s := NewSubmitter(pool)
	rec, err := SubmitWithRetry(context.Background(), s, fake, nil, testChannel, big.NewInt(5000), opts)
	if !errors.Is(err, ErrSettlementInFlight) {
		t.Fatalf("error = %v, want ErrSettlementInFlight", err)
	}
	if rec == nil || rec.Status != "submitting" {
		t.Errorf("rec = %+v, want the existing submitting record", rec)
	}
	if len(fake.calls) != 0 {
		t.Error("Settle was called despite an unresolved submitting row already existing")
	}
}
