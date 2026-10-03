# Selected WebUI design and implementation handoff

[English](webui-selected-design.md) | [简体中文](webui-selected-design.zh-CN.md)

## Selection record

Date: 2026-09-09. The owner explicitly selected image `exec-f508c114-b351-4751-aac4-b6f9f2d6627c` from the second, same-data comparison. This selects the **Queue detail primary/supporting two-column layout**, not all P0 contracts or every page design.

- Workspace source: [queue-detail-selected.png](design/webui/queue-detail-selected.png).
- Original pixels: **1487 × 1058**. Although the prompt requested 1440 × 1024, the generated file has different dimensions. QA must use the actual dimensions or explicitly normalize both images without distortion.
- SHA256: `91E82CD2EF7BDCBADEEF899053C9C06257C14C6C06243D8442A2DCCEEAFCA4EA`.
- Produced with built-in ImageGen, not the fallback CLI/API. The exact selected output was copied unchanged into the repository; the original was retained.
- Prompt brief: same Queue/data/actions/tabs, primary evidence column about 68%, supporting configuration column about 32%, existing dark-green/light visual language, no charts or message operations.

## Fixed visual structure

Preserve the source's RJS lettering, dark navigation, white top bar, resource breadcrumb, title/revision/freshness, refresh/edit actions, amber degradation notice and horizontal tabs. Keep the primary evidence area on the left and configuration on the right. The left area contains the three metrics and complete replica table; the right has six configuration pairs. Keep the diagnostic action below both columns.

In the actual image the sidebar is approximately 250 pixels, header 70 pixels (corrected during screenshot comparison), content starts near x=283, and the two content areas are approximately 780/382 pixels with a 16-pixel gap. These are source-image estimates, not immutable responsive CSS values. On narrow screens, stack evidence before configuration while retaining all fields and access controls.

Fixed comparison fixture: `orders_events`, declaration revision `12`, observation `2026-09-09 16:20:00 Asia/Shanghai`, degraded `demo-cluster` with three-node/R3 intent; stored `12,480`, pending `8,420`, ack-pending `240`; replicas `nats-1` Leader/current/0, `nats-2` Follower/current/0, `nats-3` Follower/offline/unknown. Configuration: `RJSQ_orders_events`, `file`, `3`, `24h`, `30s`, `5`. This is synthetic data, never evidence of real service state.

## First implementation slice

Create a self-contained **frontend-only interactive prototype** of this selected detail view, separate from `admin-ui/dist/` and release binaries. Use the selected image as visual truth, not as a full-page raster background. Navigation context, five tabs, refresh and edit/review should be interactive with clearly labeled synthetic data. Do not connect this prototype to production APIs, save credentials, implement real mutations or infer that all P0 functionality has been approved. Unsupported surrounding navigation must explain its prototype scope rather than silently do nothing.

Asset inventory: no photographs, illustrations or custom raster assets beyond the archived reference; RJS is text branding. Navigation/action/status symbols should come from a suitable licensed icon library, not handmade approximations. Keep system-font fallbacks and existing palette. Do not change the production frontend toolchain merely to run a design prototype.

Before real API integration, retain C-01–07 and safety gates from the [design plan](webui-design-plan.md). In particular, correct generated explanatory copy: metrics are different observations and need not be disjoint or additive; replica Lag must follow the actual API meaning rather than automatically being called business-message count. These are semantic corrections, not permission to change the selected layout or remove data.

## Verification status and next gate

Selection and source archival are complete. The owner subsequently explicitly authorized local headless Playwright. An isolated [interactive prototype](../design-prototypes/webui/README.md) now runs on loopback port 18224 and its [design QA](../design-prototypes/webui/design-qa.md) passed after corrections: 13 browser checks, six accessibility scan states with zero violations, three unit tests and four packaging tests. This qualifies only the synthetic prototype, not production APIs, the whole P0 scope, or a release candidate.

Next gate: owner review of the interactive detail experience, followed by the remaining page designs and C-01–07 contracts before production integration. Keep browser permission scoped to the isolated local prototype, not production services or the remote soak.
