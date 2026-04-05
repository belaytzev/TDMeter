package web

import (
	"testing"
	"time"

	"github.com/belaytzev/tdmeter/checker"
)

func TestStatusStore_EmptyBeforeUpdate(t *testing.T) {
	s := NewStatusStore()
	results, lastCheck := s.Results()

	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
	if !lastCheck.IsZero() {
		t.Fatalf("expected zero time, got %v", lastCheck)
	}
}

func TestStatusStore_UpdateAndRetrieve(t *testing.T) {
	s := NewStatusStore()
	now := time.Now()

	input := []checker.Result{
		{Name: "proxy1", Server: "1.2.3.4", Port: "443", Status: checker.StatusOnline, LatencyMs: 42, CheckedAt: now},
		{Name: "proxy2", Server: "5.6.7.8", Port: "8443", Status: checker.StatusOffline, LatencyMs: -1, CheckedAt: now},
	}

	s.Update(input)

	results, lastCheck := s.Results()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if lastCheck.Before(now) {
		t.Fatalf("expected lastCheck >= now, got %v", lastCheck)
	}

	// Verify it's a copy, not the same slice
	input[0].Status = checker.StatusDegraded
	results2, _ := s.Results()
	if results2[0].Status != checker.StatusOnline {
		t.Fatal("results should be a copy, not a reference to the original slice")
	}
}

func TestStatusStore_FindByName(t *testing.T) {
	s := NewStatusStore()
	s.Update([]checker.Result{
		{Name: "my-proxy", Server: "1.2.3.4", Port: "443", Status: checker.StatusOnline, LatencyMs: 10},
		{Name: "other", Server: "5.6.7.8", Port: "443", Status: checker.StatusOffline, LatencyMs: -1},
	})

	r, ok := s.FindByName("my-proxy")
	if !ok {
		t.Fatal("expected to find 'my-proxy'")
	}
	if r.Status != checker.StatusOnline {
		t.Fatalf("expected online, got %s", r.Status)
	}

	_, ok = s.FindByName("nonexistent")
	if ok {
		t.Fatal("expected not found for 'nonexistent'")
	}
}

func TestStatusStore_FindByName_CaseInsensitive(t *testing.T) {
	s := NewStatusStore()
	s.Update([]checker.Result{
		{Name: "My-Proxy", Server: "1.2.3.4", Port: "443", Status: checker.StatusDegraded},
	})

	r, ok := s.FindByName("my-proxy")
	if !ok {
		t.Fatal("expected case-insensitive match")
	}
	if r.Name != "My-Proxy" {
		t.Fatalf("expected original name preserved, got %s", r.Name)
	}
}
