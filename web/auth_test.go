package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	handler := BasicAuth("admin", "secret123", dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid creds, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestBasicAuth_NoCredentials(t *testing.T) {
	handler := BasicAuth("admin", "secret123", dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without creds, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
}

func TestBasicAuth_WrongPassword(t *testing.T) {
	handler := BasicAuth("admin", "secret123", dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrongpass")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong password, got %d", rec.Code)
	}
}

func TestBasicAuth_WrongUsername(t *testing.T) {
	handler := BasicAuth("admin", "secret123", dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("wrong", "secret123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong username, got %d", rec.Code)
	}
}
