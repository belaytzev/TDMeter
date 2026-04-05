package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/belaytzev/tdmeter/checker"
)

func setupStore() *StatusStore {
	s := NewStatusStore()
	s.Update([]checker.Result{
		{Name: "fast-proxy", Server: "1.2.3.4", Port: "443", Status: checker.StatusOnline, LatencyMs: 42},
		{Name: "slow-proxy", Server: "5.6.7.8", Port: "8443", Status: checker.StatusDegraded, LatencyMs: -1},
		{Name: "dead-proxy", Server: "9.0.1.2", Port: "443", Status: checker.StatusOffline, LatencyMs: -1},
	})
	return s
}

func TestHealthHandler_OnlineProxy(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/fast-proxy", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "online" {
		t.Fatalf("expected status=online, got %v", body["status"])
	}
	if body["latency_ms"] != float64(42) {
		t.Fatalf("expected latency_ms=42, got %v", body["latency_ms"])
	}
}

func TestHealthHandler_DegradedProxy(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/slow-proxy", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected status=degraded, got %v", body["status"])
	}
}

func TestHealthHandler_OfflineProxy(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/dead-proxy", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHealthHandler_NotFound(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHealthHandler_CaseInsensitive(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/Fast-Proxy", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for case-insensitive match, got %d", rec.Code)
	}
}

func TestHealthHandler_EmptyName(t *testing.T) {
	store := setupStore()
	handler := HealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty name, got %d", rec.Code)
	}
}

// --- API Status endpoint tests ---

type apiProxy struct {
	Name      string  `json:"name"`
	Server    string  `json:"server"`
	Port      string  `json:"port"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
}

type apiResponse struct {
	Proxies   []apiProxy `json:"proxies"`
	LastCheck string     `json:"last_check"`
}

func TestAPIStatusHandler_ReturnsAllProxies(t *testing.T) {
	store := setupStore()
	handler := APIStatusHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp apiResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d", len(resp.Proxies))
	}
	if resp.LastCheck == "" {
		t.Fatal("expected last_check to be set")
	}

	// Verify first proxy details
	p := resp.Proxies[0]
	if p.Name != "fast-proxy" || p.Status != "online" || p.LatencyMs != 42 {
		t.Fatalf("unexpected first proxy: %+v", p)
	}
}

func TestAPIStatusHandler_EmptyStore(t *testing.T) {
	store := NewStatusStore()
	handler := APIStatusHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp apiResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Proxies) != 0 {
		t.Fatalf("expected 0 proxies, got %d", len(resp.Proxies))
	}
}
