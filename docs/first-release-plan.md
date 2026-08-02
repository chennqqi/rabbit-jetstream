# First Release Plan

The first formal release is a native JetStream queue product, not an AMQP compatibility release. Its supported application path is `rabbit-jetstream-go`; unchanged RabbitMQ clients and AMQP 0-9-1 wire compatibility remain future Roadmap work.

## Version Set

- Server, management, operator, chart: `v0.1.0-rc.1` before the final `v0.1.0` tag.
- Native Go SDK: `v0.1.0-rc.1` before the final `v0.1.0` tag.
- Resource and message contract: `v1alpha1`.
- NATS Server subtree and local SDK integration baseline: `v2.14.1`.

An RC is valid only for an exact server commit and SDK commit recorded together. A passing test from either repository alone is insufficient.

## Local RC Gates

Before requesting a release candidate:

1. Both repositories are clean and all unit, race, vet, coverage, contract, Docker integration, fault, upgrade/rollback, backup/restore, Helm and security checks pass.
2. Priority publishing and bounded strict-priority consumption pass for every configured priority, including fairness, reconnect, duplicate publish, redelivery, slow consumers, backlog recovery and DLQ priority preservation.
3. The SDK public API, example, changelog and compatibility limitations are frozen. The server compatibility matrix remains `partial` until the SDK and server are actually versioned releases.
4. Linux container images and Helm artifacts are built from the recorded server commit. Generated manifests include component versions, image IDs and SHA-256 hashes.
5. Release notes list unsupported RabbitMQ behavior explicitly; AMQP compatibility is not a release blocker and must not be advertised.

Run `make test-local-rc` for the paired quick gate. `-Mode Full` adds SDK integration and the complete Docker Desktop scenario matrix. `-Mode Release` additionally runs coverage, backup/restore, rolling upgrade/rollback, Helm, security and the three-replica CI performance workload. Every mode writes `artifacts/local-rc.json`, binding results to the exact server and SDK commits; Release evidence also hashes its performance report. `-AllowDirty` is development-only and invalid for release evidence.

GitHub Actions and cross-repository automation are deliberately deferred. Local scripts are the source of truth while the release process is being refined collaboratively.

## Production Evidence

Docker Desktop is suitable for development and Linux-container functional gates. Final production approval additionally requires the existing 24-hour three-node soak on a dedicated native Linux host matching production, followed by `make verify-soak`. WSL and Docker Desktop do not satisfy this evidence requirement.

After the soak, perform a staged canary with rollback rehearsed: deploy by immutable digest, validate publish/consume/DLQ and observability, introduce one node failure, then expand traffic only while error rate, redelivery, backlog, latency and storage remain within the documented thresholds.

## Exit Criteria

Tag `v0.1.0` only when the paired commits, local gate output, native-Linux soak evidence, upgrade/rollback evidence, backup/restore evidence, known limitations and operator runbook have been reviewed together. Remote CI, signing, attestations and registry publication are the final delivery phase, not a substitute for these gates.
