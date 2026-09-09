# Bare-metal qualification

[English](baremetal-qualification.md) | [简体中文](baremetal-qualification.zh-CN.md)

This is a native Linux/AMD64, single-physical-host stability profile, not a container profile or proof of multi-host availability. Three authenticated JetStream processes use R3 storage. The management process is also monitored. Kubernetes qualification and application Canary approval remain separate gates.

## Build and provenance

Build **only on the local workstation**. Never transfer source, compile, install packages or run containers on the qualification host. `scripts/release/package-baremetal.ps1` verifies the frozen runtime bundle, copies its management/CLI binaries, extracts the identical NATS executable locally from its frozen AMD64 OCI layers, and builds verification helpers locally using the pinned Go image with networking disabled.

Commit the verification changes before packaging. Runtime revision and verification-tool revision are separate manifest fields: tooling changes do not relabel the existing runtime regression evidence. All deployed files are covered by `SHA256SUMS`; the original runtime manifest and NATS OCI descriptors are retained. The new native preflight identifies `deployment_mode: bare-metal`, binary SHA-256, manifest SHA-256, filesystem and unprivileged UID. It does not fabricate Docker evidence.

```powershell
./scripts/release/package-baremetal.ps1 -RuntimeBundle dist/v0.1.0-rc.2
```

Transfer only the resulting archive into a **new** root-owned directory under `/opt/rjs-qualification/`. Verify the archive SHA-256 before extraction, then its `SHA256SUMS`. Directories must be traversable and executables readable/executable by the dynamic service account; none may be writable by non-root users. Never replace an existing bundle or state directory.

## Safety and execution

The supplied `scripts/baremetal-systemd.sh` uses root only to launch a transient service. Work runs as a systemd `DynamicUser`; no persistent account, enabled unit, existing service, firewall, kernel setting or existing application data is changed. The service only writes to its new `StateDirectory` and private temporary directory. Network access is restricted to loopback; all ten ports bind `127.0.0.1`. Passwords are random and retained only in private run files/environment.

The service shares a bounded cgroup: CPU quota 400% (four logical CPUs), memory high/max 3/4 GiB, no swap, 128 tasks, low CPU/IO priority, device read/write limits 40/20 MiB/s, 256 MiB per-file limit, and a 26-hour hard runtime limit. Each NATS node allows at most 128 MiB memory and 2 GiB file storage. Guards stop **only this test** if available host memory falls below 16 GiB, free disk below 30 GiB, readiness fails, a node changes identity, or a child exits. No automatic restart is configured. Resource limits reduce interference risk; they do not promise zero performance impact on a shared host.

First launch `calibration` (2 minutes, ports 24220–24229) using a unique run ID. Inspect completion, resource summary and effective systemd limits. Only after it passes launch `soak` with another new run ID (24 hours, ports 24240–24249):

```bash
bash /opt/rjs-qualification/BUNDLE/scripts/baremetal-systemd.sh /opt/rjs-qualification/BUNDLE UNIQUE-ID RUNTIME-40-CHAR-REVISION calibration
# After successful calibration, use a different UNIQUE-ID:
bash /opt/rjs-qualification/BUNDLE/scripts/baremetal-systemd.sh /opt/rjs-qualification/BUNDLE UNIQUE-ID RUNTIME-40-CHAR-REVISION soak
systemctl status rjs-qual-UNIQUE-ID.service
# Only if you need to stop this particular run:
systemctl stop rjs-qual-UNIQUE-ID.service
```

Do not kill processes by name or delete state directories. Failed run evidence is retained. A fresh run requires fresh state and restarts the full observation window.

## Acceptance

The fixed workload is 5,000 messages/s, 1,024-byte payloads, eight publishers, batch 256 and R3. Require continuous publishing for at least 24 hours, complete resource samples every 10 seconds, no node restart, zero missing/corrupt/duplicate messages or publish/consume retries, publish and consume throughput at least 4,900 messages/s, and publish P99 no more than 10 ms. Calibration uses the same rate and integrity/performance thresholds but is never accepted as 24-hour evidence.

`/var/lib/rjs-qual-UNIQUE-ID/run/started.json` records the start and estimated end. `completion.json` must say `completed`; a healthy initial state alone is not a pass. Completion produces `report.json`, `resources.ndjson`, `resource-summary.json`, and a verified `soak-evidence.json`. Copy these and `preflight.json`/`command.json` back, then independently run `perfevidence -require-soak -source-revision RUNTIME-REVISION -evidence soak-evidence.json`. Preserve systemd limits/status alongside them.

This inaugural bare-metal profile uses the explicit absolute thresholds above. Prior container/other-host baselines remain valid for their original configurations and are not presented as comparable performance baselines. A later comparison must use the same hardware, resource limits and workload profile. Successful completion does not authorize GA publication or substitute for the sole owner's application Canary decision.
