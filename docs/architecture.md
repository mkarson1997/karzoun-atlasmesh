# AtlasMesh architecture

## Design objective

AtlasMesh separates **coordination semantics** from **storage implementation**. v0.1 keeps state in memory so the project can prove lease, retry, health and resolution invariants with deterministic tests before replication is introduced.

## Core invariants

### Service registry

1. A service instance is addressable by `(service, instance_id)`.
2. Registration has a bounded TTL.
3. Expired instances are removed before reads or mutations.
4. Resolution considers only live and healthy instances.
5. Weight changes selection frequency, never health eligibility.

### Job ownership

1. A job has one active lease owner at a time inside a queue instance.
2. Every acquisition increments `attempt`.
3. Lease expiry makes the previous owner stale immediately.
4. A stale owner cannot heartbeat, complete or fail the job.
5. Expiry on the final allowed attempt dead-letters the job.
6. At-least-once delivery means handlers must make external side effects idempotent.

### Cluster membership

Node membership is lease-based. Heartbeats extend liveness; missing heartbeats remove a member from the live view. Membership in v0.1 is observational rather than consensus-backed.

## Failure semantics

| Failure | v0.1 behavior |
|---|---|
| Service stops heartbeating | Service lease expires and resolver excludes it |
| Worker dies after claim | Lease expires; work becomes claimable if attempts remain |
| Old worker returns late | Fencing rejects completion with `ErrLeaseLost` |
| Final worker lease expires | Job moves to dead-letter state |
| Process restarts | In-memory state is lost; durable state is a planned milestone |

## Planned persistence boundary

The registry, membership and queue APIs intentionally expose behavior that can later be backed by PostgreSQL, etcd or a consensus log. The storage milestone must preserve atomic claim/fencing semantics rather than merely serialize Go maps to disk.

## Observability

The HTTP layer emits structured request logs and Prometheus counters. Payload bodies, job results, credentials and arbitrary metadata are not emitted into operational metrics by default.
