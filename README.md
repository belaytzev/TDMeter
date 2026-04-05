<p align="center">
  <img src="web/logo.png" alt="TDMeter" width="96" />
</p>

<h1 align="center">TDMeter</h1>

<p align="center">
  <strong>MTProto Proxy Health Monitor</strong><br>
  Two-stage health checks for Telegram MTProto proxies with a real-time web dashboard, Prometheus metrics, and monitoring integrations.
</p>

<p align="center">
  <a href="#-quick-start"><img src="https://img.shields.io/badge/quick--start-Docker-blue?logo=docker" alt="Quick Start" /></a>
  <a href="#-prometheus-metrics"><img src="https://img.shields.io/badge/metrics-Prometheus-orange?logo=prometheus" alt="Prometheus" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green" alt="License: MIT" /></a>
</p>

---

## 🔍 What Is TDMeter?

TDMeter monitors your Telegram MTProto proxy servers by running **two-stage health checks** and reporting the results through a beautiful dark-themed web dashboard, a JSON API, per-proxy health endpoints, and Prometheus metrics.

No Telegram account or authentication is required. TDLib's proxy testing works in an unauthenticated state.

<!-- Screenshot placeholder -->
<!-- <p align="center"><img src=".github/screenshot.png" alt="TDMeter Dashboard" width="800" /></p> -->

## 🚀 Key Features

- 🔬 **Two-stage health checks** — TCP connectivity test + TDLib MTProto protocol verification
- 🟢🟡🔴 **Three-state status model** — Online, Degraded, and Offline for precise diagnostics
- 🖥️ **Real-time web dashboard** — Dark theme, auto-refresh, status filtering (Alpine.js + custom CSS)
- 📊 **Prometheus metrics** — Five gauges covering proxy status, latency, and check duration
- 🩺 **Per-proxy health endpoints** — `/health/{name}` returns 200/503 for Uptime Kuma integration
- 🔌 **JSON API** — `/api/status` for custom integrations
- 🔒 **Optional Basic Auth** — Protects web routes while keeping `/metrics` open for Prometheus
- ⚙️ **YAML + env var config** — File-based configuration with environment variable overrides
- 🐳 **Docker ready** — Multi-stage build, single binary, ~30MB runtime image
- 📦 **Single binary** — All templates and the logo are embedded via `go:embed`

## ⚙️ How It Works

TDMeter performs a **two-stage check** for every proxy on each interval:

```
┌──────────────┐      ┌───────────────────┐      ┌──────────────────┐
│  Stage 1     │  OK  │  Stage 2          │  OK  │  Status:         │
│  TCP Connect ├─────►│  TDLib pingProxy  ├─────►│  🟢 Online       │
└──────┬───────┘      └────────┬──────────┘      └──────────────────┘
       │                       │
       │ FAIL                  │ FAIL
       ▼                       ▼
┌──────────────────┐   ┌──────────────────┐
│  Status:         │   │  Status:         │
│  🔴 Offline      │   │  🟡 Degraded     │
└──────────────────┘   └──────────────────┘
```

| Status | TCP | TDLib | Meaning |
|--------|-----|-------|---------|
| 🟢 **Online** | ✅ | ✅ | Proxy is fully functional |
| 🟡 **Degraded** | ✅ | ❌ | Server reachable but MTProto protocol fails |
| 🔴 **Offline** | ❌ | — | Server unreachable, TDLib check is skipped |

## 🚀 Quick Start

### 🐳 Docker

```bash
# 1. Create your config file
cp config.example.yaml config.yaml
# Edit config.yaml with your Telegram API credentials and proxy list

# 2. Build and run
docker build -t tdmeter .
docker run -d \
  -v $(pwd)/config.yaml:/etc/tdmeter/config.yaml:ro \
  -p 2112:2112 \
  --name tdmeter \
  tdmeter
```

Open **http://localhost:2112** for the dashboard, or check metrics at **http://localhost:2112/metrics**.

### 🐳 Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  tdmeter:
    build: .
    volumes:
      - ./config.yaml:/etc/tdmeter/config.yaml:ro
    ports:
      - "2112:2112"
    restart: unless-stopped
```

```bash
docker compose up -d
```

> **💡 Note:** TDLib compilation requires **4GB+ RAM**. The Docker build uses a three-stage approach:
> 1. Build TDLib from source (Alpine + CMake)
> 2. Build Go binary with CGO and static TDLib linking
> 3. Minimal Alpine runtime image (~30MB)

## 🔨 Build from Source

### 📋 Prerequisites

- Go 1.24+
- TDLib (built from source, pinned to commit `22d49d5` for go-tdlib v0.7.6 compatibility)
- Telegram API credentials — `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org)

### 🛠️ Build

