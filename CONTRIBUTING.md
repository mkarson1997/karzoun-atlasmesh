# Contributing

Thanks for improving AtlasMesh.

## Workflow

1. Open or reference an issue for non-trivial behavior changes.
2. Create a focused branch.
3. Add or update tests for coordination semantics.
4. Run `make ci` and `make vuln`.
5. Open a pull request and describe failure semantics, not only the happy path.

## Engineering bar

Changes to leases, retries, health or scheduling must document concurrency behavior and stale-owner behavior. Avoid hidden network calls in core packages. Prefer the standard library unless a dependency materially improves correctness or interoperability.
