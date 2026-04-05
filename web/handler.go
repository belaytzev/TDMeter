package web

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/belaytzev/tdmeter/checker"
)

// HealthHandler returns an http.Handler that serves per-proxy health checks.
// GET /health/{name} returns 200 if online, 503 otherwise, 404 if not found.
func HealthHandler(store *StatusStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/health/")
		if name == "" {
			http.NotFound(w, r)
			return
		}

		result, ok := store.FindByName(name)
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		resp := map[string]any{
			"status": string(result.Status),
		}
		if result.Status == checker.StatusOnline {
			resp["latency_ms"] = result.LatencyMs
		}

		if result.Status != checker.StatusOnline {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(resp)
	})
}

// APIStatusHandler returns an http.Handler that serves all proxy statuses as JSON.
// GET /api/status returns {"proxies": [...], "last_check": "..."}.
func APIStatusHandler(store *StatusStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		results, lastCheck := store.Results()

		type proxyJSON struct {
			Name      string  `json:"name"`
			Server    string  `json:"server"`
			Port      string  `json:"port"`
			Status    string  `json:"status"`
			LatencyMs float64 `json:"latency_ms"`
		}

		proxies := make([]proxyJSON, len(results))
		for i, r := range results {
			proxies[i] = proxyJSON{
				Name:      r.Name,
				Server:    r.Server,
				Port:      r.Port,
				Status:    string(r.Status),
				LatencyMs: r.LatencyMs,
			}
		}

		lastCheckStr := ""
		if !lastCheck.IsZero() {
			lastCheckStr = lastCheck.Format("2006-01-02T15:04:05Z07:00")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"proxies":    proxies,
			"last_check": lastCheckStr,
		})
	})
}

// LogoHandler returns an http.Handler that serves the embedded logo.
func LogoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(logoPNG)
	})
}

// DashboardHandler returns an http.Handler that serves the HTML dashboard.
func DashboardHandler(checkInterval time.Duration) http.Handler {
	tmpl, err := template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		panic("failed to parse dashboard template: " + err.Error())
	}

	refreshMs := checkInterval.Milliseconds()
	if refreshMs < 5000 {
		refreshMs = 5000
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, struct {
			RefreshInterval int64
			LogoHash        string
		}{RefreshInterval: refreshMs, LogoHash: LogoHash}); err != nil {
			slog.Error("failed to render dashboard", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	})
}
