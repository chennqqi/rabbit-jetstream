# Candidate visual integration QA

[English](design-qa.md) | [简体中文](design-qa.zh-CN.md)

final result: blocked

Topbar follow-up: identity and clear-session controls no longer occupy a separate body row. Desktop/mobile disclosures were visually inspected; current evidence is `../artifacts/webui-selected-C2Yr8E/identity-mobile.png` and companion desktop/closed captures. Keyboard, outside-focus, short-screen scrolling and responsive focus regressions pass (`../artifacts/webui-live-3KKHHW/report.json`). This improves header density without hiding identity fields or changing clear-session safeguards. Remaining panel density/fidelity findings still require final source comparison; no new visual-QA pass is claimed.

Density follow-up: `../artifacts/webui-selected-gh56io/selected-desktop.png` was viewed together with the prior `webui-selected-ZscyyF/selected-desktop.png` at the same fixture/viewport. Four long explanations now disclose via keyboard-operable native controls; their critical boundary labels, all numeric evidence and active warnings remain visible. Metrics and replica table move earlier, but header/panel density still differs substantially from the selected source. Fixture checks prove guide access without API calls; this is not a new reference-design QA pass.

Mobile navigation follow-up: horizontal primary scrolling is replaced by a disclosure with tested Enter/Tab/Escape, focus return, route collapse and breakpoint transitions. Evidence: `../artifacts/webui-live-ioWWJy/mobile-navigation-open.png` and `../artifacts/webui-selected-ZscyyF/selected-mobile.png`. Mobile resource title/actions also wrap separately after screenshot review. The earlier primary-navigation finding's interaction portion is addressed; full accessibility/fidelity comparison and the other P1/P2 findings remain pending. This is not a new passed visual QA.

Formatting follow-up: `../artifacts/webui-selected-ThfQJC/selected-desktop.png` now shows 24h/30s and the fixed instant as Shanghai wall time with explicit GMT offset; raw duration/ISO evidence is retained. Functional assertions and real-service regression passed. This does not constitute a new visual comparison pass or close the remaining density/layout findings.

Selected-data capture is now available: `../artifacts/webui-selected-7D2S98/selected-desktop.png` (1487 × 1058, density 1), `selected-desktop-full.png`, `resource-header.png`, `evidence-configuration.png` and `selected-mobile.png`. It uses actual candidate components with Chinese/operator/orders_events/R3/offline fixture data and the selected instant, not a real backend. Source/build hashes are in its report. Only the candidate notice is annotated as synthetic. ETag 12 is distinct from synthetic Plan content revision; Leader metrics are intentionally unknown. This removes the lack of a controlled capture mechanism; a fresh reference/candidate comparison and P1/P2 fixes are still required. No new visual-QA pass is claimed here.

Header functionality follow-up: breadcrumb, whole-page read-only refresh and declaration metadata above tabs now exist (`../artifacts/webui-live-5ZWN3E/report.json`), with query/cursor/draft retention tests. This resolves the missing functional controls, not the remaining header density, icons, health evidence or same-state fidelity comparison. Full-page captures reset scroll position before capture. The original findings below remain the baseline until a new same-state visual QA pass.

Functional follow-up: scoped primary Consumer pending/ack-pending and the collection diagnostic entry are now implemented and regression-tested (`../artifacts/webui-live-nPoxKY/report.json`). This supersedes the functional portion of the earlier evidence/diagnostic finding below; same-state visual comparison and the other findings remain open. This follow-up is not a design-QA pass.

This is the real-API candidate's first selected-layout integration, not the previously verified synthetic prototype. No completed visual handoff is claimed.

## Evidence and normalization

- Source: `../docs/design/webui/queue-detail-selected.png`, 1487 × 1058.
- Implementation: `../artifacts/webui-live-8Uk6qS/queue-summary-desktop.png`, 1487 × 1303 full-page capture; CSS viewport 1487 × 1058, deviceScaleFactor 1.
- Mobile: `../artifacts/webui-live-8Uk6qS/queue-summary-mobile.png`, CSS viewport 375 × 812, deviceScaleFactor 1, full-page capture.
- Reference and implementation were opened together in the same comparison input. No scaling/cropping was used to hide the implementation's longer content.
- States differ: reference Chinese/operator/orders_events/R3/offline fixture versus implementation English/auditor/live_candidate/R1/available real API. Therefore this is a structural comparison only, not a valid final same-state fidelity pass. Focused pixel comparison is deferred until the same fixture/state is available; full views already establish blocking differences.

## Findings

- P1: Same-state comparison missing. Add an isolated selected-fixture capture that exercises the actual candidate components without mixing synthetic observations into live monitoring. Match language, role, viewport, Queue and failure state before judging final fidelity.
- P1: Resource header composition differs. Candidate warning and session disclosure precede the title; declaration metadata is below tabs, refresh is inside evidence, breadcrumb is absent. Integrate resource actions/metadata into the selected header while retaining distinct declaration/observation timestamps and permission gates.
- P1: Evidence semantics and diagnostic strip incomplete. Candidate uses stored messages/bytes/observed Consumer count, not the reference's selected-Consumer pending/ack-pending. Do not sum Consumers or invent health to imitate the image. Complete the scoped Consumer evidence and diagnostic entry before final layout QA.
- P2: Content density differs. Candidate has more explanatory copy and a six-column replica table, lengthening the panel beyond the reference. Preserve all evidence and uncertainty while improving hierarchy; do not discard fields just to match height.
- P2: Mobile primary navigation uses horizontal scrolling rather than the earlier prototype's menu. All routes remain keyboard/click reachable in the smoke, but navigation discoverability and focus behavior need dedicated accessibility review.

## Fidelity surfaces

- Fonts: Arial/Microsoft YaHei UI/system fallbacks reused from the selected prototype. Chinese same-state typography remains unqualified; live English heading and metadata hierarchy differ.
- Spacing: 250px sidebar, 70px header, content x=283 and 2.04:1 panels with 16px gap are implemented. Vertical rhythm and action placement still differ as above.
- Colors: reused dark green navigation, light background, green active links and subdued borders. Full state/contrast audit remains pending.
- Assets: no custom raster assets occur in this reference. Text RJS branding and the same pinned Tabler icon library are reused; no screenshot background or handcrafted icon replacements. Additional action/status icons are not yet integrated.
- Copy/content: real observations remain distinct from configuration. Leader metrics stay unknown when unreported; metadata does not imply health. Reference's misleading metric/lag language is intentionally not copied. Remaining exact-state content differences block fidelity acceptance.

## Verification and history

Iteration 1: introduced selected navigation/shell, collapsible identity evidence and six-field supporting configuration. Real browser regression passed; desktop side-by-side and mobile evidence-before-configuration assertions passed, including no page overflow. Identity disclosure and auditor navigation permissions were checked. Existing Consumer, audit, draft, conflict, unknown-result and broker-loss workflows passed with no page errors. 87 Node tests and candidate build passed. Screenshot comparison identified the above remaining findings; no post-fix fidelity pass exists yet.

## Next implementation steps

1. Establish same-state candidate capture and finish header/action structure.
2. Integrate scoped Consumer metrics and diagnostic entry without weakening read/mutation contracts.
3. Re-capture desktop/mobile, compare focused regions, run accessibility and interaction checks; close P1/P2 findings before a visual handoff.
