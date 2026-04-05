package checker

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPChecker performs TCP connectivity checks against proxy endpoints.
type TCPChecker struct {
	timeout time.Duration
}

// NewTCPChecker creates a TCPChecker with the given timeout.
func NewTCPChecker(timeout time.Duration) *TCPChecker {
	return &TCPChecker{timeout: timeout}
}

// Check attempts a TCP connection to server:port and returns whether it was
// reachable, the round-trip duration, and any error encountered.
func (c *TCPChecker) Check(ctx context.Context, server string, port int) (reachable bool, duration time.Duration, err error) {
	addr := fmt.Sprintf("%s:%d", server, port)

	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	dialer := net.Dialer{
		Deadline: deadline,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	duration = time.Since(start)

	if err != nil {
		return false, duration, err
	}
	conn.Close()
	return true, duration, nil
}
