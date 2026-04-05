package checker

import "testing"

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name     string
		tcpOk    bool
		tdlibOk  bool
		expected Status
	}{
		{"both ok returns online", true, true, StatusOnline},
		{"tcp ok tdlib fail returns degraded", true, false, StatusDegraded},
		{"both fail returns offline", false, false, StatusOffline},
		{"tcp fail tdlib ok returns offline", false, true, StatusOffline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineStatus(tt.tcpOk, tt.tdlibOk)
			if got != tt.expected {
				t.Errorf("DetermineStatus(%v, %v) = %q, want %q", tt.tcpOk, tt.tdlibOk, got, tt.expected)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusOnline != "online" {
		t.Errorf("StatusOnline = %q, want %q", StatusOnline, "online")
	}
	if StatusDegraded != "degraded" {
		t.Errorf("StatusDegraded = %q, want %q", StatusDegraded, "degraded")
	}
	if StatusOffline != "offline" {
		t.Errorf("StatusOffline = %q, want %q", StatusOffline, "offline")
	}
}
