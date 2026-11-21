package settle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

type fakeLedgerSource struct {
	ledger uint32
	err    error
}

func (f fakeLedgerSource) CurrentLedger(context.Context) (uint32, error) {
	return f.ledger, f.err
}

func fixedFactory(live LiveChannel) ChannelFactory {
	return func(string) LiveChannel { return live }
}

// fakeLiveChannel implements LiveChannel for daemon tests: Settle
// succeeds immediately (no retry needed within the tick), Withdrawn is
// unused unless a test specifically exercises reconciliation.
type fakeLiveChannel struct {
	*fakeChannelSettler
	withdrawn *fakeWithdrawnReader
}

func (f *fakeLiveChannel) Withdrawn(ctx context.Context) (*big.Int, error) {
	return f.withdrawn.Withdrawn(ctx)
}

func newFakeLiveChannel(hash string, err error) *fakeLiveChannel {
	return &fakeLiveChannel{
		fakeChannelSettler: &fakeChannelSettler{hash: hash, err: err},
		withdrawn:          &fakeWithdrawnReader{withdrawn: big.NewInt(0)},
	}
}

func TestDaemon_Tick_SettlesChannelsOverThreshold(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, testChannel, 20_000_000) // over MaxExposure

	live := newFakeLiveChannel("hash1", nil)
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(live), nil,
		fakeLedgerSource{ledger: 100}, defaultPolicy())

	attempted, underPressure, err := daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if attempted != 1 {
		t.Errorf("attempted = %d, want 1", attempted)
	}
	if underPressure {
		t.Error("underPressure = true, want false (no deadline set)")
	}
	if len(live.fakeChannelSettler.calls) != 1 {
		t.Fatalf("Settle called %d times, want 1", len(live.fakeChannelSettler.calls))
	}
	if live.fakeChannelSettler.calls[0].amount.Cmp(big.NewInt(20_000_000)) != 0 {
		t.Errorf("Settle called with amount %s, want 20000000", live.fakeChannelSettler.calls[0].amount)
	}

	status, exists := settlementStatus(t, pool, testChannel, 20_000_000)
	if !exists || status != "confirmed" {
		t.Errorf("settlements row status = %q (exists=%v), want confirmed", status, exists)
	}
}

func TestDaemon_Tick_DoesNotSettleChannelsUnderThreshold(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertTestCommitment(t, pool, testChannel, 100, true, time.Now().UTC().Truncate(time.Microsecond)) // well under every threshold

	live := newFakeLiveChannel("should-not-be-used", nil)
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(live), nil,
		fakeLedgerSource{ledger: 100}, defaultPolicy())

	attempted, _, err := daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if attempted != 0 {
		t.Errorf("attempted = %d, want 0", attempted)
	}
	if len(live.fakeChannelSettler.calls) != 0 {
		t.Error("Settle was called for a channel under every threshold")
	}
}

// TestDaemon_Tick_IgnoresClosedAndRefundedChannels confirms the watch
// scope itself — status IN ('open', 'closing') per §6 — excludes
// terminal-state channels regardless of how much exposure they show.
func TestDaemon_Tick_IgnoresClosedAndRefundedChannels(t *testing.T) {
	pool := testSettlePool(t)
	for i, status := range []string{"closed", "refunded"} {
		addr := testChannel[:len(testChannel)-1] + string(rune('A'+i))
		insertTestChannel(t, pool, addr, status, 0, nil, nil)
		insertTestCommitment(t, pool, addr, 50_000_000, true, time.Now().UTC().Truncate(time.Microsecond))
	}

	live := newFakeLiveChannel("should-not-be-used", nil)
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(live), nil,
		fakeLedgerSource{ledger: 100}, defaultPolicy())

	attempted, _, err := daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if attempted != 0 {
		t.Errorf("attempted = %d, want 0 (closed/refunded channels must never be watched)", attempted)
	}
	if len(live.fakeChannelSettler.calls) != 0 {
		t.Error("Settle was called for a closed or refunded channel")
	}
}

