package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belaytzev/tdmeter/checker"
	"github.com/belaytzev/tdmeter/config"
	"github.com/belaytzev/tdmeter/metrics"
	"github.com/belaytzev/tdmeter/web"
)

// Scheduler orchestrates periodic proxy health checks with bounded concurrency.
type Scheduler struct {
	proxies     []config.ProxyConfig
	tcp         *checker.TCPChecker
	tdlib       checker.Checker
	metrics     *metrics.Metrics
	store       *web.StatusStore
	concurrency int
	interval    time.Duration
	stopCh      chan struct{}
	done        chan struct{}
	started     atomic.Bool
	startOnce   sync.Once
	stopOnce    sync.Once
}

// New creates a Scheduler with the given dependencies.
func New(
	proxies []config.ProxyConfig,
	tcp *checker.TCPChecker,
	tdlib checker.Checker,
	m *metrics.Metrics,
	store *web.StatusStore,
	concurrency int,
	interval time.Duration,
) *Scheduler {
	return &Scheduler{
		proxies:     proxies,
		tcp:         tcp,
		tdlib:       tdlib,
		metrics:     m,
		store:       store,
		concurrency: concurrency,
		interval:    interval,
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// RunCheckRound performs a single round of proxy health checks using a bounded
// worker pool. Each proxy undergoes TCP check, then TDLib check if TCP succeeds.
func (s *Scheduler) RunCheckRound(ctx context.Context, proxies []config.ProxyConfig) []checker.Result {
	results := make([]checker.Result, len(proxies))
	work := make(chan int, len(proxies))

	var wg sync.WaitGroup
	for i := 0; i < s.concurrency && i < len(proxies); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				results[idx] = s.checkProxy(ctx, proxies[idx])
			}
		}()
	}

	for i := range proxies {
		work <- i
	}
	close(work)

	wg.Wait()
	return results
}

func (s *Scheduler) checkProxy(ctx context.Context, p config.ProxyConfig) checker.Result {
	r := checker.Result{
		Name:      p.Name,
		Server:    p.Server,
		Port:      fmt.Sprintf("%d", p.Port),
		CheckedAt: time.Now(),
	}

	tcpOk, _, tcpErr := s.tcp.Check(ctx, p.Server, p.Port)
	if tcpErr != nil {
		slog.Debug("tcp check failed", "proxy", p.Name, "err", tcpErr)
	}

	var tdlibOk bool
	if tcpOk {
		latency, tdlibErr := s.tdlib.Check(ctx, p.Server, p.Port, p.Secret)
		if tdlibErr != nil {
			slog.Debug("tdlib check failed", "proxy", p.Name, "err", tdlibErr)
		} else {
			tdlibOk = true
			r.LatencyMs = latency
		}
	}

	r.Status = checker.DetermineStatus(tcpOk, tdlibOk)
	if r.Status != checker.StatusOnline {
		r.LatencyMs = -1
	}
	return r
}

// Start begins periodic check rounds at the configured interval.
// It runs the first round immediately, then repeats on a ticker.
// It is safe to call Start multiple times; only the first call has effect.
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() { s.startLoop(ctx) })
}

func (s *Scheduler) startLoop(ctx context.Context) {
	s.started.Store(true)
	go func() {
		defer close(s.done)

		s.runAndUpdate(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runAndUpdate(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Scheduler) runAndUpdate(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	start := time.Now()
	results := s.RunCheckRound(ctx, s.proxies)
	duration := time.Since(start)

	s.metrics.Update(results, duration)
	s.store.Update(results)

	online, degraded, offline := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case checker.StatusOnline:
			online++
		case checker.StatusDegraded:
			degraded++
		case checker.StatusOffline:
			offline++
		}
	}
	slog.Info("check round complete",
		"total", len(results),
		"online", online,
		"degraded", degraded,
		"offline", offline,
		"duration", duration.Round(time.Millisecond),
	)
}

// Stop signals the scheduler to stop and waits for it to finish.
// It is safe to call Stop multiple times. If Start was never called, Stop returns immediately.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	if s.started.Load() {
		<-s.done
	}
}
