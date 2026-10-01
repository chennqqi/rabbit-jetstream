# Release License Records

[English](release-license-records.md) | [简体中文](release-license-records.zh-CN.md)

The pinned NATS Server subtree is distributed under its included Apache-2.0 `LICENSE`, copied into the release bundle as `NATS-LICENSE`. Container base images and third-party modules retain their respective licenses; image SBOM attestations enumerate their packages for review.

The server management code and Native SDK repositories currently have no top-level license grant. This record does not assign one. The sole owner must decide redistribution terms before external distribution and review third-party notice obligations against the generated SBOMs. Local qualification and private release-candidate packaging do not imply permission for public redistribution.

BuildKit SBOM/provenance attestations are embedded in the OCI archives. They are local build records, not an identity-signed public publisher attestation. Registry publication and identity signing remain separate release actions.
