//go:build tdlib

package checker

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/belaytzev/tdmeter/config"
	tdlib "github.com/zelenin/go-tdlib/client"
)

// TDLibChecker checks Telegram proxy connectivity using TDLib's addProxy and
// pingProxy APIs. It maintains a single TDLib client in an unauthenticated
// state -- proxy methods do not require Telegram authorization.
//
// TDLib's JSON interface (td_json_client) is thread-safe for concurrent
// requests, so no mutex is needed around addProxy/pingProxy/removeProxy calls.
type TDLibChecker struct {
	client    *tdlib.Client
	timeout   time.Duration
	done      chan struct{}
	closeOnce sync.Once
	inflight  sync.WaitGroup
}

// proxyOnlyAuthorizer handles TDLib authorization just enough to set parameters,
// then pauses at WaitPhoneNumber. The client remains usable for proxy operations.
type proxyOnlyAuthorizer struct {
	params   *tdlib.SetTdlibParametersRequest
	clientCh chan *tdlib.Client
	done     chan struct{}
}

func (a *proxyOnlyAuthorizer) Handle(c *tdlib.Client, state tdlib.AuthorizationState) error {
	switch state.AuthorizationStateConstructor() {
	case tdlib.ConstructorAuthorizationStateWaitTdlibParameters:
		_, err := c.SetTdlibParameters(context.Background(), a.params)
		return err
	case tdlib.ConstructorAuthorizationStateWaitPhoneNumber:
		// Export the client reference so the checker can use it for proxy ops.
		select {
		case a.clientCh <- c:
		default:
		}
		// Block until the checker is shutting down.
		<-a.done
		return fmt.Errorf("tdmeter: proxy checker shutting down")
	default:
		return tdlib.NotSupportedAuthorizationState(state)
	}
}

func (a *proxyOnlyAuthorizer) Close() {}

// NewTDLibChecker creates a TDLib client configured for proxy checking.
// The client initializes with the given API credentials and database path,
// remaining in an unauthenticated state suitable for addProxy/pingProxy calls.
func NewTDLibChecker(apiID int32, apiHash string, dbPath string, timeout time.Duration) (*TDLibChecker, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath is required")
	}

	auth := &proxyOnlyAuthorizer{
		params: &tdlib.SetTdlibParametersRequest{
			UseTestDc:           false,
			DatabaseDirectory:   dbPath,
			FilesDirectory:      filepath.Join(dbPath, "files"),
			UseFileDatabase:     false,
			UseChatInfoDatabase: false,
			UseMessageDatabase:  false,
			ApiId:               apiID,
			ApiHash:             apiHash,
			SystemLanguageCode:  "en",
			DeviceModel:         "TDMeter",
			ApplicationVersion:  "1.0.0",
		},
		clientCh: make(chan *tdlib.Client, 1),
		done:     make(chan struct{}),
	}

	// Silence TDLib's internal logging. As of go-tdlib v1.0.0-beta1 this is no
	// longer a NewClient option; SetLogVerbosityLevel runs synchronously via
	// Execute and needs no client.
	if _, err := tdlib.SetLogVerbosityLevel(&tdlib.SetLogVerbosityLevelRequest{NewVerbosityLevel: 0}); err != nil {
		return nil, fmt.Errorf("tdlib set log verbosity failed: %w", err)
	}

	// NewClient blocks on Authorize. We run it in a goroutine because our
	// authorizer intentionally pauses at WaitPhoneNumber.
	errCh := make(chan error, 1)
	go func() {
		_, err := tdlib.NewClient(auth)
		if err != nil {
			errCh <- err
		}
	}()

	// Wait for the client to reach WaitPhoneNumber (ready for proxy ops) or fail.
	select {
	case c := <-auth.clientCh:
		return &TDLibChecker{
			client:  c,
			timeout: timeout,
			done:    auth.done,
		}, nil
	case err := <-errCh:
		return nil, fmt.Errorf("tdlib init failed: %w", err)
	case <-time.After(30 * time.Second):
		close(auth.done)
		return nil, fmt.Errorf("tdlib init timed out")
	}
}

func proxyType(p config.ProxyConfig) (tdlib.ProxyType, error) {
	switch p.Type {
	case config.ProxyTypeMTProto:
		return &tdlib.ProxyTypeMtproto{Secret: p.Secret}, nil
	case config.ProxyTypeSOCKS5:
		return &tdlib.ProxyTypeSocks5{Username: p.Username, Password: p.Password}, nil
	case config.ProxyTypeHTTP:
		return &tdlib.ProxyTypeHttp{Username: p.Username, Password: p.Password, HttpOnly: p.HTTPOnly}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", p.Type)
	}
}

// Check performs an addProxy + pingProxy sequence against the given proxy,
// returning the round-trip latency in milliseconds. The proxy entry is
// removed after the check.
func (c *TDLibChecker) Check(ctx context.Context, p config.ProxyConfig) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if c.client == nil {
		return 0, fmt.Errorf("tdlib client not initialized")
	}

	pt, err := proxyType(p)
	if err != nil {
		return 0, err
	}

	type checkResult struct {
		latencyMs float64
		err       error
	}

	ch := make(chan checkResult, 1)
	c.inflight.Add(1)
	go func() {
		defer c.inflight.Done()
		proxy, err := c.client.AddProxy(ctx, &tdlib.AddProxyRequest{
			Server: p.Server,
			Port:   int32(p.Port),
			Enable: false,
			Type:   pt,
		})
		if err != nil {
			ch <- checkResult{err: fmt.Errorf("addProxy failed: %w", err)}
			return
		}
		defer func() {
			// Use a fresh context so cleanup runs even if the check's ctx expired.
			if _, err := c.client.RemoveProxy(context.Background(), &tdlib.RemoveProxyRequest{ProxyId: proxy.Id}); err != nil {
				slog.Warn("removeProxy failed", "proxyId", proxy.Id, "error", err)
			}
		}()

		seconds, err := c.client.PingProxy(ctx, &tdlib.PingProxyRequest{ProxyId: proxy.Id})
		if err != nil {
			ch <- checkResult{err: fmt.Errorf("pingProxy failed: %w", err)}
			return
		}

		ch <- checkResult{latencyMs: seconds.Seconds * 1000}
	}()

	select {
	case res := <-ch:
		return res.latencyMs, res.err
	case <-ctx.Done():
		return 0, fmt.Errorf("tdlib check timed out: %w", ctx.Err())
	}
}

// Close shuts down the TDLib client and releases resources.
// It is safe to call Close multiple times.
func (c *TDLibChecker) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	c.inflight.Wait()
	return nil
}
