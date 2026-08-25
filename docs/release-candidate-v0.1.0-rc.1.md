# v0.1.0-rc.1 Release Candidate Checklist

This checklist records release state; machine-readable evidence remains authoritative.

## Completed

- [x] Native Linux/AMD64 qualification: 3 nodes, 3 replicas, priorities `0..7`.
- [x] Server and Native SDK 24-hour 5,000 msg/s integrity soaks.
- [x] 3/5/8-level capacity matrix and 16 KiB/256 KiB payload checks.
- [x] Single-node outage, network isolation and rolling-restart validation.
- [x] Management API, Admin UI, authentication, audit, backup/restore, Helm and operational documentation.
- [x] Explicit AMQP, ARM64, at-least-once and maximum-priority boundaries.
- [x] Local Quick gate for the paired repositories; output is written to ignored `artifacts/local-rc*.json`.

## Required Before Publishing the RC

- [ ] Run the clean `Release` local gate with Docker Desktop available: `make test-local-release`.
- [ ] Build the frozen bundle: `make package-local-rc` and verify `SHA256SUMS`.
- [ ] Review vulnerability output, SBOMs, image digests and license inventory.
- [ ] Record the exact server and SDK revisions in the release notes.
- [ ] Create and push the annotated `v0.1.0-rc.1` tags only after review.

## Required Before v0.1.0 GA

- [ ] Complete the staged 1%/10%/25%/50%/100% canary on a reversible workload.
- [ ] Export canary, node-failure and rollback evidence and obtain service-owner, application-owner and on-call sign-offs.
- [ ] Run `make verify-release-approval` against the completed approval document.

The expensive qualification host is no longer required for RC packaging. A production-like environment is required for the application canary, not another synthetic 24-hour soak unless the frozen runtime code or dependencies change.
