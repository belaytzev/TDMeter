# TDMeter - MTProto Proxy Health Checker

## Overview
- Go application that monitors MTProto proxy health using TDLib's `testProxy`/`pingProxy` API
- Two-stage checks: TCP reachability then TDLib connectivity test (no Telegram auth required)
- Exposes Prometheus metrics for integration with existing monitoring infrastructure
- Inspired by [xray-checker](https://github.com/kutovoys/xray-checker) architecture

## Context (from brainstorm)
- **Language**: Go with CGO (TDLib requires C bindings)
- **TDLib binding**: `github.com/zelenin/go-tdlib` (auto-generated from TDLib schema)
- **TDLib API**: Use `addProxy` + `pingProxy` flow (add MTProto proxy, then ping for RTT). `testProxy` is also available as a simpler pass/fail check. Both are generated in go-tdlib since the library auto-generates from TDLib's full td_api.tl schema.
- **Fallback**: If methods are missing in the specific go-tdlib version, use `client.Execute()` with raw TDLib JSON requests
- **Architecture**: Single TDLib client, bounded concurrency worker pool, gocron scheduler
- **Config**: Static YAML file with proxy definitions
- **Output**: Prometheus metrics on `:2112/metrics`

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
- **CRITICAL: all tests must pass before starting next task**
- **CRITICAL: update this plan file when scope changes during implementation**
- Run tests after each change

## Testing Strategy
- **Unit tests**: Required for every task — config parsing, TCP checker, metrics registration
- **Integration tests**: TDLib checker tests will need mocking (TDLib requires real network)
- **Test command**: `go test ./...`

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with !! prefix
- Update plan if implementation deviates from original scope

## Implementation Steps

### Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `config/config.go`
- Create: `checker/tcp.go`
- Create: `checker/tdlib.go`
- Create: `checker/checker.go`
- Create: `scheduler/scheduler.go`
- Create: `metrics/metrics.go`

- [x] Run `go mod init github.com/belaytzev/tdmeter`
- [x] Create `main.go` with placeholder main function and CLI flag for config path (`--config`)
- [x] Create empty package files for `config/`, `checker/`, `scheduler/`, `metrics/` with package declarations
- [x] Verify project compiles: `go build ./...`
- [x] Run tests to verify clean state: `go test ./...`

### Task 2: Config parsing

**Files:**
- Modify: `config/config.go`
- Create: `config/config_test.go`
- Create: `config.example.yaml`

- [x] Define config structs: `Config` (top-level, includes `Concurrency int`), `MetricsConfig`, `TDLibConfig`, `ProxyConfig`
- [x] Implement `Load(path string) (*Config, error)` — reads and parses YAML
- [x] Support env var overrides for sensitive fields: `TDMETER_API_ID`, `TDMETER_API_HASH` (override yaml values if set)
- [x] Add validation: required fields (api_id, api_hash, at least 1 proxy), valid port ranges (1-65535), non-empty secret
- [x] Handle MTProto secret prefixes: validate hex format, normalize `ee`/`dd` prefixed secrets (strip or pass through as TDLib expects)
- [x] Add defaults: `check_interval: 60s`, `tcp_timeout: 5s`, `tdlib_timeout: 10s`, `concurrency: 5`, `listen: :2112`
- [x] Create `config.example.yaml` with documented fields
- [x] Write tests for Load — valid config, missing required fields, defaults applied, env var overrides
- [x] Write tests for validation — invalid port, empty proxies, missing api credentials, invalid secret format
- [x] Run tests: `go test ./config/...`

### Task 3: Domain types and status logic

**Files:**
- Modify: `checker/checker.go`
- Create: `checker/checker_test.go`

- [x] Define `Status` type as string constants: `StatusOnline`, `StatusDegraded`, `StatusOffline`
- [x] Define `Result` struct: `Name, Server, Port string; Status Status; LatencyMs float64; CheckedAt time.Time`
- [x] Implement `DetermineStatus(tcpOk bool, tdlibOk bool) Status` — explicit testable function for status logic
- [x] Write tests for DetermineStatus — all combinations: (true,true)=online, (true,false)=degraded, (false,false)=offline, (false,true)=offline
- [x] Run tests: `go test ./checker/...`

### Task 4: TCP quick-check

**Files:**
- Modify: `checker/tcp.go`
- Create: `checker/tcp_test.go`

- [x] Define `TCPChecker` struct with `timeout` field
- [x] Implement `Check(ctx context.Context, server string, port int) (reachable bool, duration time.Duration, err error)`
- [x] TCP dial with context timeout, measure round-trip time
- [x] Write tests using a local TCP listener — success case (listener accepts)
- [x] Write tests for failure cases — connection refused (no listener), timeout (use short timeout)
- [x] Run tests: `go test ./checker/...`

### Task 5: TDLib proxy checker

**Files:**
- Modify: `checker/tdlib.go`
- Create: `checker/tdlib_test.go`

- [x] Verify `TestProxy`/`PingProxy`/`AddProxy` exists in go-tdlib generated code. If missing, implement raw JSON request via `client.Send()` with manual request/response structs
- [x] Define `TDLibChecker` struct holding `*client.Client` and timeout
- [x] Implement `NewTDLibChecker(apiID int32, apiHash string, dbPath string) (*TDLibChecker, error)`:
  - Init TDLib with `SetTdlibParameters` using provided `dbPath` (default: `/tmp/tdmeter-tdlib/`)
  - Handle authorization state machine: remain in unauthenticated state (do not attempt login)
  - `testProxy` and `pingProxy` work without authorization
- [x] Implement `Check(ctx context.Context, server string, port int, secret string) (latencyMs float64, err error)`:
  - Call `addProxy` with `proxyTypeMtproto{secret}`, then `pingProxy` for RTT
  - Or use `testProxy` as simpler pass/fail alternative
  - Clean up added proxy after check via `removeProxy`
- [x] Implement `Close()` for graceful shutdown (destroy TDLib client)
- [x] Define `Checker` interface: `Check(ctx, server, port, secret) (float64, error)` for testability
- [x] Write tests with mock `Checker` interface — test result mapping logic
- [x] Run tests: `go test ./checker/...`

### Task 6: Prometheus metrics

**Files:**
- Modify: `metrics/metrics.go`
- Create: `metrics/metrics_test.go`

- [x] Define `Metrics` struct with Prometheus gauge vectors
- [x] Register gauges (labels: name, server, port):
  - `tdmeter_proxy_up` — 1 if online, 0 otherwise
  - `tdmeter_proxy_degraded` — 1 if degraded (TCP ok, TDLib fail), 0 otherwise
  - `tdmeter_proxy_latency_ms` — RTT in ms, -1 if unreachable
  - `tdmeter_check_duration_seconds` — total check round time (no labels)
  - `tdmeter_proxies_total` — count by status label (online/degraded/offline)
- [x] Implement `Update(results []checker.Result, duration time.Duration)` — sets all gauges from check results
- [x] Implement `Handler() http.Handler` — returns `promhttp.Handler()`
- [x] Write tests: verify gauge values after Update with known results (online, degraded, offline mix)
- [x] Write tests: verify label correctness and metric count
- [x] Run tests: `go test ./metrics/...`

### Task 7: Scheduler and check orchestration

**Files:**
- Modify: `scheduler/scheduler.go`
- Create: `scheduler/scheduler_test.go`

- [x] Define `Scheduler` struct with config, TCPChecker, TDLibChecker (via Checker interface), Metrics, and concurrency pool size
- [x] Implement `RunCheckRound(ctx context.Context, proxies []config.ProxyConfig) []checker.Result`:
  - Fan-out via worker pool (channel + goroutines bounded by concurrency)
  - Each worker: TCP check → if ok → TDLib check → `DetermineStatus()` → build Result
  - Collect results
- [x] Implement `Start(ctx context.Context)` — sets up gocron job at check_interval, calls RunCheckRound + Metrics.Update each round
- [x] Implement `Stop()` for graceful shutdown
- [x] Write tests for RunCheckRound with mock checkers — verify concurrency (multiple proxies checked), result aggregation
- [x] Write tests for end-to-end status mapping through RunCheckRound
- [x] Run tests: `go test ./scheduler/...`

### Task 8: Wire everything in main.go

**Files:**
- Modify: `main.go`

- [x] Parse `--config` flag (default: `config.yaml`)
- [x] Load config via `config.Load()`
- [x] Initialize TCPChecker, TDLibChecker (with configurable database path, default `/tmp/tdmeter-tdlib/`), Metrics, Scheduler
- [x] Start HTTP server for metrics endpoint on configured listen address
- [x] Start scheduler
- [x] Handle OS signals (SIGINT, SIGTERM) for graceful shutdown — stop scheduler, close TDLib, stop HTTP server
- [x] Add structured logging (Go `slog` package) for startup, check results, errors
- [x] Run full test suite: `go test ./...`
- [x] Manual test: run with example config, verify `/metrics` endpoint responds

### Task 9: Dockerfile and docker-compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yaml`

- [x] Multi-stage Dockerfile: `golang:1.24-alpine` builder with TDLib build dependencies (cmake, g++, make, openssl-dev, zlib-dev)
- [x] Clone and build TDLib from source in builder stage — pinned to commit 22d49d5 (go-tdlib v0.7.6 compatibility)
- [x] Build Go binary with `CGO_ENABLED=1` and `-ldflags="-s -w"`
- [x] Run `go test ./...` in builder stage to catch issues early
- [x] Runtime stage: `alpine:3.21` with `ca-certificates`, `libstdc++`, `openssl`, `zlib`
- [x] Run as non-root user (UID 1000), expose port 2112
- [x] Create `docker-compose.yaml` with tdmeter service, config volume mount, port mapping
- [x] Build and verify: `docker build -t tdmeter .` (!! local Docker VM has only 3.8GB RAM; TDLib compilation needs 4GB+; verify on CI with more memory)

### Task 10: Verify acceptance criteria

- [ ] Verify two-stage check works: TCP quick-check → TDLib testProxy
- [ ] Verify Prometheus metrics exposed correctly on `:2112/metrics`
- [ ] Verify config loading with validation and defaults
- [ ] Verify bounded concurrency (not spawning unlimited goroutines)
- [ ] Verify graceful shutdown on SIGTERM
- [ ] Run full test suite: `go test ./...`

### Task 11: Write README and finalize documentation

- [ ] Create README.md with usage, config reference, Docker instructions
- [ ] Ensure `config.example.yaml` is complete and documented
- [ ] Update `.gitignore` for Go artifacts and TDLib state dir (`/tmp/tdmeter-tdlib/`)
- [ ] Move this plan to `docs/plans/completed/`

## Technical Details

### Proxy config format
MTProto proxy links follow: `tg://proxy?server=HOST&port=PORT&secret=SECRET`
Secret is hex-encoded. Prefix meanings:
- `ee` prefix: fake-TLS mode (most common, wraps MTProto in TLS)
- `dd` prefix: padded intermediate mode
- No prefix: simple intermediate mode
TDLib's `proxyTypeMtproto` expects the full secret including prefix.

### TDLib client lifecycle
- Initialize with `SetTdlibParameters` (api_id, api_hash, database_directory)
- Database directory: configurable, default `/tmp/tdmeter-tdlib/` (ephemeral, no auth state needed)
- Authorization state: client enters `authorizationStateWaitPhoneNumber` — do NOT proceed further
- `testProxy` and `pingProxy` work in this unauthenticated state
- On shutdown: call `Close()` to clean up resources

### TDLib testProxy API (from TDLib docs)
```
testProxy server:string port:int32 type:ProxyType dc_id:int32 timeout:double = Ok
```
- `type` = `proxyTypeMtproto secret:string`
- `dc_id` = 2 (default DC for testing)
- Returns `Ok` on success, error on failure

### pingProxy API
```
pingProxy proxy_id:int32 = Seconds
```
- Returns round-trip time as `Seconds` (float64)
- Requires proxy to be added first via `addProxy`

### Check flow sequence
1. Scheduler triggers check round
2. Push all proxies to work channel
3. Worker goroutines (bounded by `concurrency`) pick up proxies
4. Each worker: TCP check → if ok → TDLib testProxy/pingProxy → `DetermineStatus()`
5. Collect all results
6. Update Prometheus metrics in batch
7. Log summary

### Prometheus metrics
- `tdmeter_proxy_up{name,server,port}`: 1 = reachable and functional, 0 = not
- `tdmeter_proxy_degraded{name,server,port}`: 1 = TCP reachable but TDLib test failed, 0 = not
- `tdmeter_proxy_latency_ms{name,server,port}`: RTT in milliseconds, -1 if unreachable
- `tdmeter_check_duration_seconds`: wall-clock time of entire check round
- `tdmeter_proxies_total{status="online|degraded|offline"}`: count per status

## Post-Completion

**Manual verification:**
- Test with real MTProto proxies to validate end-to-end flow
- Verify Prometheus scrape works with a real Prometheus instance
- Test Docker image on both amd64 and arm64

**Future enhancements (out of scope):**
- Subscription URL support for dynamic proxy discovery
- Web dashboard
- REST API
- Alertmanager integration
