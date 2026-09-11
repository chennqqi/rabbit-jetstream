# Qualification sampling repair

[English](qualification-sampling-repair.md) | [简体中文](qualification-sampling-repair.zh-CN.md)

The September 9–10, 2026 bare-metal run completed 24 hours and delivered all
432,000,001 messages without loss, duplication, corruption or retries. Publish
P99 was 2.36 ms. Its final qualification status is nevertheless **failed**:
each node had 8,628 resource samples, below the unchanged minimum of 8,639.
Original evidence and the failure record must remain untouched.

The sampler previously slept ten seconds *after* collecting all three nodes.
Collection time accumulated: mean observed interval 10.017672 seconds, maximum
10.747425 seconds. Samples covered the complete window and recorded unchanged
node identities, but did not satisfy the count gate.

The repair schedules samples against the initial monotonic time reference.
Collection overhead is subtracted from the wait. Missed slots are skipped,
not backfilled; timestamps always reflect actual collection. Existing count,
maximum-gap, duration, identity and window-alignment gates remain unchanged.
Failure diagnostics now include counts and the required minimum.

Tests simulate 24 hours plus 20 seconds of sampling with collection overhead,
overruns and collection errors. Verifier regression tests continue to reject
the observed old drift, missing slots and long gaps. Tests and builds run only
on the local workstation; the remote host receives frozen artifacts only.

A new run needs a fresh state directory and a full new 24-hour window. It tests
the original runtime revision `2872e4819f5da873c752a1e06cf186d459a9b594` with
separately frozen repaired verification tools. It does not qualify uncommitted
WebUI or management changes. Those changes are preserved outside the isolated
local build worktree. Use the existing bounded bare-metal launcher without
changing its resource limits or the host's existing services.

## Restart record — 2026-09-11

- Repair commit: `aea1c2077c6e`. Local bundle archive SHA-256:
  `52fb4a8961a9479b315e6cadc28b9da812cea7ef79155405d95bc91937e5befa`.
- Local full Go tests and vet passed; Linux verification-tool race tests passed.
  Coverage: 80.5% overall, critical packages 92.3% and 96.1%. An initial Windows
  fixture run safely rejected an unavailable temporary port; the full rerun passed.
- New two-minute calibration and resource audit passed: 600,001 messages,
  zero loss/duplicates/corruption/retries, approximately 5,000 messages/s,
  publish P99 2.12 ms.
- New unit: `rjs-qual-rc2-20260911-soak01.service`. Workload started at
  **2026-09-11 15:28:52 Asia/Shanghai**; expected workload finish is
  **2026-09-12 15:28:52**, followed by final sampling and audit.
- Initial checks show the service running and samples increasing. This is
  **in progress, not passed**. Raw evidence stays on the qualification host.

```bash
ssh lsb112 'systemctl status rjs-qual-rc2-20260911-soak01.service --no-pager'
```
