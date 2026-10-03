# Prototype Instructions

## Project decisions

- Visual truth: ../../docs/design/webui/queue-detail-selected.png, 1487 x 1058, selected result exec-f508c114-b351-4751-aac4-b6f9f2d6627c.
- Preserve its primary/supporting two-column detail layout and complete data. Compare design variants on the same task and dataset.
- Synthetic-data prototype only: no production API, credentials, remote execution, or admin-ui/dist changes.
- The owner explicitly authorized local headless Playwright for this isolated prototype.
- Logic review is a separate gate from visual QA: Consumer collection/filter/detail/return must be complete; identify metric ownership; separate desired declarations from observed resources. Never imply a mock pass qualifies real authorization, concurrency or audit.

Run the local server yourself and open the preview in the browser available to this environment. Do not give the user server-start instructions when you can run it.

Before making substantial visual changes, use the Product Design plugin's `get-context` skill when the visual source is unclear or no longer matches the current goal. When the user gives durable prototype-specific design feedback, preferences, or decisions, record them in `AGENTS.md`.

When implementing from a selected generated mock, treat that image as the source of truth for layout, component anatomy, density, spacing, color, typography, visible content, and hierarchy.

Build app UI in `src/`. Keep `.openai/hosting.json`, `worker/index.js`, `scripts/prepare-sites-build.mjs`, and `tests/sites-worker.test.mjs` intact so the same local prototype can be handed to Sites. Before a Sites handoff, run `npm run build` and `npm run test:sites`; the build must leave `dist/client/index.html`, `dist/server/index.js`, and `dist/.openai/hosting.json`.
