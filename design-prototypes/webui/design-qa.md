# Design QA — Queue detail prototype

[English](design-qa.md) | [简体中文](design-qa.zh-CN.md)

## Comparison baseline

- Source: [selected visual](../../docs/design/webui/queue-detail-selected.png), 1487 × 1058 pixels.
- Implementation: [desktop capture](evidence/desktop.png), viewport 1487 × 1058 CSS pixels, deviceScaleFactor 1. No density resizing.
- State: Chinese, overview tab, orders_events revision 12, fixed mock observation 2026-09-09 16:20:00, offline nats-3.
- Full comparison: [pass 1](evidence/comparison-pass1.png); source left, implementation right.
- Focused comparison: [pass 1 detail](evidence/comparison-detail-pass1.png), equal crops x=280, y=350, 1180 × 570.
- User explicitly authorized local headless Playwright. No production endpoints used.

## Pass 1 findings

- [P1] Vertical alignment drifts: implementation header is 100px instead of approximately 70px; details begin at y=409 instead of approximately 362. Diagnostic controls fall below the source viewport. Reduce header and inter-section spacing.
- [P2] Metric explanatory text wraps and increases panel height. Preserve corrected semantics with shorter captions; match evidence panel height without hiding content.
- [P1] Narrow-screen access button has no accessible name when its text is hidden. Add an explicit translated label.
- [P1] Diagnostic secondary text fails contrast on the tinted surface. Darken the text.

Fonts/typography: system fallback retained; spacing/caption wrapping needs correction. Layout: column widths match (approximately 780/382px), vertical rhythm does not. Colors: existing green/light palette retained; diagnostic contrast fails. Image/assets: RJS text mark and licensed Tabler library icons, no fabricated raster artwork. Copy: intentionally corrected overlapping metric scopes and unknown Lag semantics; no data fields removed.

## Verification

[Pass 1 results](evidence/results-pass1.json): 11 interaction checks passed; zero console errors and zero external requests. Automated accessibility failed on contrast and the mobile button name. Desktop, review, English and 375px states were tested. Manual screenshot comparison is not yet passed.

## Implementation checklist

- Fix P1/P2 findings and rebuild.
- Repeat same-view captures and comparisons; review mobile and edit views.
- Record post-fix evidence before claiming completion.

## Pass 2 and final verification

Fixed all four pass-1 findings: header now 70px; detail begins at y=362.80; primary/supporting widths 779.75/382.25px; panel height 553.56px; diagnostic strip begins at y=931.36 and is visible in the source viewport. Shortened metric captions without removing fields or changing values. Added a translated accessible access-button name and darkened diagnostic text. Narrow layouts now explicitly retain the mock-data/deployment label.

Post-fix evidence: [full comparison](evidence/comparison.png), [focused comparison](evidence/comparison-detail.png), [review](evidence/review.png), [375px](evidence/mobile.png), [mobile editor](evidence/mobile-edit.png), [1440px](evidence/viewport-1440.png), [768px](evidence/viewport-768.png), [mobile English](evidence/mobile-english.png). Source and implementation were inspected together in both full-view and focused comparison inputs. Supplemental responsive/edit states have no selected source mock; their usability was checked separately, not claimed as pixel-matched designs.

Final [browser results](evidence/results.json): **13 checks passed**, zero page/console errors, zero external requests; **zero accessibility violations in six scanned states** (Chinese desktop, English desktop, review, 375px Chinese, 375px editor, 375px English). Three model unit tests, four template packaging tests, production build and formatting check passed. Packaging tests do not mean deployment occurred.

Five fidelity surfaces: typography remains legible with source-like scale and system fallback; main layout/proportions now align; tokens preserve green/light semantics with improved contrast; library icons and textual RJS branding are appropriate without synthetic asset substitutions; copy changes intentionally correct metric semantics while preserving all selected fixture fields. No remaining actionable P0/P1/P2 issues were found in the checked states.

Follow-up P3: exact glyph shapes and line-icon contours differ from a generated image and by platform; no claim of pixel identity. Mobile English tabs intentionally scroll horizontally and remain keyboard reachable. A complete production accessibility audit, Firefox/Safari execution and real API behavior are outside this prototype evidence. Source-specific pixel matching applies to the 1487 × 1058 Chinese overview only.

Evidence files live in the ignored `evidence/` directory and can be regenerated with the local preview plus `npm run test:ui`. No tests used remote qualification hosts or production data.

## Logic-review iteration (2026-09-09)

Scope: selected two-column overview plus authorized Consumer list/detail and state corrections. This is a visual/prototype gate, not closure of real API requirements in the [logic review](../../docs/webui-logic-review.md).

First comparison found P2 drift: longer metric copy and a default-height Consumer link moved the replica section and diagnostic strip down. Shortened the caption and gave the inline link its own 24px minimum target, zero excess padding and no auto margin. Recaptured and inspected the source and implementation together in [full](evidence/comparison.png) and [focused](evidence/comparison-detail.png) inputs, same 1487×1058 CSS/pixel viewport and density 1. Final detail top is 362.80px, columns 779.75/382.25px, panel height 553.77px, diagnostic top 931.56px. Source content changes (named Consumer, mock KV revision, stale footer) are intentional logic corrections, not accidental fidelity drift.

Supplemental inspected evidence: [list page 2](evidence/consumers-page2.png), [detail](evidence/consumer-detail.png), [failed observation](evidence/consumer-stale.png), [mobile list](evidence/mobile-consumers.png), [mobile detail](evidence/mobile-consumer-detail.png). Long-page captures are taken from scroll top to avoid displaced fixed-navigation artifacts. New states have no separate selected image; they were checked for usability, not pixel identity.

Typography: existing system fonts and hierarchy remain readable. Spacing: overview rhythm restored; mobile detail stacks and list has a labeled keyboard-focusable horizontal region. Colors: existing semantic tokens retained; warning text checked. Assets: unchanged Tabler icons/text branding, no generated asset substitutions. Content: metric ownership, expected resources, declaration/observation and mock limitations explicit. No actionable P0/P1/P2 visual issues remain in the checked scope; previous P3 font/icon differences remain.

Latest verification: **19 browser checks; 8 model tests; 4 packaging tests; 11 accessibility states with zero automated violations; zero browser errors/external calls; build and formatting passed**. Test setup was corrected to reset language/session between independent cases. Real authorization, concurrency, durable audit and backend failure behavior remain unqualified. The mock fixtures do not close production P1 findings.

## Mutation recovery iteration (2026-09-09)

The selected overview and latest implementation were recaptured at 1487×1058, density 1, and inspected together in the existing full/focused comparison inputs. Overview geometry and assets are unchanged. Added outcome dialogs use the same fonts, semantic warning colors, table spacing and responsive dialog shell. [Desktop conflict](evidence/mutation-conflict.png), [375px conflict](evidence/mobile-mutation-conflict.png), [unknown outcome](evidence/mutation-unknown.png) and [partial inspection](evidence/mutation-partial.png) were opened and inspected; these supplementary states are not claimed as source-image matches. Buttons remain visible and text is readable in the checked viewports.

One blocking behavior was found and fixed: a recovery click inherited the default action of its newly rendered submit button and entered review unexpectedly. `preventDefault` plus browser assertions now requires an independent review step. Final run: 26 browser checks, 15 model tests, 4 packaging tests; zero automated accessibility violations in 15 states, zero browser errors/external calls. No new actionable P0/P1/P2 visual issues; previous P3 type/icon differences remain. [Recovery contract and limitations](../../docs/webui-mutation-design.md) explicitly retain real API qualification gaps.

final result: passed
