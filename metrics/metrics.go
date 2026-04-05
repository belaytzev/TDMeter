package metrics

import (
	"net/http"
	"time"

	"github.com/belaytzev/tdmeter/checker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus gauge vectors for proxy health monitoring.
type Metrics struct {
	proxyUp       *prometheus.GaugeVec
	proxyDegraded *prometheus.GaugeVec
	proxyLatency  *prometheus.GaugeVec
	checkDuration prometheus.Gauge
	proxiesTotal  *prometheus.GaugeVec
	registry      *prometheus.Registry
}

// New creates a new Metrics instance with all gauges registered.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		proxyUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tdmeter_proxy_up",
			Help: "1 if proxy is online (TCP and TDLib ok), 0 otherwise",
		}, []string{"name", "server", "port"}),

		proxyDegraded: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tdmeter_proxy_degraded",
			Help: "1 if proxy is degraded (TCP ok, TDLib fail), 0 otherwise",
		}, []string{"name", "server", "port"}),

		proxyLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tdmeter_proxy_latency_ms",
			Help: "Proxy RTT in milliseconds, -1 if unreachable",
		}, []string{"name", "server", "port"}),

		checkDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tdmeter_check_duration_seconds",
			Help: "Wall-clock time of entire check round in seconds",
		}),

		proxiesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tdmeter_proxies_total",
			Help: "Count of proxies by status",
		}, []string{"status"}),

		registry: reg,
	}

	reg.MustRegister(m.proxyUp)
	reg.MustRegister(m.proxyDegraded)
	reg.MustRegister(m.proxyLatency)
	reg.MustRegister(m.checkDuration)
	reg.MustRegister(m.proxiesTotal)

	return m
}

// Update sets all gauge values from the given check results and round duration.
func (m *Metrics) Update(results []checker.Result, duration time.Duration) {
	counts := map[checker.Status]float64{
		checker.StatusOnline:   0,
		checker.StatusDegraded: 0,
		checker.StatusOffline:  0,
	}

	for _, r := range results {
		labels := prometheus.Labels{
			"name":   r.Name,
			"server": r.Server,
			"port":   r.Port,
		}

		switch r.Status {
		case checker.StatusOnline:
			m.proxyUp.With(labels).Set(1)
			m.proxyDegraded.With(labels).Set(0)
			m.proxyLatency.With(labels).Set(r.LatencyMs)
		case checker.StatusDegraded:
			m.proxyUp.With(labels).Set(0)
			m.proxyDegraded.With(labels).Set(1)
			m.proxyLatency.With(labels).Set(-1)
		case checker.StatusOffline:
			m.proxyUp.With(labels).Set(0)
			m.proxyDegraded.With(labels).Set(0)
			m.proxyLatency.With(labels).Set(-1)
		}

		counts[r.Status]++
	}

	for status, count := range counts {
		m.proxiesTotal.With(prometheus.Labels{"status": string(status)}).Set(count)
	}

	m.checkDuration.Set(duration.Seconds())
}

// Handler returns an HTTP handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
