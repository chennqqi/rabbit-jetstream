# Release Process

The scope, paired server/SDK version set and release exit criteria are defined in [First Release Plan](first-release-plan.md). Native-Linux qualification and staged promotion follow the [Production Readiness and Canary Runbook](production-readiness.md). During current development, run and refine the local gates first; cross-repository GitHub Actions are intentionally deferred until the local process is stable.

A production release is a two-stage process because GitHub Actions jobs cannot run the required 24-hour soak.

1. Copy only the frozen bundle to a dedicated native Linux amd64 host whose kernel, storage and limits match production. As an unprivileged user, run its bundled `bin/linux-amd64/nativequal` with the frozen server revision and retain `native-linux-preflight.json`. Do not copy the source tree. Docker Desktop and WSL evidence is rejected; arm64 execution is deferred until a native host exists.
2. Re-run `make verify-soak`. The verifier binds evidence to the current 40-character Git revision and independently checks report timestamps, native Linux provenance, the immutable image ID, continuous three-node samples and artifact hashes. Retain the four `performance-soak.json*` files and the baseline; do not create the final tag yet.
3. Complete the staged canary, node-failure and rollback rehearsal, fill `docs/release-approval.template.json`, obtain the three required sign-offs after the final observation window, and run `make verify-release-approval`.
4. Only after approval passes, create the final `vX.Y.Z` tag at the exact approved `source_revision`, create a draft GitHub release, upload the preflight, soak and approval evidence, then manually run the `Release` workflow and provide the draft release tag.
5. The workflow reruns race/coverage/security, Helm, Linux standalone and node-failure, every management black-box scenario, rolling upgrade, backup/restore, and migration gates on the tag. It then revalidates soak hashes, duration, integrity, sampling, baseline regression, native-Linux provenance, and source revision before publishing anything.

The workflow publishes separate NATS, management, and operator images for `linux/amd64` and `linux/arm64` to GHCR. Images carry BuildKit provenance and SPDX SBOM attestations, plus GitHub artifact attestations. It also packages the Helm chart, writes all three repositories and digests to `release-images.yaml`, generates `SHA256SUMS`, attests the files, and uploads them to the GitHub release. Use the operator digest from that file for every `rjsctl` backup, restore, diagnostic, migration, and administration run.

Verify consumers before deployment:

```bash
sha256sum -c SHA256SUMS
gh attestation verify rabbit-jetstream-0.1.0.tgz -R OWNER/rabbit-jetstream
gh attestation verify oci://ghcr.io/OWNER/rabbit-jetstream-nats:v0.1.0 -R OWNER/rabbit-jetstream
gh attestation verify oci://ghcr.io/OWNER/rabbit-jetstream-operator:v0.1.0 -R OWNER/rabbit-jetstream
```

Do not publish mutable `latest` tags. Promotion means deploying the digests in `release-images.yaml`, then retaining the CI, soak, fault, backup/restore, rolling-upgrade, vulnerability-scan, SBOM, and attestation evidence for the release lifetime.
