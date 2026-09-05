# Karzoun AtlasMesh

[![CI](https://github.com/mkarson1997/karzoun-atlasmesh/actions/workflows/ci.yml/badge.svg)](https://github.com/mkarson1997/karzoun-atlasmesh/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mkarson1997/karzoun-atlasmesh/actions/workflows/codeql.yml/badge.svg)](https://github.com/mkarson1997/karzoun-atlasmesh/actions/workflows/codeql.yml)
[![Release](https://img.shields.io/github/v/release/mkarson1997/karzoun-atlasmesh)](https://github.com/mkarson1997/karzoun-atlasmesh/releases/latest)
[![License](https://img.shields.io/github/license/mkarson1997/karzoun-atlasmesh)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/mkarson1997/karzoun-atlasmesh)](go.mod)

> A Go control-plane foundation for service discovery, expiring leases, health-aware resolution, cluster membership and at-least-once distributed job execution.

AtlasMesh is project **03/41** in the Karzoun engineering portfolio. The v0.1 foundation deliberately focuses on small, testable distributed-systems primitives before adding a replicated storage backend.

## Why this exists

Modern backends need more than an HTTP server. They need to answer questions such as:

- Which service instances are alive right now?
- How do clients avoid unhealthy instances?
- Who currently owns a job after a worker dies?
- Can a stale worker still commit a result after its lease expires?
- How does a control plane expose health and operational metrics without leaking payloads?

AtlasMesh turns those questions into explicit Go APIs with deterministic behavior and tests.

## v0.1 capabilities

- **Service registry** with TTL leases and automatic expiry
- **Weighted deterministic resolver** that excludes unhealthy instances
- **Cluster membership** with node heartbeats and expiring leases
- **Distributed job substrate** with claims, lease heartbeats, retries and dead-letter state
- **Lease fencing** that rejects stale worker completion after ownership is lost
- **REST control API** using the Go standard library router
- **Prometheus text metrics** without a runtime dependency
- **Structured JSON logs** via `log/slog`
- **Health/readiness endpoints**
- **Graceful shutdown and HTTP server timeouts**
- **Race-detector test suite**
- **CodeQL + govulncheck + Dependabot** security automation
- **Rootless scratch container**

> Current boundary: v0.1 state is in-memory and process-local. It demonstrates the coordination semantics, API contracts and failure behavior. Replicated/durable state is a roadmap milestone, not a claim hidden behind the word “distributed”.

## Install and run

### Prebuilt binaries

`v0.1.0` ships signed-by-GitHub release metadata plus SHA-256 checksums for:

- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64
- Windows amd64

Download from the [v0.1.0 release](https://github.com/mkarson1997/karzoun-atlasmesh/releases/tag/v0.1.0) and verify the artifact against `SHA256SUMS.txt` before use.

### Container

```bash
docker pull ghcr.io/mkarson1997/karzoun-atlasmesh:0.1.0
docker run --rm -p 8080:8080 ghcr.io/mkarson1997/karzoun-atlasmesh:0.1.0
```

The image uses a multi-stage build and a non-root `scratch` runtime.

### From source

Requires Go 1.26+; CI validates the supported toolchains.

```bash
go run ./cmd/atlasmesh server --listen :8080
```

Register a service:

```bash
curl -X POST http://localhost:8080/v1/services/register \
  -H 'content-type: application/json' \
  -d '{"service":"payments","id":"payments-a","address":"http://10.0.0.12:9000","weight":2,"ttl_seconds":30}'
```

Resolve a healthy instance:

```bash
curl http://localhost:8080/v1/services/payments/resolve
```

Create and claim work:

```bash
curl -X POST http://localhost:8080/v1/jobs \
  -H 'content-type: application/json' \
  -d '{"id":"job-001","type":"invoice.generate","max_attempts":3,"payload":{"invoice_id":"INV-42"}}'

curl -X POST http://localhost:8080/v1/jobs/claim \
  -H 'content-type: application/json' \
  -d '{"worker":"worker-a","lease_seconds":30}'
```

Operational endpoints:

```text
GET /healthz
GET /readyz
GET /metrics
GET /v1/cluster/status
```

## API surface

| Method | Path | Purpose |
|---|---|---|
| POST | `/v1/services/register` | Register/replace a leased service instance |
| POST | `/v1/services/{service}/{id}/heartbeat` | Renew service lease |
| POST | `/v1/services/{service}/{id}/health` | Update health state |
| GET | `/v1/services` | List live service instances |
| GET | `/v1/services/{service}/resolve` | Weighted healthy resolution |
| POST | `/v1/jobs` | Enqueue unique-by-ID logical work |
| POST | `/v1/jobs/claim` | Acquire work with an expiring lease |
| POST | `/v1/jobs/{id}/heartbeat` | Extend active worker lease |
| POST | `/v1/jobs/{id}/complete` | Complete only with current ownership |
| POST | `/v1/jobs/{id}/fail` | Retry or dead-letter work |
| POST | `/v1/nodes/register` | Register cluster member |
| POST | `/v1/nodes/{id}/heartbeat` | Renew node lease |
| GET | `/v1/cluster/status` | Operational cluster snapshot |

## Engineering guarantees

**Lease fencing:** a worker whose lease expired cannot complete or mutate a job after another worker acquires it.

**At-least-once boundary:** work may be delivered more than once after crashes or lease expiry. External side effects must therefore use idempotency keys.

**Deterministic resolution:** weighted selection is stable for the same healthy set, which makes behavior straightforward to test and reason about.

**Bounded inputs:** JSON request bodies, job payloads/results, TTLs, retries and lease durations are constrained.

## Architecture

```text
               ┌────────────────────────┐
               │      AtlasMesh API     │
               └────────────┬───────────┘
                            │
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                 ▼
   Service Registry      Job Queue      Node Membership
    TTL + Health       Lease Fencing      Heartbeats
          │                 │                 │
          └─────────────────┼─────────────────┘
                            ▼
                 In-memory State (v0.1)
                            │
                            ▼
             Replicated/Durable Store (roadmap)
```

See [`docs/architecture.md`](docs/architecture.md) for invariants and failure semantics.

## Development

```bash
make ci
make vuln
```

The repository is intentionally dependency-light. Core runtime behavior uses only the Go standard library; `govulncheck` is pinned as a CI/developer tool.

## Release engineering

Version tags drive the release workflow. A `v*` tag builds cross-platform binaries, produces SHA-256 checksums, publishes a GitHub Release, and pushes matching version + `latest` images to GitHub Container Registry.

## Security

Report vulnerabilities privately according to [`SECURITY.md`](SECURITY.md). Never include credentials, production tokens or private infrastructure details in a public issue.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
