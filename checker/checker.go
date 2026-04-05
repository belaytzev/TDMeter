package checker

import "time"

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
