# Connection subscription detail proposal

[English](webui-subscription-design.md) | [简体中文](webui-subscription-design.zh-CN.md)

Status: the recommended authenticated `resources:read` visibility policy, bounded monitoring/API contract and explicit-load WebUI are implemented and locally verified. Exact client-name/user/account/MQTT identity search is implemented separately. The bounded local 1,000-row latency/RSS/client-lock qualification now passes; formal production-host qualification remains separate.

## Verified upstream constraints

The pinned `upstream/nats-server/server/monitor.go` supports exact `cid` with `subs=detail`. `newSubsDetailList` enumerates that client's subscriptions under its client lock; there is no independent subscription offset/limit. Connection offset/limit does not bound the number of subscriptions. `SubDetail.sid` is a string, while `cid` is uint64 and `msgs`/`max` are int64. A subscription identity is node ID + CID + exact SID, not subject, queue-group name or numeric interpretation of SID. Multiple subscriptions can share the same subject.

`ConnInfo.subscriptions` and its subscription details are produced while holding the same client lock. Empty detail arrays can be omitted by JSON `omitempty` when the count is zero. For a positive count, absent detail rows must never mean an empty subscription collection. The response `total` remains the node connection count, not the number of matching subscriptions. Inspect the full assignment path, not the similarly named local `totalClients` variable.

## Metadata visibility decision

Recommended: reuse existing `resources:read` for authenticated operator and auditor, exposing operational Subject, queue-group and SID alongside numeric counters. Exclude account/identity tags, names, IP/ports, credentials, JWTs, certificates and raw upstream objects. Document that operational identifiers may contain business-sensitive text and are not safe to publish. The existing explicit loopback-demo resource-read exception must also be considered: approving this option includes those operational fields in demo resource reads unless a separately approved policy restricts it.

Alternative: operator-only access to these operational identifiers, with auditors retaining numeric connection diagnostics. This requires a distinct enforced endpoint permission and corresponding session/UI capability; disabling a button alone is insufficient. Do not silently change the meaning of the existing shared read role.

The owner confirmed the recommended shared authenticated read policy. This does not permit message payload access or any publish/ack/replay action, and it creates no new role. Literal-loopback demo mode retains its already documented resource-read exception; non-loopback deployments require an authenticated operator or auditor.

## Implemented bounded read contract

- Dedicated explicit read of one exact node/CID; do not add `subs=detail` to the periodically refreshed connection list or basic detail.
- Reuse complete unique-node resolution and one five-second deadline. Read numeric detail first; a reported count above 1,000 rejects expansion before requesting subscription rows. Missing count is unknown, not zero.
- If eligible, request exact CID with `auth=false` and `subs=detail`. Recheck node/CID identity, source time, reported count, row count and SID uniqueness. Missing connection remains distinct from unavailable observation.
- Limit response bytes to the existing 2 MiB and accepted rows to 1,000. Growth between preflight and expansion can still exceed limits; return an explicit limit/unavailable result, never a silently truncated success. The preflight does not prove upstream CPU/allocation bounds or eliminate client-lock cost.
- Keep SID as an exact string. Define bounded UTF-8/control-character validation for SID, Subject and queue group before implementation; do not coerce arbitrary identifiers or merge repeated Subjects. Preserve exact integer counters; absent optional `max` is unreported, not a made-up limit.
- Once a complete bounded observation is accepted, deterministic SID ordering and local pagination/filtering are allowed only when labeled as operating on that complete observation. A refresh produces a new observation, not a stable snapshot spanning requests. Above-limit collections remain explicitly unsupported until a different approved source can provide bounded paging; this does not satisfy arbitrary-scale subscription browsing.
- UI starts unobserved and loads only on explicit action, explains metadata sensitivity/limits, and distinguishes no subscriptions, absent connection, denied, unavailable, limit exceeded and incompatible data. Do not infer Consumer ownership, backlog or health from transport subscriptions. Clear data on session disposal or confirmed denial/identity change.

The 1,000-row limit and explicit-load behavior are now measured on the recorded local host. This evidence bounds the tested candidate and host only; it is not a production sizing promise. No server subtree changes, browser monitoring-port exposure, subscription mutation or remote workload is part of the qualification.

## Required acceptance evidence

Unit/contract tests: nonnumeric and leading-zero SID preservation; duplicate SID rejection without collapsing duplicate Subjects; exact large counters; absent zero-count array versus malformed positive-count array; wrong CID/node; truncated/oversized responses; count growth; sanitized errors; deadline/cancellation; selected role policy enforced before upstream I/O.

Real isolated NATS tests: create only owned clients/subscriptions (no publish), include repeated Subjects and distinct string SIDs, queue groups, wildcard Subjects, unsubscribe/churn, zero subscriptions and above-limit cases. Stop only owned processes/sockets. Build all helpers locally and record artifact fingerprints.

Browser tests: explicit loading only, truthful full-observation filtering/paging, row identity, errors/recovery, role denial and session clearing, bilingual long identifiers, keyboard/accessible labels and narrow-screen screenshots. Continue separate scale/lock-cost evaluation and the full WEB-019 search work; a passing bounded demo is not overall completion.
