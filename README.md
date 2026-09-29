<p align="center">
  <img src="web/logo.png" alt="TDMeter" width="96" />
</p>

<h1 align="center">TDMeter</h1>

<p align="center">
  Health checks for the proxies you use to reach Telegram: MTProto, SOCKS5, and HTTP.
</p>

<p align="center">
  <a href="#quick-start"><img src="https://img.shields.io/badge/quick--start-Docker-blue?logo=docker" alt="Quick Start" /></a>
  <a href="#prometheus-metrics"><img src="https://img.shields.io/badge/metrics-Prometheus-orange?logo=prometheus" alt="Prometheus" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green" alt="License: MIT" /></a>
</p>

---

TDMeter checks each proxy on a schedule and tells you whether Telegram actually works through it. It has a small web dashboard, a JSON API, a health endpoint per proxy (handy for Uptime Kuma), and Prometheus metrics.

You don't need a Telegram account. TDLib can ping proxies before login, so TDMeter never authorizes.

<!-- <p align="center"><img src=".github/screenshot.png" alt="TDMeter Dashboard" width="800" /></p> -->

## How it works

Every proxy goes through two checks on each round:

1. **TCP connect** to the proxy's host and port. If this fails, the proxy is offline and the second check is skipped.
2. **TDLib `pingProxy`**: TDLib connects to a Telegram datacenter through the proxy and measures the round trip.

```
TCP connect ──fail──► offline
     │
     ok
     ▼
TDLib pingProxy ──fail──► degraded
     │
     ok
     ▼
   online (with latency)
```

| Status | TCP | TDLib | What it means |
|--------|-----|-------|---------------|
| online | ok | ok | Telegram works through the proxy |
| degraded | ok | fail | The proxy answers, but Telegram doesn't work through it: wrong secret or credentials, or the proxy won't connect to Telegram |
| offline | fail | skipped | The proxy host doesn't accept connections |

Degraded is the interesting one. A plain port check would call these proxies healthy.

## Quick start

TDMeter is easiest to run in Docker, since the image builds TDLib for you.

```bash
cp config.example.yaml config.yaml
# put your api_id, api_hash, and proxies into config.yaml

docker build -t tdmeter .
docker run -d \
  -v $(pwd)/config.yaml:/etc/tdmeter/config.yaml:ro \
  -p 2112:2112 \
  --name tdmeter \
  --restart unless-stopped \
  tdmeter
```

The dashboard is at http://localhost:2112 and metrics are at http://localhost:2112/metrics.

With Docker Compose:

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

Compiling TDLib needs at least 4 GB of RAM. If the build runs out of memory, lower the parallelism with `--build-arg TDLIB_BUILD_JOBS=2` (or `1`). The Dockerfile has three stages: TDLib is built from source on Alpine, then the Go binary is linked statically against it, and the result is copied into a plain Alpine image of about 60 MB.

## Configuration

TDMeter reads a YAML file (`--config`, default `config.yaml`). A few values can also come from environment variables, see below.

```yaml
tdlib:
  api_id: 12345                  # from https://my.telegram.org
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

  - name: "socks-office"
    type: socks5
    server: "10.0.0.5"
    port: 1080
    username: "monitor"          # optional
    password: "changeme"

  - name: "http-gateway"
    type: http
    server: "gw.example.com"
    port: 3128

metrics:
  listen: ":2112"

web:
  auth:
    username: ""                 # leave both empty to disable auth
    password: ""

check_interval: 60s
tcp_timeout: 5s
tdlib_timeout: 10s
concurrency: 5
```

### Reference

| Field | Default | Description |
|-------|---------|-------------|
| `tdlib.api_id` | required | Telegram API ID |
| `tdlib.api_hash` | required | Telegram API hash |
| `tdlib.db_path` | `/tmp/tdmeter-tdlib/` | TDLib database directory |
| `proxies` | required, at least one | Proxies to check |
| `proxies[].name` | required | Display name, also used in `/health/{name}` |
| `proxies[].type` | `mtproto` | `mtproto`, `socks5`, or `http` |
| `proxies[].server` | required | Hostname or IP |
| `proxies[].port` | required | 1 to 65535 |
| `proxies[].secret` | required for `mtproto` | Hex-encoded MTProto secret. Not allowed for other types |
| `proxies[].username` | empty | `socks5` and `http` only. For SOCKS5 it must be under 128 bytes |
| `proxies[].password` | empty | `socks5` and `http` only. For SOCKS5 it must be under 128 bytes |
| `proxies[].http_only` | `false` | `http` only. Set it for proxies that don't support `CONNECT`; TDLib then sends plain HTTP requests |
| `metrics.listen` | `:2112` | Listen address for the dashboard, API, and metrics |
| `web.auth.username` | empty | Basic auth user. Set both user and password, or neither |
| `web.auth.password` | empty | Basic auth password |
| `check_interval` | `60s` | Time between check rounds |
| `tcp_timeout` | `5s` | Timeout for the TCP check |
| `tdlib_timeout` | `10s` | Timeout for the TDLib ping |
| `concurrency` | `5` | How many proxies are checked at once |

TDMeter refuses to start if a proxy has fields that don't fit its type, for example a `secret` on a SOCKS5 proxy. This catches a mixed-up `type` early instead of reporting the proxy as degraded forever.

### MTProto secrets

Secrets are hex strings. The first byte selects the mode:

| Prefix | Mode |
|--------|------|
| `ee` | Fake-TLS, the most common one. Traffic looks like TLS |
| `dd` | Padded intermediate. Adds random padding |
| none | Plain intermediate obfuscation |

### SOCKS5 and HTTP proxies

