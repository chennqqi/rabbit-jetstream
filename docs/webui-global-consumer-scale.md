# Global Consumer index scale prototype

[English](webui-global-consumer-scale.md) | [简体中文](webui-global-consumer-scale.zh-CN.md)

This record measures the in-process collection/join/query algorithm at the agreed 10,000 Stream / 100,000 Consumer ceiling. It does **not** include NATS request latency, browser rendering, concurrent production traffic or a multi-run p95 distribution, so it does not qualify the end-to-end latency requirement.

Environment: Windows 11 build 26100, AMD64, Intel Core Ultra 7 155H, 22 logical benchmark threads, Go 1.25.13. The host memory query was denied by local WMI permissions and is intentionally not guessed.

Reproduce from the repository root:

```powershell
$env:GOCACHE = "$PWD\.tmp\gocache"
go test ./management/internal/api -run '^$' -bench 'Benchmark(Collect|Query)GlobalConsumers100k$' -benchtime=1x -benchmem -count=3
```

Initial results:

| Operation | Rows | Wall time per run | Allocated bytes | Allocations |
| --- | ---: | ---: | ---: | ---: |
| Complete generation collection/join | 100,000 | 149.55–184.85 ms | 258.01–258.08 MB | 1,510,391–1,510,595 |
| Filter `consumer-09` to 10,000 rows, before optimization | 100,000 scanned | 10.50–10.93 ms | 10.45 MB | 100,022 |
| Same query after removing per-row concatenation | 100,000 scanned | 7.15–12.11 ms across eight one-shot samples | 5.65 MB | 22 |

The query optimization removes 100,000 temporary haystack allocations without changing case-insensitive literal matching or deterministic identity ordering. The remaining query allocation includes the 10,000-row result slice. Collection memory includes generated Consumer projections, join maps, exact encoded-row budget accounting and the benchmark source's per-Stream response allocations.

The implementation exercises the algorithmic scale ceiling in this prototype; one-shot timing is not a statistically valid p95. End-to-end acceptance still requires a real broker dataset on stated hardware, repeated collection and query samples, management RSS/GC observations, broker API request cost, UI first-usable timing and p95 calculation. Windows race testing was attempted but is unavailable because this workstation has neither CGO enabled by default nor a C compiler; Linux CI race remains required.
