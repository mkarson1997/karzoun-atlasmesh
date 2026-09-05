# Roadmap

## v0.1 — Control-plane semantics

- [x] Service registry with TTL leases
- [x] Health-aware weighted resolution
- [x] Cluster member heartbeats
- [x] Job claims with expiring leases
- [x] Stale-worker fencing
- [x] Retry and dead-letter states
- [x] REST API, metrics and structured logs
- [x] Race-detector CI, CodeQL and govulncheck

## v0.2 — Durable coordination

- [ ] Storage interfaces for registry, jobs and membership
- [ ] PostgreSQL transactional adapter
- [ ] Atomic `SKIP LOCKED` job claims
- [ ] Durable idempotency/fencing token
- [ ] Migration versioning and recovery tests

## v0.3 — Distributed control plane

- [ ] Multi-node control-plane coordination
- [ ] Leader election for singleton schedulers
- [ ] Watch/stream API for registry changes
- [ ] Failure detector tuning and jitter
- [ ] Zone-aware resolution policies

## v0.4 — Production operations

- [ ] OpenTelemetry traces and histograms
- [ ] Authentication and scoped authorization
- [ ] Rate limits and audit events
- [ ] Kubernetes manifests and Helm chart
- [ ] Chaos/fault-injection test harness
- [ ] Load and latency benchmark suite
