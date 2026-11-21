package settle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tallybook-Org/tallybook/internal/httpserver"
)

func ExampleMetrics_ServeHTTP() {
	// ServeHTTP always sets Content-Type to Prometheus text format.
	// A metric that fails to collect is logged and omitted; the endpoint
	// still serves 200 with whatever data it could gather (partial data
	// beats no data). Here all queries fail because the context is
	// already cancelled before ServeHTTP is called.
	pool, _ := pgxpool.New(context.Background(), "host=localhost user=unused dbname=unused port=5432")
	m := NewMetrics(pool, fakeLedgerSource{ledger: 0}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/metrics", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, req)
	fmt.Println(w.Header().Get("Content-Type"))
	// Output:
	// text/plain; version=0.0.4; charset=utf-8
}

func serveMetrics(t *testing.T, m *Metrics) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestMetrics_OpenChannels(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	insertTestChannel(t, pool, testChannel[:len(testChannel)-1]+"1", "closing", 0, nil, nil)
	insertTestChannel(t, pool, testChannel[:len(testChannel)-1]+"2", "closed", 0, nil, nil)
	insertTestChannel(t, pool, testChannel[:len(testChannel)-1]+"3", "refunded", 0, nil, nil)

	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	body := serveMetrics(t, m)

	if !strings.Contains(body, "tallybook_settle_open_channels 2\n") {
		t.Errorf("body does not contain the expected open-channels line (want 2, closed/refunded excluded):\n%s", body)
	}
}

func TestMetrics_UnsettledExposureByToken(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 1000, nil, nil) // testToken
	insertValidCommitmentForSettle(t, pool, testChannel, 4000)      // unsettled = 3000

	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	body := serveMetrics(t, m)

	want := `tallybook_settle_unsettled_exposure{token="` + testToken + `"} 3000`
	if !strings.Contains(body, want) {
		t.Errorf("body does not contain %q:\n%s", want, body)
	}
}

func TestMetrics_NearestDeadline(t *testing.T) {
	pool := testSettlePool(t)
	near := int32(1500)
	far := int32(9000)
	insertTestChannel(t, pool, testChannel, "closing", 0, ptrInt32(1000), &near)
	insertTestChannel(t, pool, testChannel[:len(testChannel)-1]+"1", "closing", 0, ptrInt32(8000), &far)

	m := NewMetrics(pool, fakeLedgerSource{ledger: 1000}, nil)
	body := serveMetrics(t, m)

	// Nearest deadline is 1500; current ledger 1000; remaining = 500.
	if !strings.Contains(body, "tallybook_settle_nearest_refund_deadline_ledgers_remaining 500\n") {
		t.Errorf("body does not contain the expected deadline-remaining line:\n%s", body)
	}
}

func TestMetrics_NearestDeadlineCanBeNegativeWhenOverdue(t *testing.T) {
	pool := testSettlePool(t)
	deadline := int32(500)
	insertTestChannel(t, pool, testChannel, "closing", 0, ptrInt32(0), &deadline)

	m := NewMetrics(pool, fakeLedgerSource{ledger: 1000}, nil) // already 500 ledgers past deadline
	body := serveMetrics(t, m)

	if !strings.Contains(body, "tallybook_settle_nearest_refund_deadline_ledgers_remaining -500\n") {
		t.Errorf("body does not contain the expected negative (overdue) deadline line:\n%s", body)
	}
}

func TestMetrics_NoWatchedDeadlineOmitsTheMetric(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil) // no deadline at all

	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	body := serveMetrics(t, m)

	if strings.Contains(body, "tallybook_settle_nearest_refund_deadline_ledgers_remaining") {
		t.Errorf("body contains the deadline metric even though no watched channel has one:\n%s", body)
	}
}

