package metrics

import (
	"testing"
	"time"

	"github.com/belaytzev/tdmeter/checker"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func getGaugeValue(g *prometheus.GaugeVec, labels prometheus.Labels) float64 {
	var m dto.Metric
	gauge, err := g.GetMetricWith(labels)
	if err != nil {
		return -999
	}
	gauge.(prometheus.Metric).Write(&m)
	return m.GetGauge().GetValue()
}

func getScalarGaugeValue(g prometheus.Gauge) float64 {
	var m dto.Metric
	g.(prometheus.Metric).Write(&m)
	return m.GetGauge().GetValue()
}

func TestUpdate_OnlineProxy(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "proxy1", Server: "1.2.3.4", Port: "443", Status: checker.StatusOnline, LatencyMs: 42.5},
	}
	m.Update(results, 2*time.Second)

	labels := prometheus.Labels{"name": "proxy1", "server": "1.2.3.4", "port": "443"}
	if v := getGaugeValue(m.proxyUp, labels); v != 1 {
		t.Errorf("proxy_up: got %v, want 1", v)
	}
	if v := getGaugeValue(m.proxyDegraded, labels); v != 0 {
		t.Errorf("proxy_degraded: got %v, want 0", v)
	}
	if v := getGaugeValue(m.proxyLatency, labels); v != 42.5 {
		t.Errorf("proxy_latency_ms: got %v, want 42.5", v)
	}
}

func TestUpdate_DegradedProxy(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "proxy2", Server: "5.6.7.8", Port: "8443", Status: checker.StatusDegraded, LatencyMs: 0},
	}
	m.Update(results, time.Second)

	labels := prometheus.Labels{"name": "proxy2", "server": "5.6.7.8", "port": "8443"}
	if v := getGaugeValue(m.proxyUp, labels); v != 0 {
		t.Errorf("proxy_up: got %v, want 0", v)
	}
	if v := getGaugeValue(m.proxyDegraded, labels); v != 1 {
		t.Errorf("proxy_degraded: got %v, want 1", v)
	}
	if v := getGaugeValue(m.proxyLatency, labels); v != -1 {
		t.Errorf("proxy_latency_ms: got %v, want -1", v)
	}
}

func TestUpdate_OfflineProxy(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "proxy3", Server: "9.10.11.12", Port: "1080", Status: checker.StatusOffline, LatencyMs: 0},
	}
	m.Update(results, time.Second)

	labels := prometheus.Labels{"name": "proxy3", "server": "9.10.11.12", "port": "1080"}
	if v := getGaugeValue(m.proxyUp, labels); v != 0 {
		t.Errorf("proxy_up: got %v, want 0", v)
	}
	if v := getGaugeValue(m.proxyDegraded, labels); v != 0 {
		t.Errorf("proxy_degraded: got %v, want 0", v)
	}
	if v := getGaugeValue(m.proxyLatency, labels); v != -1 {
		t.Errorf("proxy_latency_ms: got %v, want -1", v)
	}
}

func TestUpdate_MixedStatuses(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "p1", Server: "1.1.1.1", Port: "443", Status: checker.StatusOnline, LatencyMs: 10},
		{Name: "p2", Server: "2.2.2.2", Port: "443", Status: checker.StatusOnline, LatencyMs: 20},
		{Name: "p3", Server: "3.3.3.3", Port: "443", Status: checker.StatusDegraded},
		{Name: "p4", Server: "4.4.4.4", Port: "443", Status: checker.StatusOffline},
		{Name: "p5", Server: "5.5.5.5", Port: "443", Status: checker.StatusOffline},
	}
	m.Update(results, 5*time.Second)

	// Check proxies_total counts
	if v := getGaugeValue(m.proxiesTotal, prometheus.Labels{"status": "online"}); v != 2 {
		t.Errorf("proxies_total{online}: got %v, want 2", v)
	}
	if v := getGaugeValue(m.proxiesTotal, prometheus.Labels{"status": "degraded"}); v != 1 {
		t.Errorf("proxies_total{degraded}: got %v, want 1", v)
	}
	if v := getGaugeValue(m.proxiesTotal, prometheus.Labels{"status": "offline"}); v != 2 {
		t.Errorf("proxies_total{offline}: got %v, want 2", v)
	}
}

func TestUpdate_CheckDuration(t *testing.T) {
	m := New()
	m.Update(nil, 3500*time.Millisecond)

	if v := getScalarGaugeValue(m.checkDuration); v != 3.5 {
		t.Errorf("check_duration_seconds: got %v, want 3.5", v)
	}
}

func TestUpdate_LabelCorrectness(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "my-proxy", Server: "example.com", Port: "8443", Status: checker.StatusOnline, LatencyMs: 15},
	}
	m.Update(results, time.Second)

	// Verify correct labels are set by querying with exact labels
	labels := prometheus.Labels{"name": "my-proxy", "server": "example.com", "port": "8443"}
	if v := getGaugeValue(m.proxyUp, labels); v != 1 {
		t.Errorf("proxy_up with exact labels: got %v, want 1", v)
	}

	// Verify wrong labels return no value (GetMetricWith creates new zero-value series)
	wrongLabels := prometheus.Labels{"name": "wrong", "server": "wrong", "port": "0"}
	if v := getGaugeValue(m.proxyUp, wrongLabels); v != 0 {
		t.Errorf("proxy_up with wrong labels: got %v, want 0", v)
	}
}

func TestHandler_NotNil(t *testing.T) {
	m := New()
	if m.Handler() == nil {
		t.Error("Handler() returned nil")
	}
}

func TestMetricCount(t *testing.T) {
	m := New()
	results := []checker.Result{
		{Name: "p1", Server: "1.1.1.1", Port: "443", Status: checker.StatusOnline, LatencyMs: 10},
		{Name: "p2", Server: "2.2.2.2", Port: "443", Status: checker.StatusDegraded},
	}
	m.Update(results, time.Second)

	// Gather all metrics from the registry
	families, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Should have 5 metric families
	if len(families) != 5 {
		names := make([]string, len(families))
		for i, f := range families {
			names[i] = *f.Name
		}
		t.Errorf("metric families: got %d (%v), want 5", len(families), names)
	}
}
