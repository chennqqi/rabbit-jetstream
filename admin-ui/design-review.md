# Admin Console Product Review (PM Review)

[English](design-review.md) | [简体中文](design-review.zh-CN.md)

Status: review complete, pending owner scheduling · Review date: 2026-10-01
Subject: `admin-ui/dist/` (the committed embedded candidate, served by a locally built `rjs-management` binary)
Method: local real-service walkthrough in Chromium. A local JetStream (NATS) node and the management server were started, 6 Queues and consumer data were seeded through the API, and every page was manually reviewed after local-account login (desktop 1440×900, mobile 375×812, both languages). All findings come from rendered pages; screenshots are in [design-review-screens/](design-review-screens/). Reproduction steps in the [appendix](#appendix-environment-and-reproduction).

---

## 1. Overall conclusion

**The engineering candidate achieves correctness, not product quality.** The console executes its "observation versus declaration honesty" model rigorously, and its bilingual and accessibility work is above the bar of most internal tools. But today it reads like an **acceptance interface built for the system's own authors**, not a daily tool for on-call operators, platform administrators, and auditors. The Overview page spends three screens on disclaimers instead of answering "is the system healthy, and where do I need to act".

Dimension scores (out of 5):

| Dimension | Score | Summary |
|---|---|---|
| Feature coverage | 4.0 | Complete read paths; write path has a preview-confirm loop |
| Bilingual & accessibility | 4.5 | Aligned en/zh structures; visible focus/aria investment |
| Information architecture | 2.0 | 13 flat nav entries; actions mixed with resources |
| Visual design | 2.0 | Clean skeleton, but no hierarchy, no status semantics, no brand feel |
| Data visualization & decision support | 1.5 | Almost zero graphics; key numbers carry no emphasis or thresholds |
| Copy & user communication | 2.0 | Disclaimer culture and engineering jargon shift proof burden onto users |
| Interaction efficiency | 2.5 | Any refresh drops the session; detail-page actions are under-weighted |
| Stability (observed this session) | 2.0 | One P0 crash that kills the whole management process |

**Overall: 3.0 / 5.** The foundations (token discipline, bilingual structure, semantic model) are worth keeping, but judged against "hand this to a real operations team", the current state hurts both first impression and daily trust.

---

## 2. Users and core questions (review baseline)

Each page was checked against whether it answers, within 30 seconds, the most common question of each target user:

| User | Frequent question | Answered today? |
|---|---|---|
| On-call operator | Is the system healthy? Which queue is most backlogged? | ❌ Overview has no backlog ranking or health bands; 500 pending deliveries get no visual emphasis |
| Platform admin | Who changed what? Who has access now? | ⚠️ Audit works but the filter barrier is high; permissions are shown as raw English codes |
| Auditor | Did changes go through preview-confirm? Where is the evidence? | ⚠️ The evidence-chain concept is good, but entries are plain text links with poor discoverability |

---

## 3. Key findings

Severity: **P0** blocks trust/usability, **P1** significantly hurts daily experience, **P2** polish.

### P0-1 With Prometheus unconfigured, the alert-monitoring path can kill the entire management process

- **Observed**: in a deployment without `RJS_PROMETHEUS_URL`, after logging in and browsing normally for a few minutes, the management process panicked and exited (stack: `prometheus.(*Client).AlertRules` ← `api.watchAlerts`, `management/internal/api/events.go:227`). Every console page went down with it.
- **PM impact**: this is not about looks — it is a **reliability defect**. "Operational alerts" is a primary-nav feature; an unconfigured dependency must not be able to crash the control plane. Operator trust starts with "the console is always there".
- **Recommendation**: add a receiver nil-guard to `AlertRules` (return a "not configured" error); add a failure-path test: no Prometheus config + alert watcher running + SSE subscription, asserting the process stays alive.
- Reproduction and evidence in the [appendix](#appendix-environment-and-reproduction).

### P1-1 The Overview has no overview value: no health bands, no backlog ranking, no trends

![Overview top](design-review-screens/04-overview-top.png)

- **Observed**: the Overview is three text cards ("Monitoring issues and coverage", "Management service and account", "Declared Queues"). The essential content is "configured endpoints: 1", "declared Queue total: 6", "account storage usage (bytes) 64713". Three screens, zero graphics, zero rankings, zero drill-through.
- **PM impact**: an on-call operator gets no "should I worry" signal from the first screen. Byte counts are not humanized (64.7 KB shown as "64713").
- **Recommendation**: rebuild the Overview as a cockpit: a health summary strip (endpoints/Queues/Consumers counts + status colors); a "top 5 queues by pending deliveries" list linking to details; humanized numbers; disclaimers folded into info tooltips.

### P1-2 Key states and backlog carry no visual semantics

![Queue detail summary](design-review-screens/06-queue-detail-summary.png)

- **Observed**: on the Queue summary, "pending (primary consumer) 500" is styled exactly like "pending ack 0"; the list page repeats the same long plain text "Stream consistent; Consumer configuration not checked" on every row; the node page's "monitor read succeeded" is plain text. The console has no status badges and no color semantics anywhere (amber appears only on the disclaimer banner).
- **PM impact**: 500 backlogged messages and zero backlog look identical, which hands triage back to the user. The core value of an operations console is translating state into priority.
- **Recommendation**: introduce a three-tier badge system for observed state / read state / alerts (the token palette already exists); pending deliveries beyond a threshold get warning color and bold; split "Stream consistent; Consumer configuration not checked" into a badge (consistent ✓) plus muted secondary text.

### P1-3 Information architecture: 13 flat nav entries, actions mixed with resources

- **Observed**: primary nav is "Overview / Queue list / Stream list / Consumer list / Node list / Audit / Create Queue / Bulk changes / Diagnostics / Operational alerts / Access and settings / Tenant access / Compatibility" — 13 items, no grouping. "Create Queue" (an action) sits next to "Queue list" (a resource); "Compatibility" (build metadata, developer-facing) sits with core operational resources.
- **PM impact**: navigation is the first mental model users internalize. Mixing actions into resources makes people scan the whole sidebar to find create/edit; mixing developer pages into operational pages dilutes professionalism.
- **Recommendation**: group, e.g.:
  - **Resources**: Overview / Queues / Streams / Consumers / Nodes
  - **Operations**: Alerts / Diagnostics / Audit
  - **Governance**: Bulk changes / Tenant access / Access and settings
  - Move "Create Queue" into the Queue list page header as the primary button; demote "Compatibility" to a settings entry or footer link.

### P1-4 Any full page refresh drops the login, while refresh preferences survive

- **Observed**: the access token lives in memory only (a deliberate security design), so any full refresh (F5, opening a shared link) returns to the login form; language and refresh preferences are kept locally.
- **PM impact**: refreshing is a high-frequency action in on-call work. Paying "log in again" for it teaches users to avoid refreshing and rely on stale data — the opposite of this console's freshness-first philosophy.
- **Recommendation**: keep the no-persistent-token boundary but offer a safe middle ground: after a refresh, show "session expired — re-verify with one step" (an in-page password re-entry that restores the session); or an opt-off "remember username (this device only)". At minimum, acknowledge the trade-off in the product instead of letting users discover it.

### P1-5 Disclaimer culture: the system's evidence boundaries are shifted onto users

![Double disclaimers atop the Queue list](design-review-screens/02-queue-list.png)

- **Observed**: nearly every page opens with 1–3 negating sentences: "Each page joins one declaration enumeration with one Stream enumeration. Counts and Stream consistency are observations, not Consumer-configuration, convergence or health proof." "Reads come from independent sources, not an atomic snapshot." "30 seconds is the console freshness threshold, not a Broker health threshold." Plus implementation detail like "Automatic refresh: 10 seconds after each read; failure backoff up to 60 seconds."
- **PM impact**: the honesty **concept** is right (it is a genuine differentiator), but the delivery makes every page open with "our numbers may not mean what you think", eroding rather than building trust. Operators need default trust with on-demand verification.
- **Recommendation**: demote disclaimers to a collapsible "about these numbers" tooltip or fine print at page bottom; turn "auto refresh 10s" into a status indicator ("refreshed 11:23:58 · auto in 10s"); use in-page amber banners only for genuinely degraded situations (stale data, partial failure).

### P1-6 The Queue "Configuration" tab is a one-line JSON dump; declared-vs-observed has no visualization

![Configuration tab as one-line JSON](design-review-screens/08-queue-detail-config-json.png)

- **Observed**: the Configuration tab's body is one un-wrapped compact JSON line (`{"apiVersion":"rabbit-jetstream.io/v1alpha1",...}`) plus a collapsed "declared Plan (raw data)". Declared-vs-observed comparison is the console's core value proposition, yet no page offers a structured diff view.
- **PM impact**: to answer "how is this queue actually configured", operators must read JSON; "does the declaration match reality" requires mental joins across tabs.
- **Recommendation**: render a structured key-value table (storage/replicas/retention/delivery, one row each); on the summary, place "declared value vs observed value" side by side with mismatches highlighted. Keep raw JSON in the existing collapsed "raw data" area (the pattern is already there — extend it).

### P2 Visual and interaction polish list

| # | Observation | Evidence | Recommendation |
|---|---|---|---|
| 1 | Full 32-char Plan revision hash appears in lists and detail; English UI overflows horizontally at 1440px | [03](design-review-screens/03-queue-list-en-hscroll.png), [08](design-review-screens/08-queue-detail-config-json.png) | Truncate to 8 chars + copy button; column-width governance to remove 1440px overflow |
| 2 | Login page shows a permanent red "SSO sign-in unavailable or callback invalid" when SSO is not configured | [01](design-review-screens/01-login.png) | Do not render an error when SSO is unconfigured; at most a collapsed note |
| 3 | Login button and the create flow's primary CTA use the same outline style as secondary buttons | [01](design-review-screens/01-login.png), [10](design-review-screens/10-queue-create.png) | Establish primary (solid) vs secondary (outline) button hierarchy |
| 4 | Create-form controls stretch to a full 1300px; inputs and selects feel visually heavy | [10](design-review-screens/10-queue-create.png) | Cap control width (~480px); two-column layout |
| 5 | Branding duplicated: RJS / Rabbit JetStream / management console each appear twice (sidebar + top bar) | [02](design-review-screens/02-queue-list.png) | Top bar keeps page context only (breadcrumb + actions); brand stays in the sidebar |
| 6 | Queue detail actions "Edit draft and preview" / "Review deletion" are plain text links — under-weighted primary actions | [06](design-review-screens/06-queue-detail-summary.png) | Promote detail actions to a button group (edit = primary, delete = danger outline) |
| 7 | Native file inputs, native selects, monospace permission-code bullets | [11](design-review-screens/11-queue-create-import.png), [19](design-review-screens/19-access-settings.png) | Unified control skins; human-readable permission descriptions |
| 8 | Consumer filter area: full-width input + full-width "filter consumers" button, five controls filling a screen for one data row | [09](design-review-screens/09-queue-detail-consumers.png) | Collapse the filter area into a single toolbar row |
| 9 | Mobile 375px: brand + identity + buttons take 108px of top bar, plus banner, nav, three disclaimer lines before content | [22](design-review-screens/22-mobile-nav.png), [23](design-review-screens/23-mobile-queue-list.png) | Compress the mobile top bar to one line; make the banner dismissible |
| 10 | "Verified identity" hides in a ▶ disclosure while "Clear local session" is a top-level button — inverted weight | [02](design-review-screens/02-queue-list.png) | Show identity (username + tenant) flat; fold session clearing into an identity menu |
| 11 | Stream list has only two columns; system streams (KV_RJS_META) are indistinguishable from business streams | [12](design-review-screens/12-stream-list.png) | Add retention/replicas/storage columns; badge internal streams |
| 12 | Alerts degraded state is a single red line with no next step | [17](design-review-screens/17-alerts-degraded.png) | Explain "an administrator must configure RJS_PROMETHEUS_URL" with a doc link |
| 13 | Tenant access "create account" tenant checkbox has no readable label (a lone blue checkbox) | [20](design-review-screens/20-tenant-access.png) | Label it, e.g. "grant membership for tenant local" |

---

## 4. Page-by-page walkthrough record

| Page | First screen answers the core question? | Main observations |
|---|---|---|
| Login | ✅ | Misleading SSO error; no button hierarchy; large empty space ([01](design-review-screens/01-login.png)) |
| Overview | ❌ | Three screens of text cards; no health bands/rankings/trends; raw byte counts ([04](design-review-screens/04-overview-top.png), [05](design-review-screens/05-overview-bottom.png)) |
| Queue list | ⚠️ partial | Repeated long state text per row; full-length hashes; unbalanced filter controls ([02](design-review-screens/02-queue-list.png)) |
| Queue detail · summary | ⚠️ partial | The only page with big numbers (good); backlog has no alert semantics; actions are text links ([06](design-review-screens/06-queue-detail-summary.png)) |
| Queue detail · configuration | ❌ | One-line JSON dump ([08](design-review-screens/08-queue-detail-config-json.png)) |
| Queue detail · consumers | ⚠️ | Filter area fills a screen; full-width button ([09](design-review-screens/09-queue-detail-consumers.png)) |
| Create Queue | ❌ | Full-width controls; JSON draft editing mixed with the form; two file-upload areas stacked on one page ([10](design-review-screens/10-queue-create.png), [11](design-review-screens/11-queue-create-import.png)) |
| Stream list | ⚠️ | Two columns only; no system-stream marking; horizontal scrollbar ([12](design-review-screens/12-stream-list.png)) |
| Stream detail | ❌ | Single-column label-value stacking, no grid or hierarchy ([13](design-review-screens/13-stream-detail.png)) |
| Consumer detail | ⚠️ | 500 pending with no emphasis; single-column stacking ([14](design-review-screens/14-consumer-detail.png)) |
| Node list | ⚠️ | 55-char Node ID as link text; plain-text status; bottom half empty ([15](design-review-screens/15-node-list.png)) |
| Audit | ❌ | Six full-width filter controls + hand-typed RFC3339; no time presets (last 1h/24h) ([16](design-review-screens/16-audit.png)) |
| Operational alerts | ❌ | Degraded state is one red line with no guidance; page 90% empty ([17](design-review-screens/17-alerts-degraded.png)) |
| Bulk changes | ⚠️ | An entire page holding one upload row plus a disclaimer ([18](design-review-screens/18-bulk-change.png)) |
| Access and settings | ⚠️ | Raw English permission codes as bullets; verbose explanations ([19](design-review-screens/19-access-settings.png)) |
| Tenant access | ⚠️ | Unlabeled checkboxes; scattered layout ([20](design-review-screens/20-tenant-access.png)) |
| Compatibility | — | Build metadata with no operator action value; demote the entry ([21](design-review-screens/21-compatibility.png)) |
| Mobile 375 | ❌ | Top bar + banner + nav + disclaimers crowd out the first screen; tables scroll horizontally ([22](design-review-screens/22-mobile-nav.png), [23](design-review-screens/23-mobile-queue-list.png)) |

---

## 5. Strengths to keep

1. **Bilingual engineering**: complete, structure-aligned en/zh with test protection; instant switching (verified).
2. **The semantic honesty concept**: declared vs observed distinction, no invented health — a rare differentiator; only the expression needs work.
3. **Accessibility investment**: skip-to-content, unified focus rings, table captions, aria-live route announcements.
4. **Token discipline**: all colors through the token layer, dark theme follows the system, restrained iconography — a clean foundation for the visual upgrade.
5. **The write-path preview-confirm-conditional-write loop**: drafts, server-side preview, ETag conflict recovery — safer than many mature messaging consoles.
6. **Degradation is never hidden**: history and alerts clearly error when unavailable (the copy is stiff, but it does not fake normal).

---

## 6. Recommended roadmap

### Quick wins (1–2 weeks, no architecture change)
1. Fix the P0-1 panic + regression test (half a day).
2. Ship the status badge system: three tiers for observed state / read state / alerts (palette exists in tokens).
3. Hash truncation + copy; humanized bytes; 1440px table overflow fixes.
4. Demote page-top disclaimers to tooltips; turn "auto refresh" into a status indicator.
5. Fix the login page's misleading SSO error; primary/secondary button hierarchy.
6. Group the navigation; move "Create Queue" into the list page header.

### Mid-term (1–2 months)
1. Overview cockpit rebuild (health strip + top-5 backlog + trend charts — Prometheus data already exists).
2. Declared-vs-observed side-by-side comparison (Queue detail summary).
3. Table column-width and responsive governance: card layouts for mobile lists.
4. Session refresh recovery flow (keeping the no-persistent-token boundary).
5. Audit time presets + structured filters.

### Long-term
1. Customizable dashboards and subscriptions (alerts evolving from read-only to closed-loop).
2. Tenant-personalized home; manual dark-theme toggle.
3. A full usability round with the "answer core questions in 30 seconds" acceptance bar (3 users per persona).

---

## Appendix: environment and reproduction

```
# Build and start (verified on local Windows)
cd upstream/nats-server && go build -o ../../bin/nats-server.exe .
go build -o bin/rjs-management.exe ./management/cmd/rjs-management
./bin/nats-server.exe -js -a 127.0.0.1 -p 4222 -m 8222 -sd <data-dir>

RJS_ADMIN_TOKEN=review-admin-token \
RJS_HTTP_ADDR=127.0.0.1:8223 \
RJS_LOCAL_ACCOUNTS_FILE=<accounts.json> \
RJS_LOCAL_AUTH_SIGNING_KEY=<key> \
./bin/rjs-management.exe
# Browse http://127.0.0.1:8223/admin/
```

- Demo data: 6 Queues declared via `PUT /api/v1/queues/{queue}` (`If-None-Match: *`); ~700 messages published and a pull consumer created through NATS.
- P0-1 crash evidence: start without `RJS_PROMETHEUS_URL`, log in and browse; after a few minutes the process exits with `panic: runtime error: invalid memory address or nil pointer dereference`, top frames `management/internal/prometheus/alerts.go:49` and `management/internal/api/events.go:227` (watchAlerts). Setting `RJS_PROMETHEUS_URL` to an unreachable address avoids the crash (error path instead).
- Screenshot inventory: [design-review-screens/](design-review-screens/), 23 files, names matching the references above.

## Review boundaries

- This is a product-perspective walkthrough; it does not repeat the pixel-fidelity checks in `design-qa.md` and does not replace the axe/playwright gates.
- Not covered: multi-node clusters, OIDC/browser SSO flows, the diagnostics download flow, the full bulk-change execution flow. Recommend a follow-up round.
- All "recommendations" are product suggestions; visual-spec changes must still follow [webui-selected-design.md](webui-selected-design.md) and the `shell.css` token discipline (new colors enter the token layer only through visual QA).