Install TDLib first. See the [TDLib build instructions](https://tdlib.github.io/td/build.html).

```bash
# Clone and build
git clone https://github.com/belaytzev/tdmeter.git
cd tdmeter

# Build with TDLib support (CGO required)
CGO_ENABLED=1 go build -tags=tdlib -o tdmeter .

# Copy and edit config
cp config.example.yaml config.yaml

# Run
./tdmeter --config config.yaml
```

### 🧪 Running Tests

```bash
# Run unit tests (no TDLib required)
go test ./...

# TDLib integration tests require the tdlib build tag and a working TDLib installation
CGO_ENABLED=1 go test -tags=tdlib ./...
```

## 📝 Configuration

TDMeter uses a YAML config file with optional environment variable overrides.

Copy `config.example.yaml` and edit it:

```yaml
tdlib:
  api_id: 12345                          # From https://my.telegram.org
  api_hash: "your_api_hash_here"
  db_path: "/tmp/tdmeter-tdlib/"

proxies:
  - name: "proxy-eu-1"
    server: "proxy1.example.com"
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"

  - name: "proxy-us-1"
    server: "proxy2.example.com"
    port: 8443
    secret: "dd0123456789abcdef0123456789abcdef"

metrics:
  listen: ":2112"

web:
  auth:
    username: ""                          # Leave empty to disable auth
    password: ""

check_interval: 60s
tcp_timeout: 5s
tdlib_timeout: 10s
concurrency: 5
```

### 📋 Config Reference

| Field | Default | Description |
|-------|---------|-------------|
| `tdlib.api_id` | *(required)* | Telegram API ID |
| `tdlib.api_hash` | *(required)* | Telegram API Hash |
| `tdlib.db_path` | `/tmp/tdmeter-tdlib/` | TDLib database directory |
| `proxies` | *(required, min 1)* | List of MTProto proxies to monitor |
| `proxies[].name` | *(required)* | Display name for the proxy |
| `proxies[].server` | *(required)* | Proxy server hostname or IP |
| `proxies[].port` | *(required)* | Proxy port (1-65535) |
| `proxies[].secret` | *(required)* | Hex-encoded MTProto secret |
| `metrics.listen` | `:2112` | Address for HTTP server (dashboard + metrics) |
| `web.auth.username` | *(empty)* | Basic auth username (set both or neither) |
| `web.auth.password` | *(empty)* | Basic auth password (set both or neither) |
| `check_interval` | `60s` | How often to run health checks |
| `tcp_timeout` | `5s` | TCP connection timeout |
| `tdlib_timeout` | `10s` | TDLib proxy test timeout |
| `concurrency` | `5` | Maximum concurrent proxy checks |

### 🔑 MTProto Secret Format

Secrets are hex-encoded. The prefix byte determines the mode:

| Prefix | Mode | Description |
|--------|------|-------------|
| `ee` | Fake-TLS | Most common. Wraps MTProto in TLS to avoid detection |
| `dd` | Padded intermediate | Adds padding to obfuscate traffic patterns |
| *(none)* | Simple intermediate | Basic MTProto obfuscation |

### 🌍 Environment Variable Overrides

Environment variables take precedence over YAML values:

| Variable | Overrides | Example |
|----------|-----------|---------|
| `TDMETER_API_ID` | `tdlib.api_id` | `TDMETER_API_ID=12345` |
| `TDMETER_API_HASH` | `tdlib.api_hash` | `TDMETER_API_HASH=abc123...` |
| `TDMETER_AUTH_USERNAME` | `web.auth.username` | `TDMETER_AUTH_USERNAME=admin` |
| `TDMETER_AUTH_PASSWORD` | `web.auth.password` | `TDMETER_AUTH_PASSWORD=secret` |

## 🖥️ Web Dashboard

TDMeter ships with a built-in dark-themed web dashboard at the root URL (`/`).

<!-- <p align="center"><img src=".github/screenshot.png" alt="TDMeter Dashboard" width="800" /></p> -->

**Dashboard features:**
- 📊 Summary stats bar — total, online, degraded, offline counts at a glance
- 🔍 Status filter buttons — quickly filter proxies by status
- 🔄 Auto-refresh — polls the API on each check interval (toggleable)
- ⚡ Latency display — shows RTT in milliseconds for online proxies
- 🔗 Health endpoint links — hover any proxy card to copy its health URL
- 📱 Responsive design — works on desktop and mobile
- 🎨 Custom logo support — replace `web/logo.png` and rebuild

## 🩺 Monitoring Integration

### 📡 Uptime Kuma

TDMeter exposes per-proxy health endpoints designed for [Uptime Kuma](https://github.com/louislam/uptime-kuma):

1. In Uptime Kuma, create a new monitor of type **HTTP(s)**
2. Set the URL to `http://your-tdmeter:2112/health/{proxy-name}`
3. Set expected status code to **200**
4. The endpoint returns **200** when the proxy is online and **503** when degraded or offline

**Example:**

```
http://localhost:2112/health/proxy-eu-1  →  200 {"status":"online","latency_ms":142.5}
http://localhost:2112/health/proxy-us-1  →  503 {"status":"offline"}
```

> **💡 Tip:** If you enabled Basic Auth, configure the credentials in Uptime Kuma's authentication settings for the monitor.

### 📊 Prometheus + Grafana

Scrape the `/metrics` endpoint (always unauthenticated, even when Basic Auth is enabled):

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'tdmeter'
    static_configs:
      - targets: ['tdmeter:2112']
```

## 🔌 API Endpoints

| Endpoint | Auth | Method | Description |
|----------|------|--------|-------------|
| `/` | 🔒 Optional | GET | Web dashboard (HTML) |
| `/api/status` | 🔒 Optional | GET | All proxy statuses as JSON |
| `/health/{name}` | 🔒 Optional | GET | Single proxy health (200/503) |
| `/metrics` | 🔓 Open | GET | Prometheus metrics |
| `/logo.png` | 🔓 Open | GET | Embedded logo image |

> **🔒 Optional** = protected only when `web.auth.username` and `web.auth.password` are configured.
> **🔓 Open** = always unauthenticated (so Prometheus can scrape without credentials).

### 📄 JSON API Response

`GET /api/status`

```json
{
  "proxies": [
    {
      "name": "proxy-eu-1",
      "server": "proxy1.example.com",
      "port": "443",
      "status": "online",
      "latency_ms": 142.5
    },
    {
      "name": "proxy-us-1",
      "server": "proxy2.example.com",
      "port": "8443",
      "status": "offline",
      "latency_ms": -1
    }
  ],
  "last_check": "2025-01-15T12:00:05Z"
}
```

## 📊 Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tdmeter_proxy_up` | Gauge | `name`, `server`, `port` | `1` if online, `0` otherwise |
| `tdmeter_proxy_degraded` | Gauge | `name`, `server`, `port` | `1` if degraded, `0` otherwise |
| `tdmeter_proxy_latency_ms` | Gauge | `name`, `server`, `port` | RTT in milliseconds, `-1` if unreachable |
| `tdmeter_check_duration_seconds` | Gauge | — | Wall-clock time of entire check round |
| `tdmeter_proxies_total` | Gauge | `status` | Count of proxies by status (`online`/`degraded`/`offline`) |

### 📈 Example Grafana Queries

```promql
# Proxy availability (1 = up, 0 = down)
tdmeter_proxy_up{name="proxy-eu-1"}

# Average latency across all online proxies
avg(tdmeter_proxy_latency_ms > 0)

# Count of offline proxies
tdmeter_proxies_total{status="offline"}

# Check round duration
tdmeter_check_duration_seconds
```

## 🐳 Docker Deployment

### 🔧 Build Image

```bash
docker build -t tdmeter .
```

### ▶️ Run Container

```bash
docker run -d \
  -v $(pwd)/config.yaml:/etc/tdmeter/config.yaml:ro \
  -p 2112:2112 \
  --name tdmeter \
  --restart unless-stopped \
  tdmeter
```

### 🔐 With Basic Auth via Environment Variables

```bash
docker run -d \
  -v $(pwd)/config.yaml:/etc/tdmeter/config.yaml:ro \
  -e TDMETER_AUTH_USERNAME=admin \
  -e TDMETER_AUTH_PASSWORD=supersecret \
  -p 2112:2112 \
  --name tdmeter \
  --restart unless-stopped \
  tdmeter
```

### 📂 Project Structure

```
tdmeter/
├── main.go                 # 🚀 Entrypoint, HTTP server, signal handling
├── config/
│   └── config.go           # ⚙️ YAML + env var config loading & validation
├── checker/
│   ├── checker.go          # 🔍 Status types, Checker interface, DetermineStatus
│   ├── tcp.go              # 🌐 TCP connectivity checker
│   └── tdlib.go            # 📡 TDLib MTProto proxy checker (build tag: tdlib)
├── scheduler/
│   └── scheduler.go        # ⏱️ Periodic check orchestrator with bounded concurrency
├── metrics/
│   └── metrics.go          # 📊 Prometheus gauge registration & updates
├── web/
│   ├── handler.go          # 🖥️ Dashboard, API, health, and logo handlers
│   ├── auth.go             # 🔒 Basic auth middleware
│   ├── store.go            # 💾 Thread-safe status store
│   ├── embed.go            # 📦 Embedded templates & logo (go:embed)
│   ├── logo.png            # 🎨 Dashboard logo
│   └── templates/
│       └── index.html      # 🖥️ Alpine.js dashboard template
├── config.example.yaml     # 📝 Example configuration
├── Dockerfile              # 🐳 Multi-stage build (TDLib + Go + Alpine)
└── README.md               # 📖 You are here
```

## 🤝 Contributing

Contributions are welcome! If you want to help:

1. Fork the repository
2. Create a branch for your changes
3. Make and test your changes
4. Create a Pull Request

## 📄 License

MIT
