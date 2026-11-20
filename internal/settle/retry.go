package settle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"math/rand"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"

	"github.com/Tallybook-Org/tallybook/internal/stellar"
)

// IsPermanent reports whether err from Submit is worth retrying at all.
// §6's failure handling draws exactly this line: "A settle that fails on
// chain for a contract reason: log the decoded error, mark the attempt
// failed, do not retry blindly" — as opposed to a transient RPC/network
// problem, which is retried. *stellar.ErrSimulationFailed and
// *stellar.ErrTransactionFailed both mean the call genuinely reached the
// chain and was rejected there; nothing about retrying changes that
// outcome. Submit's own logic-level sentinels (a missing commitment, a
// malformed signature, an already-in-flight duplicate) are permanent for
// the same reason: retrying without anything else changing cannot resolve
// them either.
func IsPermanent(err error) bool {
	var simErr *stellar.ErrSimulationFailed
	var txErr *stellar.ErrTransactionFailed
	if errors.As(err, &simErr) || errors.As(err, &txErr) {
		return true
	}
	if errors.Is(err, ErrCommitmentNotFound) || errors.Is(err, ErrInvalidSignatureLength) || errors.Is(err, ErrSettlementInFlight) {
		return true
	}
	return false
}

// RetryOptions bounds SubmitWithRetry's backoff. Now, Sleep, and Jitter
// are overridable so retry timing is testable without a real clock or
// real wall-clock waits (step 48); each defaults to the real thing when
// left nil.
type RetryOptions struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64

	// Deadline is when the caller must stop retrying and let the failure
	// escalate instead — typically the channel's refund deadline (as a
	// wall-clock estimate), or the zero value for "no deadline, retry
	// indefinitely." §6: "Never give up before the deadline passes."
	Deadline time.Time
	// TightenAfter is how close to Deadline triggers the tightened retry
	// interval (§6: "keep retrying at a tightened interval").
	TightenAfter   time.Duration
	TightenedDelay time.Duration

	Now   func() time.Time
	Sleep func(context.Context, time.Duration) error
	// Jitter transforms a computed delay before it's slept — defaults to
	// a real +/-20% random jitter. Tests override it to the identity
	// function for deterministic assertions.
	Jitter func(time.Duration) time.Duration
}

// DefaultRetryOptions returns reasonable backoff parameters for a
// settlement that must land before deadline.
func DefaultRetryOptions(deadline time.Time) RetryOptions {
	return RetryOptions{
		InitialDelay:   2 * time.Second,
		MaxDelay:       2 * time.Minute,
		Multiplier:     2,
		Deadline:       deadline,
		TightenAfter:   30 * time.Minute,
		TightenedDelay: 15 * time.Second,
	}
}

func defaultJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	factor := 0.8 + 0.4*rand.Float64() // +/- 20%
	return time.Duration(float64(d) * factor)
}

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (o RetryOptions) resolved() RetryOptions {
	if o.InitialDelay <= 0 {
		o.InitialDelay = time.Second
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = o.InitialDelay
	}
	if o.Multiplier <= 1 {
		o.Multiplier = 2
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Sleep == nil {
		o.Sleep = realSleep
	}
	if o.Jitter == nil {
		o.Jitter = defaultJitter
	}
	return o
}

// SubmitWithRetry is Submit's retrying counterpart: it manages one
// settlements row across possibly many attempts, rather than settling it
// after just one, so a transient failure never gets locked in as
// terminal. Intent is recorded (or an existing row found) exactly once,
// up front, via the same recordOrFindIntent Submit itself uses; if a row
// already existed, SubmitWithRetry defers to it exactly like Submit does
// (idempotent on 'confirmed'/'failed', ErrSettlementInFlight on
// 'submitting' — retrying does not mean re-entering a row someone else,
// or an earlier crashed process, is already working on).
//
// For a fresh attempt, channel.Settle is retried with exponential backoff
// and jitter on a transient failure (§6: "exponential backoff with
// jitter, capped") until it succeeds, a permanent error occurs
// (IsPermanent — logged at ERROR, the row marked 'failed', returned
// immediately), ctx is cancelled, or opts.Deadline passes. Once within
// opts.TightenAfter of the deadline, it switches to opts.TightenedDelay
// instead of the normal backoff and logs at ERROR rather than WARN (§6:
// "keep retrying at a tightened interval"). It never gives up before
// opts.Deadline passes.
//
// A transient failure never marks the row 'failed' — including when
// retries stop because the deadline passed or ctx was cancelled. Whether
// the last attempt actually reached the chain is genuinely unknown at
// that point, and guessing would risk exactly the ambiguity ordering
// rule 3 exists to avoid; the row is deliberately left 'submitting' for
// ReconcileSubmitting to resolve against the chain's real state,
// typically on the next restart or daemon tick.
func SubmitWithRetry(ctx context.Context, s *Submitter, channel ChannelSettler, signer *keypair.Full, channelAddress string, amount *big.Int, opts RetryOptions) (*Record, error) {
	opts = opts.resolved()

	rec, sig, resolved, err := s.recordOrFindIntent(ctx, channelAddress, amount)
	if err != nil {
		return nil, err
	}
	if resolved {
		if SettlementStatus(rec.Status) == SettlementStatusSubmitting {
			return rec, fmt.Errorf("settle: submit with retry %s %s: %w", channelAddress, amount, ErrSettlementInFlight)
		}
		return rec, nil
	}

	delay := opts.InitialDelay
	attempt := 0

	for {
		attempt++
		hash, sendErr := channel.Settle(ctx, signer, amount, sig)

		if sendErr == nil {
			if err := s.markConfirmed(ctx, rec.ID, hash); err != nil {
				return nil, fmt.Errorf("settle: submit with retry %s %s: mark confirmed: %w", channelAddress, amount, err)
			}
			rec.Status = "confirmed"
			rec.TxHash = hash
			return rec, nil
		}

		if IsPermanent(sendErr) {
			if markErr := s.markFailed(ctx, rec.ID, hash, sendErr.Error()); markErr != nil {
				return nil, fmt.Errorf("settle: submit with retry %s %s: send failed (%w) and marking it failed also failed: %w",
					channelAddress, amount, sendErr, markErr)
			}
			rec.Status = "failed"
			rec.TxHash = hash
			rec.FailureReason = sendErr.Error()
			slog.ErrorContext(ctx, "settle: settlement failed for a permanent reason, not retrying",
				"channel", channelAddress, "amount", amount, "attempt", attempt, "error", sendErr)
			return rec, fmt.Errorf("settle: submit with retry %s %s: %w", channelAddress, amount, sendErr)
		}

		now := opts.Now()
		if !opts.Deadline.IsZero() && !now.Before(opts.Deadline) {
			slog.ErrorContext(ctx, "settle: retry deadline passed with the settlement still failing; leaving it submitting for reconciliation",
				"channel", channelAddress, "amount", amount, "attempt", attempt, "error", sendErr)
			return rec, fmt.Errorf("settle: retry: deadline passed: %w", sendErr)
		}

		tightening := !opts.Deadline.IsZero() && opts.Deadline.Sub(now) <= opts.TightenAfter
		wait := delay
		if tightening {
			wait = opts.TightenedDelay
			slog.ErrorContext(ctx, "settle: transient failure with the deadline approaching, tightening retry interval",
				"channel", channelAddress, "amount", amount, "attempt", attempt, "wait", wait, "error", sendErr)
		} else {
			slog.WarnContext(ctx, "settle: transient settlement failure, retrying with backoff",
				"channel", channelAddress, "amount", amount, "attempt", attempt, "wait", wait, "error", sendErr)
		}

		if sleepErr := opts.Sleep(ctx, opts.Jitter(wait)); sleepErr != nil {
			slog.ErrorContext(ctx, "settle: retry wait interrupted; leaving the settlement submitting for reconciliation",
				"channel", channelAddress, "amount", amount, "attempt", attempt, "error", sleepErr)
			return rec, fmt.Errorf("settle: retry: %w", sleepErr)
		}

		if !tightening {
			delay = time.Duration(float64(delay) * opts.Multiplier)
			if delay > opts.MaxDelay {
				delay = opts.MaxDelay
			}
		}
	}
}
