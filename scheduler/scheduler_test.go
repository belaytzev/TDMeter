package scheduler

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belaytzev/tdmeter/checker"
	"github.com/belaytzev/tdmeter/config"
	"github.com/belaytzev/tdmeter/metrics"
)

// mockChecker implements checker.Checker for testing.
type mockChecker struct {
	latency float64
	err     error
	calls   atomic.Int32
}

func (m *mockChecker) Check(_ context.Context, _ string, _ int, _ string) (float64, error) {
	m.calls.Add(1)
	return m.latency, m.err
}

func (m *mockChecker) Close() error { return nil }

// conditionalMockChecker returns success on first call, error on subsequent calls.
type conditionalMockChecker struct {
	counter *atomic.Int32
	latency float64
}

func (c *conditionalMockChecker) Check(_ context.Context, _ string, _ int, _ string) (float64, error) {
	n := c.counter.Add(1)
	if n == 1 {
		return c.latency, nil
	}
	return 0, fmt.Errorf("tdlib failed")
}

func (c *conditionalMockChecker) Close() error { return nil }

func testProxies(n int) []config.ProxyConfig {
	proxies := make([]config.ProxyConfig, n)
	for i := 0; i < n; i++ {
		proxies[i] = config.ProxyConfig{
			Name:   fmt.Sprintf("proxy-%d", i),
			Server: "127.0.0.1",
			Port:   10000 + i,
			Secret: "ee0000000000000000000000000000000000",
		}
	}
	return proxies
}

func startListeners(t *testing.T, n int) []net.Listener {
	t.Helper()
	lns := make([]net.Listener, n)
	for i := 0; i < n; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to start listener %d: %v", i, err)
		}
		lns[i] = ln
	}
	return lns
}

func closeListeners(lns []net.Listener) {
	for _, ln := range lns {
		ln.Close()
	}
}

func listenerProxy(t *testing.T, ln net.Listener, name string) config.ProxyConfig {
	t.Helper()
	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	return config.ProxyConfig{
		Name:   name,
		Server: host,
		Port:   port,
		Secret: "ee0000000000000000000000000000000000",
	}
}


func TestRunCheckRound_AllOffline(t *testing.T) {
	tcp := checker.NewTCPChecker(50 * time.Millisecond)
	mock := &mockChecker{latency: 42.0}
	m := metrics.New()

	s := New(nil, tcp, mock, m, 2, time.Minute)
	proxies := testProxies(3)

	results := s.RunCheckRound(context.Background(), proxies)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Status != checker.StatusOffline {
			t.Errorf("result[%d]: expected offline, got %s", i, r.Status)
		}
		if r.LatencyMs != -1 {
			t.Errorf("result[%d]: expected latency -1, got %f", i, r.LatencyMs)
		}
	}
	if mock.calls.Load() != 0 {
		t.Errorf("expected 0 tdlib calls when TCP fails, got %d", mock.calls.Load())
	}
}

func TestRunCheckRound_Degraded(t *testing.T) {
	lns := startListeners(t, 2)
	defer closeListeners(lns)

	tcp := checker.NewTCPChecker(time.Second)
	mock := &mockChecker{err: fmt.Errorf("tdlib timeout")}
	m := metrics.New()

	s := New(nil, tcp, mock, m, 2, time.Minute)

	proxies := make([]config.ProxyConfig, len(lns))
	for i, ln := range lns {
		proxies[i] = listenerProxy(t, ln, fmt.Sprintf("proxy-%d", i))
	}

	results := s.RunCheckRound(context.Background(), proxies)

	for i, r := range results {
		if r.Status != checker.StatusDegraded {
			t.Errorf("result[%d]: expected degraded, got %s", i, r.Status)
		}
		if r.LatencyMs != -1 {
			t.Errorf("result[%d]: expected latency -1 for degraded, got %f", i, r.LatencyMs)
		}
	}
	if int(mock.calls.Load()) != len(proxies) {
		t.Errorf("expected %d tdlib calls, got %d", len(proxies), mock.calls.Load())
	}
}

func TestRunCheckRound_Online(t *testing.T) {
	lns := startListeners(t, 3)
	defer closeListeners(lns)

	tcp := checker.NewTCPChecker(time.Second)
	mock := &mockChecker{latency: 55.5}
	m := metrics.New()

	s := New(nil, tcp, mock, m, 5, time.Minute)

	proxies := make([]config.ProxyConfig, len(lns))
	for i, ln := range lns {
		proxies[i] = listenerProxy(t, ln, fmt.Sprintf("proxy-%d", i))
	}

	results := s.RunCheckRound(context.Background(), proxies)

	for i, r := range results {
		if r.Status != checker.StatusOnline {
			t.Errorf("result[%d]: expected online, got %s", i, r.Status)
		}
		if r.LatencyMs != 55.5 {
			t.Errorf("result[%d]: expected latency 55.5, got %f", i, r.LatencyMs)
		}
	}
}

func TestRunCheckRound_Concurrency(t *testing.T) {
	tcp := checker.NewTCPChecker(50 * time.Millisecond)
	mock := &mockChecker{}
	m := metrics.New()

	s := New(nil, tcp, mock, m, 2, time.Minute)
	proxies := testProxies(5)

	results := s.RunCheckRound(context.Background(), proxies)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Name != fmt.Sprintf("proxy-%d", i) {
			t.Errorf("result[%d]: expected name proxy-%d, got %s", i, i, r.Name)
		}
		if r.Status != checker.StatusOffline {
			t.Errorf("result[%d]: expected offline, got %s", i, r.Status)
		}
	}
}

func TestRunCheckRound_MixedStatus(t *testing.T) {
	lns := startListeners(t, 2)
	defer closeListeners(lns)

	callCount := atomic.Int32{}
	tdlib := &conditionalMockChecker{
		counter: &callCount,
		latency: 30.0,
	}
	tcp := checker.NewTCPChecker(time.Second)
	m := metrics.New()

	// concurrency=1 to ensure deterministic ordering
	s := New(nil, tcp, tdlib, m, 1, time.Minute)

	proxies := []config.ProxyConfig{
		listenerProxy(t, lns[0], "online"),
		listenerProxy(t, lns[1], "degraded"),
		{Name: "offline", Server: "127.0.0.1", Port: 19999, Secret: "ee0000000000000000000000000000000000"},
	}

	results := s.RunCheckRound(context.Background(), proxies)

	if results[0].Status != checker.StatusOnline {
		t.Errorf("expected online, got %s", results[0].Status)
	}
	if results[0].LatencyMs != 30.0 {
		t.Errorf("expected latency 30.0, got %f", results[0].LatencyMs)
	}
	if results[1].Status != checker.StatusDegraded {
		t.Errorf("expected degraded, got %s", results[1].Status)
	}
	if results[2].Status != checker.StatusOffline {
		t.Errorf("expected offline, got %s", results[2].Status)
	}
}

func TestStartAndStop(t *testing.T) {
	tcp := checker.NewTCPChecker(50 * time.Millisecond)
	mock := &mockChecker{latency: 10}
	m := metrics.New()

	proxies := testProxies(1)
	s := New(proxies, tcp, mock, m, 1, 50*time.Millisecond)

	ctx := context.Background()
	s.Start(ctx)

	// Wait enough for the immediate run to complete
	time.Sleep(100 * time.Millisecond)

	s.Stop()
	// If Stop returns without hanging, the test passes
}