func TestMetrics_SettleCounts(t *testing.T) {
	pool := testSettlePool(t)
	insertTestChannel(t, pool, testChannel, "open", 0, nil, nil)
	for _, s := range []struct {
		amount int64
		status string
	}{
		{100, "confirmed"}, {200, "failed"}, {300, "failed"}, {400, "submitting"},
	} {
		if s.status == "confirmed" {
			_, err := pool.Exec(context.Background(),
				`INSERT INTO settlements (channel, cumulative_amount, status, confirmed_at) VALUES ($1, $2, $3, now())`,
				testChannel, s.amount, s.status,
			)
			if err != nil {
				t.Fatalf("insert settlement: %v", err)
			}
			continue
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO settlements (channel, cumulative_amount, status) VALUES ($1, $2, $3)`,
			testChannel, s.amount, s.status,
		); err != nil {
			t.Fatalf("insert settlement: %v", err)
		}
	}

	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	body := serveMetrics(t, m)

	if !strings.Contains(body, "tallybook_settle_attempts_total 4\n") {
		t.Errorf("body does not contain attempts_total=4:\n%s", body)
	}
	if !strings.Contains(body, "tallybook_settle_failures_total 2\n") {
		t.Errorf("body does not contain failures_total=2:\n%s", body)
	}
}

func TestMetrics_LastChainReadSeconds(t *testing.T) {
	pool := testSettlePool(t)
	daemon := NewDaemon(pool, NewCalculator(pool), NewSubmitter(pool), fixedFactory(newFakeLiveChannel("h", nil)), nil,
		fakeLedgerSource{ledger: 100}, defaultPolicy())

	if _, _, err := daemon.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned unexpected error: %v", err)
	}

	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, daemon)
	body := serveMetrics(t, m)

	if !strings.Contains(body, "tallybook_settle_last_chain_read_seconds ") {
		t.Errorf("body does not contain the last-chain-read gauge:\n%s", body)
	}
}

func TestMetrics_NilDaemonOmitsLastChainReadGauge(t *testing.T) {
	pool := testSettlePool(t)
	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	body := serveMetrics(t, m)

	if strings.Contains(body, "tallybook_settle_last_chain_read_seconds") {
		t.Errorf("body contains the last-chain-read gauge despite a nil Daemon:\n%s", body)
	}
}

// TestMetrics_OneFailingMetricDoesNotBreakTheRest is the partial-failure
// resilience case: the ledger source (needed only for the deadline gauge)
// fails, but every other metric must still be served.
func TestMetrics_OneFailingMetricDoesNotBreakTheRest(t *testing.T) {
	pool := testSettlePool(t)
	deadline := int32(1500)
	insertTestChannel(t, pool, testChannel, "closing", 0, ptrInt32(1000), &deadline)

	m := NewMetrics(pool, fakeLedgerSource{err: errors.New("rpc down")}, nil)
	body := serveMetrics(t, m)

	if strings.Contains(body, "tallybook_settle_nearest_refund_deadline_ledgers_remaining") {
		t.Errorf("body contains the deadline metric despite the ledger source failing:\n%s", body)
	}
	if !strings.Contains(body, "tallybook_settle_open_channels 1\n") {
		t.Errorf("body does not contain open_channels despite it not depending on the ledger source:\n%s", body)
	}
}

func TestMetrics_ContentType(t *testing.T) {
	pool := testSettlePool(t)
	m := NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil)
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want a text/plain prefix", ct)
	}
}

func TestMetricsConcurrentReadersServeWithinTimeout(t *testing.T) {
	pool := testSettlePool(t)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", NewMetrics(pool, fakeLedgerSource{ledger: 100}, nil))
	srv, err := httpserver.New("127.0.0.1:0", mux, httpserver.Timeouts{
		ReadHeader: time.Second,
		Read:       time.Second,
		Write:      3 * time.Second,
		Idle:       time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()
	t.Cleanup(func() {
		if err := srv.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("close metrics server: %v", err)
		}
		<-serveErr
	})

	const readers = 16
	client := &http.Client{Timeout: 2 * time.Second}
	var wg sync.WaitGroup
	errCh := make(chan error, readers)
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get("http://" + listener.Addr().String() + "/metrics")
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errCh <- err
				return
			}
			if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "tallybook_settle_open_channels") {
				errCh <- errors.New("metrics response was incomplete")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent metrics request failed: %v", err)
	}
}
