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

func TestCheckerInterface(t *testing.T) {
	// Verify mockChecker satisfies the Checker interface.
	var _ Checker = (*mockChecker)(nil)
}

func TestMockChecker_SuccessfulCheck(t *testing.T) {
	mock := &mockChecker{latency: 42.5}

	latency, err := mock.Check(context.Background(), "proxy.example.com", 443, "ee0123456789abcdef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latency != 42.5 {
		t.Errorf("latency = %f, want 42.5", latency)
	}
}

func TestMockChecker_FailedCheck(t *testing.T) {
	mock := &mockChecker{err: fmt.Errorf("proxy unreachable")}

	_, err := mock.Check(context.Background(), "proxy.example.com", 443, "ee0123456789abcdef")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "proxy unreachable" {
		t.Errorf("error = %q, want %q", err.Error(), "proxy unreachable")
	}
}

func TestMockChecker_Close(t *testing.T) {
	mock := &mockChecker{}
	if mock.closed {
		t.Fatal("expected closed=false before Close()")
	}
	if err := mock.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.closed {
		t.Fatal("expected closed=true after Close()")
	}
}

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

func TestResultBuildingWithMockChecker(t *testing.T) {
	// Simulate building a Result from check outcomes.
	mock := &mockChecker{latency: 85.3}

	latency, err := mock.Check(context.Background(), "nl-proxy.example.com", 8443, "ee1234")
	if err != nil {
		t.Fatal(err)
	}

	status := DetermineStatus(true, true)
	now := time.Now()

	result := Result{
		Name:      "nl-proxy-1",
		Server:    "nl-proxy.example.com",
		Port:      "8443",
		Status:    status,
		LatencyMs: latency,
		CheckedAt: now,
	}

	if result.Status != StatusOnline {
		t.Errorf("result.Status = %q, want %q", result.Status, StatusOnline)
	}
	if result.LatencyMs != 85.3 {
		t.Errorf("result.LatencyMs = %f, want 85.3", result.LatencyMs)
	}
	if result.Name != "nl-proxy-1" {
		t.Errorf("result.Name = %q, want %q", result.Name, "nl-proxy-1")
	}
}

func TestTDLibCheckerStub(t *testing.T) {
	// In non-tdlib builds, NewTDLibChecker returns an error.
	_, err := NewTDLibChecker(12345, "hash", "/tmp/test", 10*time.Second)
	if err == nil {
		t.Fatal("expected error from stub NewTDLibChecker, got nil")
	}
}
