package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/belaytzev/tdmeter/checker"
	"github.com/belaytzev/tdmeter/config"
	"github.com/belaytzev/tdmeter/metrics"
	"github.com/belaytzev/tdmeter/scheduler"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("TDMeter starting", "config", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"proxies", len(cfg.Proxies),
		"interval", cfg.CheckInterval,
		"concurrency", cfg.Concurrency,
		"listen", cfg.Metrics.Listen,
	)

	tcpChecker := checker.NewTCPChecker(cfg.TCPTimeout)

	tdlibChecker, err := checker.NewTDLibChecker(cfg.TDLib.APIID, cfg.TDLib.APIHash, cfg.TDLib.DBPath, cfg.TDLibTimeout)
	if err != nil {
		slog.Error("failed to initialize TDLib checker", "error", err)
		os.Exit(1)
	}

	m := metrics.New()

	sched := scheduler.New(cfg.Proxies, tcpChecker, tdlibChecker, m, cfg.Concurrency, cfg.CheckInterval)

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())

	srv := &http.Server{
		Addr:              cfg.Metrics.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	srvErrCh := make(chan error, 1)
	go func() {
		slog.Info("metrics server starting", "addr", cfg.Metrics.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErrCh <- err
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	slog.Info("scheduler started", "interval", cfg.CheckInterval)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var sig os.Signal
	select {
	case sig = <-sigCh:
		slog.Info("shutdown signal received", "signal", sig)
	case err := <-srvErrCh:
		slog.Error("metrics server failed, shutting down", "error", err)
	}

	cancel()
	sched.Stop()
	slog.Info("scheduler stopped")

	if err := tdlibChecker.Close(); err != nil {
		slog.Error("failed to close TDLib checker", "error", err)
	}
	slog.Info("TDLib checker closed")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown metrics server", "error", err)
	}
	slog.Info("metrics server stopped")

	slog.Info("TDMeter shutdown complete")
}
