package checker

import (
	"context"
	"time"
)

// Checker defines the interface for proxy connectivity checks via TDLib.
// Implementations perform the actual TDLib testProxy/pingProxy calls.
// Mock implementations can be used in tests.
type Checker interface {
	Check(ctx context.Context, server string, port int, secret string) (latencyMs float64, err error)
	Close() error
}

// Status represents the health status of a proxy.
type Status string

const (
	StatusOnline   Status = "online"
	StatusDegraded Status = "degraded"
	StatusOffline  Status = "offline"
)

// Result holds the outcome of a proxy health check.
type Result struct {
	Name      string
	Server    string
	Port      string
	Status    Status
	LatencyMs float64
	CheckedAt time.Time
}

// DetermineStatus returns the proxy status based on TCP and TDLib check results.
// Both ok = online, TCP ok but TDLib fail = degraded, TCP fail = offline regardless of TDLib.
func DetermineStatus(tcpOk bool, tdlibOk bool) Status {
	if !tcpOk {
		return StatusOffline
	}
	if !tdlibOk {
		return StatusDegraded
	}
	return StatusOnline
}
