# TDMeter

MTProto proxy health checker that monitors proxy availability using TDLib and exposes Prometheus metrics.

## How It Works

TDMeter performs two-stage health checks on MTProto proxies:

1. TCP quick-check - verifies the proxy server is reachable
2. TDLib connectivity test - uses Telegram's `testProxy`/`pingProxy` API to verify the proxy actually works

Based on results, each proxy gets a status:
- Online - both TCP and TDLib checks pass
- Degraded - TCP reachable but TDLib test fails
- Offline - TCP unreachable

No Telegram authentication is required. TDLib's proxy testing works in an unauthenticated state.

## Requirements

- Go 1.24+
- TDLib (built from source, pinned to commit `22d49d5` for go-tdlib v0.7.6 compatibility)
- Telegram API credentials (api_id and api_hash from https://my.telegram.org)

## Quick Start with Docker

```bash
# Copy and edit the config
cp config.example.yaml config.yaml
# Edit config.yaml with your API credentials and proxy list

# Build and run
docker compose up -d

# Check metrics
curl http://localhost:2112/metrics
```

## Build from Source

TDLib must be installed first. See the [TDLib build instructions](https://tdlib.github.io/td/build.html).

```bash
# Build with TDLib support
CGO_ENABLED=1 go build -tags=tdlib -o tdmeter .

# Run
./tdmeter --config config.yaml
```

## Configuration

Copy `config.example.yaml` to `config.yaml` and edit it:

```yaml
tdlib:
  api_id: 12345
  api_hash: "your_api_hash_here"
  db_path: "/tmp/tdmeter-tdlib/"

proxies:
  - name: "proxy-eu-1"
    server: "proxy1.example.com"
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"

metrics:
  listen: ":2112"

check_interval: 60s
tcp_timeout: 5s
tdlib_timeout: 10s
concurrency: 5
```

### Config Reference

| Field | Default | Description |
|-------|---------|-------------|
| `tdlib.api_id` | (required) | Telegram API ID |
| `tdlib.api_hash` | (required) | Telegram API Hash |
| `tdlib.db_path` | `/tmp/tdmeter-tdlib/` | TDLib database directory |
| `proxies` | (required, min 1) | List of MTProto proxies to monitor |
| `proxies[].name` | (required) | Display name for the proxy |
| `proxies[].server` | (required) | Proxy server hostname or IP |
| `proxies[].port` | (required) | Proxy port (1-65535) |
| `proxies[].secret` | (required) | Hex-encoded MTProto secret |
| `metrics.listen` | `:2112` | Address for the Prometheus metrics endpoint |
| `check_interval` | `60s` | How often to run health checks |
| `tcp_timeout` | `5s` | TCP connection timeout |
| `tdlib_timeout` | `10s` | TDLib proxy test timeout |
| `concurrency` | `5` | Maximum concurrent proxy checks |

### Environment Variable Overrides

These take precedence over YAML values:

| Variable | Overrides |
|----------|-----------|
| `TDMETER_API_ID` | `tdlib.api_id` |
| `TDMETER_API_HASH` | `tdlib.api_hash` |

### MTProto Secret Format

Secrets are hex-encoded. The prefix determines the mode:
- `ee` - fake-TLS mode (most common, wraps MTProto in TLS)
- `dd` - padded intermediate mode
- No prefix - simple intermediate mode

## Prometheus Metrics

| Metric | Labels | Description |
|--------|--------|-------------|
| `tdmeter_proxy_up` | name, server, port | 1 if online, 0 otherwise |
| `tdmeter_proxy_degraded` | name, server, port | 1 if degraded, 0 otherwise |
| `tdmeter_proxy_latency_ms` | name, server, port | RTT in ms, -1 if unreachable |
| `tdmeter_check_duration_seconds` | - | Wall-clock time of entire check round |
| `tdmeter_proxies_total` | status | Count of proxies by status (online/degraded/offline) |

## Docker

### Build

```bash
docker build -t tdmeter .
```

Note: TDLib compilation requires 4GB+ RAM. The Docker build uses a multi-stage approach:
1. Build TDLib from source (alpine + cmake)
2. Build Go binary with CGO
3. Minimal alpine runtime image

### docker-compose

```bash
docker compose up -d
```

The compose file mounts `config.yaml` read-only into the container at `/etc/tdmeter/config.yaml`.

## Running Tests

```bash
go test ./...
```

TDLib integration tests require the `tdlib` build tag and a working TDLib installation. Without it, a stub implementation is used.

## License

MIT
