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
