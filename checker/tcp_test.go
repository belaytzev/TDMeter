package checker

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTCPChecker_Check_Success(t *testing.T) {
	// Start a local TCP listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	// Accept connections in background so the dial succeeds.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	checker := NewTCPChecker(5 * time.Second)

	reachable, duration, err := checker.Check(context.Background(), addr.IP.String(), addr.Port)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !reachable {
		t.Fatal("expected reachable=true")
	}
	if duration <= 0 {
		t.Fatalf("expected positive duration, got: %v", duration)
	}
}

func TestTCPChecker_Check_ConnectionRefused(t *testing.T) {
	// Pick a port that nothing is listening on by binding and immediately closing.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close() // Close immediately so the port is free but nothing listens.

	checker := NewTCPChecker(2 * time.Second)

	reachable, duration, err := checker.Check(context.Background(), addr.IP.String(), addr.Port)
	if reachable {
		t.Fatal("expected reachable=false for refused connection")
	}
	if err == nil {
		t.Fatal("expected an error for refused connection")
	}
	if duration <= 0 {
		t.Fatalf("expected positive duration, got: %v", duration)
	}
}

func TestTCPChecker_Check_Timeout(t *testing.T) {
	checker := NewTCPChecker(100 * time.Millisecond)

	// Use a non-routable address to trigger a timeout.
	reachable, duration, err := checker.Check(context.Background(), "192.0.2.1", 12345)
	if reachable {
		t.Fatal("expected reachable=false for timeout")
	}
	if err == nil {
		t.Fatal("expected an error for timeout")
	}
	if duration < 50*time.Millisecond {
		t.Fatalf("expected duration near timeout, got: %v", duration)
	}
}

func TestTCPChecker_Check_ContextCanceled(t *testing.T) {
	checker := NewTCPChecker(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	reachable, _, err := checker.Check(ctx, "127.0.0.1", 12345)
	if reachable {
		t.Fatal("expected reachable=false for canceled context")
	}
	if err == nil {
		t.Fatal("expected an error for canceled context")
	}
}
