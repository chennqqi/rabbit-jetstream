# WebUI S-4 Visual Qualification

[English](webui-s4-visual-qa.md) | [简体中文](webui-s4-visual-qa.zh-CN.md)

## Result

S-4 is complete. The Queue/Stream and global Consumer read-only collections now use shared native-table and offset-pagination primitives. The density pass preserves captions, scoped headers, keyboard-focusable horizontal overflow, links, live pagination ranges, and explicit error/stale evidence.

The deterministic visual matrix passed on 2026-09-13 against an isolated Docker Desktop deployment built from the local working tree. The run produced 96 Chromium screenshots:

- widths: 1440, 1280, 1024, and 375 CSS pixels;
- languages: English and Simplified Chinese;
- themes: light and dark;
- states: normal, empty, error, stale, long identifier, and maximum column population.

Every case asserted zero document overflow and zero clipped buttons, inputs, or selects. Firefox at 375 CSS pixels additionally proved that the table region receives keyboard focus, has real horizontal overflow, and can scroll without causing document-level overflow.

## Manual review

Representative screenshots from every dimension and the complete matrix were reviewed. The first pass exposed a real 1280-pixel layout defect: the Consumer filter selected its wide grid from viewport width without accounting for the 250-pixel navigation rail. The wide breakpoint was moved to 1451 pixels and the entire matrix was rerun. A second evidence defect, reversed language labels in screenshot filenames, was then corrected by pinning both browser locale and `rjs.language`; the complete matrix was rerun again and passed.

The screenshots and machine-readable `manifest.json` are generated under ignored `artifacts/s4-visual-qa/`. They are local qualification evidence, not release artifacts.

## Reproduction

Start a disposable same-origin management deployment with the recovery token `s4-visual-qa-token` on `http://127.0.0.1:18223`, then run:

```powershell
cd admin-ui
npm.cmd run test:visual-qa
```

The harness mocks only the global Consumer collection response so every visual state uses frozen data. Authentication, routing, embedded assets, session behavior, layout, browser rendering, and responsive behavior remain real.

