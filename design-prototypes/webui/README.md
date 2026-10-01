# Queue detail design prototype

[English](README.md) | [简体中文](README.zh-CN.md)

Local preview: [http://127.0.0.1:18224/](http://127.0.0.1:18224/).

Implements the [selected two-column design](../../docs/webui-selected-design.md) with synthetic Queue/Stream/Consumer data. Five tabs, keyboard navigation, mock refresh, bounded configuration validation, review/apply, discard confirmation, in-memory event history, Chinese/English switching and responsive navigation work. The Queue breadcrumb opens a one-row fixture list; other sidebar destinations explicitly explain their unsupported prototype scope.

**Not the production WebUI.** No NATS connection, management API, real identity, credentials, durable storage or actual queue mutation. Changes disappear on reload. Scope-limited editing uses a short modal for three fields; the planned full-page production editor remains future work. A successful mock Refresh observes the accepted mock configuration; failed reads retain previous values and timestamps.

## Logic-review iteration

Latest mutation-recovery iteration: expand **Mock write scenarios** inside the editor to exercise conflict, forbidden, expired, unknown, partial and missing-audit outcomes. Recovery requires explicit review; unknown/partial operations remain accessible after closing and cannot be blindly resubmitted. [Design, evidence and limits](../../docs/webui-mutation-design.md). Latest totals: **26 browser checks, 15 model tests, 4 packaging tests, 15 accessibility states with zero automated violations**; these supersede earlier counts below.

Consumer collection now precedes detail: filter names/Subjects/mode across the entire fixture, page through results, open a named record and return with context retained. Expand Mock scenarios for standard/priority-0/priority-0–7 and failed/forbidden/missing reads. The five-row prototype page size is for demonstrating pagination, not the production 50-row default. Safe route state survives reload; mock edits do not. Overview metrics identify their Consumer; desired and observed values are separate; equivalent duration values do not trigger writes. [Open production gates](../../docs/webui-logic-review.md).

Latest verification supersedes earlier counts below: 19 browser checks, 8 unit tests, 4 packaging tests, 11 accessibility states with zero automated violations. No production acceptance is implied.

## Local development and checks

From this directory:

```powershell
npm.cmd ci --prefer-offline --no-audit --no-fund
npm.cmd run build
npm.cmd run preview -- --host 127.0.0.1 --port 18224 --strictPort
```

In another terminal, with preview running:

```powershell
npx.cmd playwright install chromium
npm.cmd test
npm.cmd run test:ui
npm.cmd run format:check
npm.cmd run test:sites
```

Use a local account with permission to launch Chromium and read its browser cache. No remote build is required. The preview binds loopback only; no production Compose project or remote service is modified. Stop this preview with Ctrl+C in its own terminal, not by terminating unrelated Node processes.

[Design QA](design-qa.md) passed after correcting four initial findings. Evidence: 13 browser checks, six accessibility scan states with no violations, three unit tests, four template packaging tests, build and formatting checks. Browser execution is Chromium only. Screenshot artifacts and results are generated under ignored `evidence/`; regenerate them rather than treating missing local artifacts as a passed run.

Icons use the MIT-licensed [official Tabler React package](https://github.com/tabler/tabler-icons/blob/main/packages/icons-react/README.md). Fonts use local system fallbacks. The Product Design starter and its optional hosting files are retained, but nothing has been published and no production framework migration is implied.
