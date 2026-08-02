# Release Process

A production release is a two-stage process because GitHub Actions jobs cannot run the required 24-hour soak.

1. On a clean checkout of the release commit, run `make test-soak` on a dedicated native Linux host whose kernel, storage, limits, and architecture match production. Docker Desktop and WSL evidence is rejected.
2. Re-run `make verify-soak`. The verifier binds evidence to the current 40-character Git revision and independently checks report timestamps, native Linux provenance, the immutable image ID, continuous three-node samples and artifact hashes. Create the final `vX.Y.Z` tag at that exact `source_revision`, create a draft GitHub release for it, and upload the four `performance-soak.json*` files. Keep the baseline and resource samples even though only the evidence manifest is copied into the compact release bundle.
3. From that tag, manually run the `Release` workflow and provide the draft evidence-release tag.
4. The workflow reruns race/coverage/security, Helm, Linux standalone and node-failure, every management black-box scenario, rolling upgrade, backup/restore, and migration gates on the tag. It then revalidates soak hashes, duration, integrity, sampling, baseline regression, native-Linux provenance, and source revision before publishing anything.

The workflow publishes separate NATS, management, and operator images for `linux/amd64` and `linux/arm64` to GHCR. Images carry BuildKit provenance and SPDX SBOM attestations, plus GitHub artifact attestations. It also packages the Helm chart, writes all three repositories and digests to `release-images.yaml`, generates `SHA256SUMS`, attests the files, and uploads them to the GitHub release. Use the operator digest from that file for every `rjsctl` backup, restore, diagnostic, migration, and administration run.

Verify consumers before deployment:

```bash
sha256sum -c SHA256SUMS
gh attestation verify rabbit-jetstream-0.1.0.tgz -R OWNER/rabbit-jetstream
gh attestation verify oci://ghcr.io/OWNER/rabbit-jetstream-nats:v0.1.0 -R OWNER/rabbit-jetstream
gh attestation verify oci://ghcr.io/OWNER/rabbit-jetstream-operator:v0.1.0 -R OWNER/rabbit-jetstream
```

Do not publish mutable `latest` tags. Promotion means deploying the digests in `release-images.yaml`, then retaining the CI, soak, fault, backup/restore, rolling-upgrade, vulnerability-scan, SBOM, and attestation evidence for the release lifetime.
