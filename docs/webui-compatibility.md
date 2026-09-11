# Compatibility console

[English](webui-compatibility.md) | [简体中文](webui-compatibility.zh-CN.md)

The candidate `/admin/compatibility` page implements a read-only WEB-034 workspace for authenticated operators and auditors. It combines independent management build, published native SDK contract and existing server capability panels. Failures clear the affected metadata without presenting it as current or discarding successful sibling sources. Navigation cancellation suppresses late responses. No upgrade, restart, feature toggle or message operation is offered.

## Build API

`GET /api/v1/console/build` requires operator/auditor authorization even in local-demo mode. It rejects query parameters and does not call the Broker. Success is `Cache-Control: no-store`:

```json
{"schemaVersion":"rjs.build-info.v1","version":"v0.1.0-rc.2","goVersion":"go1.25.0","os":"linux","arch":"amd64","revision":"…40 lowercase hexadecimal characters…","revisionSource":"release-build","modified":false,"uiAssets":{"algorithm":"sha256-framed-files-v1","digest":"…64 lowercase hexadecimal characters…","fileCount":4}}
```

This illustrative response describes the management executable, not all Broker nodes. `uiAssets` identifies the exact closed file set embedded in and served by that process. Its SHA-256 construction frames every sorted relative path and byte length, so path/content boundaries are unambiguous; it is neither a signature nor qualification evidence. Release packaging now injects the exact clean server revision even though those binaries intentionally use `-buildvcs=false`; `revisionSource=release-build` requires both a valid 40-character revision and the clean-build marker set by the clean revision-bound pipeline. A revision injected without that marker is reported as `injected` and does not claim `modified=false`. Ordinary builds may use `go-build-info`. Absence remains unreported, not clean-build proof. Complete Go settings, linker flags, local paths and dependency lists are excluded.

## SDK and capabilities

The page consumes the existing public `/api/v1/native-sdk-contract.json`; its existing cache policy is unchanged. It presents the schema identifier, declared availability, delivery guarantee, publisher confirmation, acknowledgment policy and required/optional message header names. It does not infer installed-client compatibility or a released SDK version. In particular, `native-sdk-implemented-unreleased` remains visible rather than becoming a green “released” badge. The native contract is not RabbitMQ wire-protocol compatibility.

The reused capabilities panel separately reports parser-supported values, deployment intent, authoring schema and release-manifest status. Sources are not an atomic snapshot. Per-node versions remain on node detail pages. When `RJS_RELEASE_MANIFEST` is configured, startup binds its version, server revision and WebUI identity to this clean release build; the page displays only the exact manifest statement and file SHA-256 as `reported`. It never rewrites that statement as `qualified`.

Local release bundles record the same identity under `webui` in `release-manifest.json`, using the shared Go implementation rather than a second packaging-script hash algorithm. Bare-metal derivative packaging rejects a missing/invalid identity, carries it forward unchanged and automatically supplies the frozen `runtime-manifest.json` to its management child. Missing, oversized, malformed, incomplete or mismatched configured manifests stop application initialization. This is exact binding, not signature verification, approval-evidence validation or an independent certification of the manifest statement.

## Verification

Go tests cover authenticated operator/auditor access without a working Broker, anonymous denial, rejected queries, cache policy, clean versus ordinary revision injection, deterministic path/content-sensitive asset hashing and exclusion of sensitive build settings. Contract tests require the local package, management Dockerfile and GitHub Release workflow to inject both revision and clean-build marker. Frontend tests cover supported/malformed revision sources and asset identities, older-server omission, unknown VCS status, retained unreleased SDK stage, independent failures, read-only transport and canceled late responses. Embedded Chromium `artifacts/webui-live-nFrxil/report.json` passed 131 checks and 24 accessibility snapshots using a `-buildvcs=false` binary with the release-style revision/clean marker; it also independently matched every served UI file to the API identity. The promoted UI identity is `1d044d84576088a1f6e7b6811b18c6f52569daa0fafc138823b0ed9a316043c7`. These tests do not establish a signature or release qualification.