// TestDaemon_Tick_OneChannelFailureDoesNotStopOthers is the resilience
// case: two channels both need settling; the first's chain call fails
// permanently (a real contract-level rejection, not a transient error —
// otherwise Daemon.Tick's real, un-faked retry/backoff would burn through
// its whole tickRetryBudget for real wall-clock time before this test
// could even get to the second channel), the second must still be
// attempted and succeed.
func TestDaemon_Tick_OneChannelFailureDoesNotStopOthers(t *testing.T) {
	pool := testSettlePool(t)
	channelA := testChannel
	channelB := testChannel[:len(testChannel)-1] + "Z"
	insertTestChannel(t, pool, channelA, "open", 0, nil, nil)
	insertTestChannel(t, pool, channelB, "open", 0, nil, nil)
	insertValidCommitmentForSettle(t, pool, channelA, 20_000_000)
	insertValidCommitmentForSettle(t, pool, channelB, 20_000_000)

	failing := newFakeLiveChannel("", &stellar.ErrTransactionFailed{Hash: "deadbeef"})
	succeeding := newFakeLiveChannel("hash-b", nil)

	factory := func(address string) LiveChannel {
		if address == channelA {
			return failing
		}
		return succeeding
	}
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), factory, nil,
		fakeLedgerSource{ledger: 100}, defaultPolicy())

	attempted, _, err := daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if attempted != 2 {
		t.Errorf("attempted = %d, want 2 (both channels needed settling)", attempted)
	}

	statusA, existsA := settlementStatus(t, pool, channelA, 20_000_000)
	if !existsA || statusA != "failed" {
		t.Errorf("channel A settlement status = %q (exists=%v), want failed", statusA, existsA)
	}
	statusB, existsB := settlementStatus(t, pool, channelB, 20_000_000)
	if !existsB || statusB != "confirmed" {
		t.Errorf("channel B settlement status = %q (exists=%v), want confirmed despite channel A failing", statusB, existsB)
	}
}

// TestDaemon_Tick_ReportsUnderDeadlinePressure confirms Run's tightening
// signal is actually derived from a real deadline-pressure decision, not
// just from whether anything got attempted.
func TestDaemon_Tick_ReportsUnderDeadlinePressure(t *testing.T) {
	pool := testSettlePool(t)
	deadline := int32(101440) // currentLedger(100000) + margin(1440) == deadline
	insertTestChannel(t, pool, testChannel, "closing", 0, ptrInt32(101440-1440), &deadline)
	insertValidCommitmentForSettle(t, pool, testChannel, 100)

	live := newFakeLiveChannel("hash1", nil)
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(live), nil,
		fakeLedgerSource{ledger: 100000}, defaultPolicy())

	_, underPressure, err := daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}
	if !underPressure {
		t.Error("underPressure = false, want true")
	}
}

func TestDaemon_Tick_LedgerSourceErrorPropagates(t *testing.T) {
	pool := testSettlePool(t)
	boom := errors.New("rpc down")
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(newFakeLiveChannel("", nil)), nil,
		fakeLedgerSource{err: boom}, defaultPolicy())

	_, _, err := daemon.Tick(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
}

func ptrInt32(v int32) *int32 { return &v }

func ExampleDaemon_LastSuccessfulChainRead() {
	d := NewDaemon(nil, nil, nil, nil, nil, nil, Policy{})

	// Before any successful Tick the chain has never been read.
	_, ok := d.LastSuccessfulChainRead()
	fmt.Println(ok)

	// Once a Tick records a successful ledger read, it returns that time and true.
	t0 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	d.chainRead.recordSuccess(t0)
	got, ok := d.LastSuccessfulChainRead()
	fmt.Println(ok)
	fmt.Println(got.UTC().Format(time.RFC3339))
	// Output:
	// false
	// true
	// 2024-01-15T10:00:00Z
}
