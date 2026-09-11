# Console capabilities and deployment intent

[English](webui-capabilities.md) | [简体中文](webui-capabilities.zh-CN.md)

Status: source/local candidate implementation; not a frozen release or production qualification. C-02 now includes a versioned [all-field typed Queue authoring schema](webui-queue-schema.md); complete schema-driven forms remain open.

`GET /api/v1/console/capabilities` requires an operator or auditor bearer, including when local-demo resource reads are anonymous. Missing/invalid credentials return 401, insufficient role 403, disabled authentication 404, unsupported query parameters 400. The successful response uses `Cache-Control: no-store` and schema version `rjs.console-capabilities.v1`. It performs no NATS/monitor reads or mutations; successful metadata retrieval says nothing about backend readiness.

## Explicit configuration

`RJS_DEPLOYMENT_PROFILE` accepts `unknown` (default), `standalone`, or `cluster`. Invalid values fail application initialization before telemetry or backend setup. Setting a profile declares operator intent only: it does not create nodes, change replicas, configure storage, validate capacity or establish qualification. The official standalone and cluster Compose manifests declare their corresponding profiles. The Helm chart derives the declaration from its validated `nats.replicaCount` configuration: one is standalone and three or five are cluster. Custom deployments remain unknown unless explicitly configured.

`RJS_RELEASE_MANIFEST` optionally names a local `rabbit-jetstream.io/release-bundle/v1alpha1` manifest. When configured, startup fails before telemetry or backend setup unless the process is a revision-bound clean release build and the manifest's version, server revision, and framed embedded-WebUI identity exactly match the executable. The loader is read-only, bounded to 1 MiB, rejects unknown fields, incomplete structure and trailing JSON, and does not expose its path or artifact inventory through the API. The bare-metal qualification supervisor supplies its checksum-verified frozen `runtime-manifest.json` automatically. Development and ordinary Compose runs leave this unset.

Known profiles report source `configuration`; unknown reports `unspecified`. Neither reachable nodes nor metadata replica configuration is used to infer this value. Configuration takes effect on management process restart. The candidate Settings page displays it read-only.

## Response semantics

| Field | Meaning | Not evidence of |
| --- | --- | --- |
| `deployment.profile/source` | Explicit declared intent or unknown | Actual topology, capacity or health |
| `queue.apiVersion/kind` | Supported Queue document identity | A complete JSON schema |
| `queue.supportedReplicas` / `supportedStorage` | Parser-supported values (1/3/5 and file/memory) | Production-qualified configurations |
| `queue.minimumPriority/maximumPriority` | Parser range, currently 0–255 | Qualified priority range |
| `queue.requiresExplicitReplicas` | Always true in this contract | Automatic R1/R3 selection |
| `queue.defaults` | Omitted storage/delivery defaults taken from canonical `Queue.Default()` | A complete valid Queue document or deployment recommendation |
| `features` | Implemented API contract identifiers | Permission, current backend availability or complete UI implementation |
| `qualification.status` | `unreported` | Neither approval nor failure of release qualification |

Current defaults are storage `file`, ACK wait `30s`, and max delivery attempts `5`. No replica default is reported because the parser requires an explicit supported replica count. No default is copied into editor drafts by this display. Without a matching configured manifest, qualification is `unreported`. With one, the API reports the manifest's exact qualification statement and file SHA-256 as `reported`; this records a bound statement and never converts it into a `qualified` verdict. Release approval, signatures, native soak and Canary evidence remain separate gates.

Settings loads capabilities separately from the verified session. A failed or incompatible refresh removes previous capability values and reports unknown/unavailable; it does not substitute node counts, permissions or frontend constants. Refresh is explicit and read-only. Existing guided creation still requires explicit replicas/storage and validates through server preview. Candidate mutation workflows now use the guarded reads described below. Complete schema-based form generation and automatic refresh preferences remain future integration work.

## Capability-bound previews and pre-dispatch checks

The candidate enables capability checks for all create/edit/delete models. It reads authenticated capabilities before and after preview, verifies required contract identifiers and binds the preview to a canonical fingerprint of the response. Object-key order and order within the features/replicas/storage sets do not count as changes. Deployment, defaults, supported ranges, schema or other contract data changes do. Missing/incompatible capabilities or missing required identifiers fail closed.

Immediately before PUT or DELETE, another read must match the bound fingerprint. Failure clears the preview and confirmation, retains the draft/original ETag or deletion evidence, and returns to preview-error without generating a mutation request ID or sending the write. Deletion force and typed confirmation reset. Explicit fresh preview and confirmation are required to proceed. Errors after entering the actual write call remain conservatively unknown and cannot be retried automatically. Downloaded evidence retains the capability binding used for review.

The frontend checks are point-in-time protection, supplemented by the receiving-instance precondition below. They are not a reservation or cluster-wide atomic snapshot. The backend continues independent authorization, declaration preconditions, planning and ownership checks. Advertised Queue schemas now have a version and content ETag included in this binding; the [schema contract](webui-queue-schema.md) describes required schema reads and their limits. No background polling or hidden write retry is introduced.

## Shared capability observations

Completed capability reads through the shared API, including Settings refresh, notify retained create/edit/delete models within the same credential generation. A changed body/revision, incompatible response or failed read invalidates unsubmitted reviews and clears confirmation before returning to the page. Draft text and original declaration ETags remain; deletion force, typed confirmation and acknowledgement reset. Unchanged reads preserve reviews. Fresh explicit preview and confirmation are required.

Pending operations, unknown outcomes, accepted receipts and archived evidence are not reset or unlocked by these notifications. Archived/discarded models unsubscribe. Responses started under older credentials do not notify the new session, and observer exceptions cannot alter transport outcomes. This is observation-driven invalidation, not continuous monitoring; changes not yet observed still require the preview/pre-dispatch and receiving-instance checks above.

## Receiving-instance contract precondition

Capabilities GET returns an opaque strong `ETag` (`"rjs-capabilities-v1:<64 lowercase hex characters>"`) derived from immutable capability data and the management build's reported version. Deployment intent and canonical defaults participate; process name, uptime, credentials and resource observations do not. This token is separate from the Queue declaration ETag. It is not binary attestation: different binaries reporting the same version and capability data can share a token.

The `capability-preconditions` feature advertises support for `X-RJS-If-Capabilities-Match`. Candidate create/edit/delete flows require this feature and a valid capabilities ETag, compare both body fingerprint and opaque revision, and send the original revision on POST preview, GET delete-preview, PUT and DELETE. Older servers missing the contract cannot enable candidate mutations.

After authorization, the receiving instance checks this optional header against its own immutable contract before parsing/planning or recording audit intent. A mismatched tag returns 412 `capabilities_changed`; empty, weak, malformed, repeated or combined values return 400 `invalid_capabilities_precondition`. Missing headers retain legacy API compatibility. The condition does not replace Queue `If-Match`/`If-None-Match`, ownership checks or deletion confirmation, and it does not freeze messages, Consumers, external writers or the cluster.

A 412 from a read-only preview can invalidate review. Once a PUT/DELETE has entered transport, a final 412 still does not prove that an earlier transparent browser replay did not execute; candidate state remains unknown, never displays “no write sent” for that case, and never automatically retries. API tokens, TLS and per-request authorization remain necessary; the capability token is not a permission or secret.

Verification includes authenticated nil-backend handler tests, canonical defaults and parser acceptance checks, profile initialization tests, frontend malformed/stale-read cases, and local browser Settings refresh/denial coverage. These checks do not establish production qualification or actual deployment capacity.