These are checked the same way as MTProto proxies. If the proxy accepts the TCP connection but rejects the credentials or won't connect to Telegram, it shows as degraded.

One thing to watch with HTTP proxies: TDLib opens `CONNECT` tunnels to Telegram datacenter IPs on port 443, 80, or 5222. Squid, with its default `SSL_ports` ACL, only allows `CONNECT` to 443, and many other proxies do the same. Such a proxy may flip between online and degraded depending on which port TDLib picks. Allow those ports for Telegram's IP ranges, or set `http_only: true` if the proxy can't do `CONNECT` at all.

### Environment variables

These override the YAML values:

| Variable | Overrides |
|----------|-----------|
| `TDMETER_API_ID` | `tdlib.api_id` |
| `TDMETER_API_HASH` | `tdlib.api_hash` |
| `TDMETER_AUTH_USERNAME` | `web.auth.username` |
| `TDMETER_AUTH_PASSWORD` | `web.auth.password` |

For example, to keep the dashboard password out of the config file:

```bash
docker run -d \
  -v $(pwd)/config.yaml:/etc/tdmeter/config.yaml:ro \
  -e TDMETER_AUTH_USERNAME=admin \
  -e TDMETER_AUTH_PASSWORD=supersecret \
  -p 2112:2112 \
  tdmeter
```

## Dashboard

The dashboard lives at `/`. It shows counts by status at the top, a card per proxy with its type, address, and latency, and filter buttons for each status. It refreshes on the check interval, and you can turn that off. Hover a card to get a link to its health endpoint.

The logo comes from `web/logo.png` and is embedded into the binary, so replace the file and rebuild to change it.

## HTTP endpoints

| Endpoint | Behind basic auth | Description |
|----------|-------------------|-------------|
| `GET /` | if enabled | Dashboard |
| `GET /api/status` | if enabled | All proxies as JSON |
| `GET /health/{name}` | if enabled | One proxy: 200 when online, 503 otherwise. The name is case-insensitive |
| `GET /metrics` | never | Prometheus metrics |
| `GET /logo.png` | never | Logo |

`/metrics` stays open even with basic auth on, so Prometheus can scrape it without credentials.

`GET /api/status` returns:

```json
{
  "proxies": [
    {
      "name": "proxy-eu-1",
      "type": "mtproto",
      "server": "proxy1.example.com",
      "port": "443",
      "status": "online",
      "latency_ms": 142.5
    },
    {
      "name": "socks-office",
      "type": "socks5",
      "server": "10.0.0.5",
      "port": "1080",
      "status": "offline",
      "latency_ms": -1
    }
  ],
  "last_check": "2025-01-15T12:00:05Z"
}
```

### Uptime Kuma

Add an HTTP(s) monitor per proxy that points at `http://your-tdmeter:2112/health/{proxy-name}` and expects status 200:

```
/health/proxy-eu-1    200  {"status":"online","latency_ms":142.5}
/health/socks-office  503  {"status":"offline"}
```

If basic auth is on, put the credentials in the monitor's authentication settings.

## Prometheus metrics

```yaml
scrape_configs:
  - job_name: tdmeter
    static_configs:
      - targets: ["tdmeter:2112"]
```

| Metric | Labels | Description |
|--------|--------|-------------|
| `tdmeter_proxy_up` | `name`, `server`, `port` | 1 if online, 0 otherwise |
| `tdmeter_proxy_degraded` | `name`, `server`, `port` | 1 if degraded, 0 otherwise |
| `tdmeter_proxy_latency_ms` | `name`, `server`, `port` | Round trip in ms, -1 if not online |
| `tdmeter_check_duration_seconds` | none | How long the last round took |
| `tdmeter_proxies_total` | `status` | Number of proxies per status |

All five are gauges. A few queries to start with:

```promql
# is this proxy up
tdmeter_proxy_up{name="proxy-eu-1"}

# average latency of online proxies
avg(tdmeter_proxy_latency_ms > 0)

# how many proxies are offline
tdmeter_proxies_total{status="offline"}
```

## Building from source

You need:

- Go 1.26 or newer
- TDLib 1.8.46, built from commit `b498497` (the version go-tdlib v1.0.0-beta1 expects). See the [TDLib build instructions](https://tdlib.github.io/td/build.html), or copy the steps from the `Dockerfile`
- `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org)

```bash
git clone https://github.com/belaytzev/tdmeter.git
cd tdmeter
CGO_ENABLED=1 go build -tags=tdlib -o tdmeter .
cp config.example.yaml config.yaml
./tdmeter --config config.yaml
```

Without `-tags=tdlib` the binary still builds, but it uses a stub checker and exits on startup. That build is only useful for running tests.

### Tests

```bash
go test ./...                          # no TDLib needed
CGO_ENABLED=1 go test -tags=tdlib ./...  # also runs the TDLib-specific tests, needs TDLib installed
```

## Project layout

```
main.go                 entrypoint, HTTP server, shutdown
config/config.go        YAML and env loading, validation
checker/checker.go      status types, Checker interface
checker/tcp.go          TCP check
checker/tdlib.go        TDLib check (build tag: tdlib)
scheduler/scheduler.go  runs check rounds with bounded concurrency
metrics/metrics.go      Prometheus gauges
web/handler.go          dashboard, API, health, and logo handlers
web/auth.go             basic auth middleware
web/store.go            latest results, safe for concurrent use
web/embed.go            embeds the template and logo
web/templates/index.html  dashboard (Alpine.js)
```

## Contributing

Pull requests are welcome. CI runs `gofmt`, `go vet` (with and without the `tdlib` tag), and `go test -race` on every PR, then does a full Docker build. Running the first three locally before you push saves a round trip.

## License

MIT
