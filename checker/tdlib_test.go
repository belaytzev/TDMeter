package checker

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockChecker implements the Checker interface for testing.
type mockChecker struct {
	latency float64
	err     error
	closed  bool
}

func (m *mockChecker) Check(ctx context.Context, server string, port int, secret string) (float64, error) {
	return m.latency, m.err
}

func (m *mockChecker) Close() error {
	m.closed = true
	return nil
}

// Compile-time interface conformance check.
var _ Checker = (*mockChecker)(nil)

func TestResultMappingWithMockChecker(t *testing.T) {
	// Test the full result-mapping flow: TCP check -> Checker -> DetermineStatus.
	tests := []struct {
		name           string
		tcpOk          bool
		checkerLatency float64
		checkerErr     error
		wantStatus     Status
		wantLatency    float64
	}{
		{
			name:           "both succeed - online",
			tcpOk:          true,
			checkerLatency: 150.0,
			wantStatus:     StatusOnline,
			wantLatency:    150.0,
		},
		{
			name:       "tcp ok but checker fails - degraded",
			tcpOk:      true,
			checkerErr: fmt.Errorf("ping failed"),
			wantStatus: StatusDegraded,
			wantLatency: -1,
		},
		{
			name:       "tcp fail - offline regardless of checker",
			tcpOk:      false,
			wantStatus: StatusOffline,
			wantLatency: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockChecker{latency: tt.checkerLatency, err: tt.checkerErr}

			var tdlibOk bool
			var latencyMs float64

			if tt.tcpOk {
				lat, err := mock.Check(context.Background(), "proxy.example.com", 443, "eesecret")
				tdlibOk = err == nil
				if tdlibOk {
					latencyMs = lat
				} else {
					latencyMs = -1
				}
			} else {
				latencyMs = -1
			}

			status := DetermineStatus(tt.tcpOk, tdlibOk)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if latencyMs != tt.wantLatency {
				t.Errorf("latency = %f, want %f", latencyMs, tt.wantLatency)
			}
		})
	}
}

func TestTDLibCheckerStub(t *testing.T) {
	// In non-tdlib builds, NewTDLibChecker returns an error.
	_, err := NewTDLibChecker(12345, "hash", "/tmp/test", 10*time.Second)
	if err == nil {
		t.Fatal("expected error from stub NewTDLibChecker, got nil")
	}
}
