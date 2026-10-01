import {parseJSON, stringifyJSON} from "./api.mjs";
import {validateAuditWindow} from "./audit-window.mjs";
import {previewOperations} from "./preview-operations.mjs";
import {declarationReview} from "./declaration-review.mjs";
import {canArchiveEditor} from "./editor-handoff.mjs";
import {readMutationCapabilities,capabilityHeaders,observedCapabilityChange} from "./mutation-capabilities.mjs";
import {createRequestID} from "./request-id.mjs";

const copy = value => value === undefined ? undefined : parseJSON(stringifyJSON(value));
const validETag = value => typeof value === "string" && /^"[1-9][0-9]*"$/.test(value);
const editable = new Set(["editing", "review", "blocked", "preview-error", "conflict", "denied"]);

// Runtime-independent real-API workflow. The UI must render its state and
// obtain user confirmation before discard/session clear. No browser storage.
export function createQueueEditor(api,{requireCapabilities=api.requireMutationCapabilities===true}={}) {
  let generation = 0;
  let state = {phase: "idle"};
  const endpoint = () => `/api/v1/queues/${encodeURIComponent(state.name)}`;
  const headers = () => state.create ? {"If-None-Match": "*"} : {"If-Match": state.etag};
  const locked = () => ["loading", "previewing", "submitting", "uncertain", "inspecting", "reading-conflict", "reading-next"].includes(state.phase);
  const reason = error => ({message: error.message, code: error.code, status: error.status, requestId: error.headers?.get("X-Request-ID"), body: copy(error.body)});
  function replaceAllowed(discard) {
    if(state.phase==="archived")throw new Error("Archived editor is read-only");
    if (locked()) throw new Error("Operation pending or unresolved");
    if (state.draft && !discard) throw new Error("Explicit discard confirmation required");
  }
  return {
    snapshot: () => copy(state),
    invalidateCapabilities(observation){
      if(!["review","blocked"].includes(state.phase)||!observedCapabilityChange(state.capabilities,observation))return false;
      generation++;state={...state,phase:"preview-error",preview:undefined,error:{code:"capabilities_changed",message:"Observed capabilities changed or became unavailable"}};
      return true;
    },
    async load(name, {discard = false} = {}) {
      replaceAllowed(discard);
      const current = ++generation;
      state = {phase: "loading", name};
      try {
        const response = await api.request(endpoint());
        if (current !== generation) return;
        const document = response.body.document;
        const etag = response.headers.get("ETag");
        if (!document || !validETag(etag) || response.body.queue !== name || document.metadata?.name !== name) {
          state = {phase: "uneditable", name, error: {message: response.body.document_error || "Missing canonical document, original ETag or matching identity"}};
          return;
        }
        state = {phase: "editing", name, create: false, etag, base: copy(document), draft: copy(document)};
      } catch (error) { if (current === generation) state = {phase: "load-error", name, error: reason(error)}; }
    },
    async editNext() {
      if (state.phase !== "accepted" || state.create) throw new Error("Only an accepted edit can start another edit");
      const current = ++generation;
      const accepted = copy(state);
      state = {...state, phase: "reading-next", nextEditError: undefined};
      try {
        const response = await api.request(endpoint());
        if (current !== generation) return;
        const document = response.body?.document, etag = response.headers.get("ETag");
        if (!document || !validETag(etag) || response.body.queue !== accepted.name || document.metadata?.name !== accepted.name) {
          throw new Error("Latest declaration cannot be safely edited");
        }
        const receipt = {requestId: accepted.requestId, responseRequestId: accepted.responseRequestId,
          originalETag: accepted.etag, returnedETag: accepted.returnedETag,
          document: accepted.draft, submittedPlan: accepted.submittedPlan, result: accepted.result};
        state = {phase: "editing", name: accepted.name, create: false, etag, base: copy(document), draft: copy(document),
          acceptedOperations: [...(accepted.acceptedOperations ?? []), receipt]};
      } catch (error) {
        if (current === generation) state = {...accepted, nextEditError: reason(error)};
      }
    },
    create(document, {discard = false} = {}) {
      replaceAllowed(discard);
      if (!document?.metadata?.name) throw new Error("Queue name required");
      const draft = copy(document);
      generation++;
      state = {phase: "editing", name: draft.metadata.name, create: true, draft};
    },
    edit(document) {
      if (!editable.has(state.phase)) throw new Error("Editor is not editable");
      if (document?.metadata?.name !== state.name) throw new Error("Queue identity cannot change in this editor");
      state = {...state, phase: "editing", draft: copy(document), preview: undefined, comparison: undefined, error: undefined};
    },
    async readConflict() {
      if (state.phase !== "conflict") throw new Error("Only a known conflict can be compared");
      const current = generation;
      state = {...state, phase: "reading-conflict", comparison: undefined, error: undefined};
      try {
        const response = await api.request(endpoint());
        if (current !== generation) return;
        const document = response.body.document;
        const etag = response.headers.get("ETag");
        if (!document || !validETag(etag) || response.body.queue !== state.name || document.metadata?.name !== state.name) {
          throw new Error(response.body.document_error || "Latest declaration cannot be safely edited");
        }
        // Reading latest state is evidence only. Keep original base and local
        // draft untouched until the user supplies an explicitly reviewed draft.
        state = {...state, phase: "conflict", comparison: {document: copy(document), etag}};
      } catch (error) {
        if (current === generation) state = {...state, phase: "conflict", error: reason(error)};
      }
    },
    rebase(document, {confirmed = false} = {}) {
      if (!confirmed || state.phase !== "conflict" || !state.comparison) throw new Error("Read current conflict evidence and explicitly confirm the merged draft");
      if (document?.metadata?.name !== state.name) throw new Error("Queue identity cannot change during conflict recovery");
      const draft = copy(document);
      const base = copy(state.comparison.document);
      state = {...state, phase: "editing", create: false, base, draft, etag: state.comparison.etag, comparison: undefined, preview: undefined, error: undefined};
    },
    async preview() {
      if (!editable.has(state.phase)) throw new Error("Editor cannot preview");
      const current = generation;
      state = {...state, phase: "previewing", preview: undefined, comparison: undefined, error: undefined, capabilities:undefined};
      try {
        const capabilities=requireCapabilities?await readMutationCapabilities(api,["queue-preview","conditional-queue-writes"]):undefined;
        if(current!==generation)return;
        const response = await api.request(`${endpoint()}/preview`, {method: "POST", body: copy(state.draft), headers: {...headers(),...capabilityHeaders(capabilities)}});
        if (current !== generation) return;
        const preview = response.body;
        if (preview.plan?.queue !== state.name || preview.result?.queue !== state.name ||
            typeof preview.result?.blocked !== "boolean" || !["ready", "noop", "blocked"].includes(preview.result?.status) ||
            preview.result.blocked !== (preview.result.status === "blocked") ||
            preview.create_only !== state.create || (!state.create && preview.base_revision !== state.etag)) {
          throw new Error("Preview identity or original revision does not match this editor");
        }
        if (previewOperations(preview.result) === null || declarationReview({...state, preview}).status === "invalid" ||
            preview.result.operations.some(operation => operation.blocked) !== preview.result.blocked) {
          throw Object.assign(new Error("Preview review data is invalid or inconsistent"), {code: "invalid_preview"});
        }
        if(requireCapabilities)await readMutationCapabilities(api,["queue-preview","conditional-queue-writes"],capabilities);
        if(current!==generation)return;
        state = {...state, phase: preview.result.blocked ? "blocked" : "review", preview: copy(preview),capabilities};
      } catch (error) {
        if (current === generation) state = {...state, phase: error.status === 409 ? "conflict" : [401, 403].includes(error.status) ? "denied" : "preview-error", error: reason(error)};
      }
    },
    async apply() {
      if (state.phase !== "review" || !state.preview || state.preview.result.blocked) throw new Error("A current unblocked preview and explicit review are required");
      const current = generation;
      let dispatched=false;
      state = {...state, phase: "submitting", error: undefined, inspection: undefined};
      try {
        if(requireCapabilities){
          if(!state.capabilities)throw Object.assign(new Error("Missing preview capabilities"),{code:"capabilities_unavailable"});
          await readMutationCapabilities(api,["queue-preview","conditional-queue-writes"],state.capabilities);
        }
        if(current!==generation)return;
        const requestId = createRequestID();
        state={...state,requestId,submittedPlan:copy(state.preview.plan)};dispatched=true;
        const response = await api.request(endpoint(), {method: "PUT", body: copy(state.draft), headers: {...headers(),...capabilityHeaders(state.capabilities), "X-Request-ID": requestId}});
        if (current !== generation) return;
        if (response.body?.queue !== state.name || !["ready", "noop"].includes(response.body?.status) || response.body?.blocked !== false) {
          throw new Error("Unrecognized apply result; inspect resource state before any further write");
        }
        // Successful apply is not observation/convergence. Missing returned
        // ETag must never be invented, incremented, or reused for another write.
        state = {...state, phase: "accepted", result: copy(response.body), responseRequestId: response.headers.get("X-Request-ID"), returnedETag: response.headers.get("ETag") || null, preview: undefined};
      } catch (error) {
        if (current !== generation) return;
        if(!dispatched){state={...state,phase:"preview-error",preview:undefined,error:reason(error)};return;}
        // Browsers may transparently replay PUT after a pre-header reset.
        // A final 409 (or another rejection) can describe the replay, not the
        // original attempt that already committed. Without request-level
        // outcome proof, HTTP status alone cannot authorize rebase/retry.
        state = {...state, phase: "uncertain", preview: undefined, error: reason(error)};
      }
    },
    async inspectUncertain() {
      if (state.phase !== "uncertain") throw new Error("Only an unresolved write can be inspected");
      const current = generation;
      const name = state.name;
      const url = endpoint();
      state = {...state, phase: "inspecting"};
      const read = async (path, collection) => {
        try {
          const response = await api.request(path);
          if (response.body?.queue !== name || (collection && !Array.isArray(response.body.items))) throw new Error("Inspection response identity or shape mismatch");
          return {status: "available", body: copy(response.body), etag: response.headers.get("ETag"), readAt: new Date().toISOString()};
        } catch (error) {
          return {status: error.kind === "http" && error.status === 404 && error.code === "not_found" ? "missing" : "unavailable", error: reason(error), readAt: new Date().toISOString()};
        }
      };
      const requestID = state.requestId;
      const auditRead = async () => {
        try {
          const response = await api.request(`/api/v1/audit/requests/${encodeURIComponent(requestID)}`);
          validateAuditWindow(response.body, requestID);
          const readAt = new Date().toISOString();
          return {status: "available", body: copy(response.body), readAt, windows: [{body: copy(response.body), readAt}]};
        } catch (error) { return {status: "unavailable", error: reason(error), readAt: new Date().toISOString()}; }
      };
      const [declaration, consumers, audit] = await Promise.all([read(url, false), read(`${url}/consumers?limit=200`, true), auditRead()]);
      if (current !== generation) return;
      // Evidence is not causal attribution: matching declaration/observations
      // do not prove this request completed or that its audit outcome exists.
      state = {...state, phase: "uncertain", inspection: {declaration, consumers, audit}};
    },
    async inspectOlderAudit() {
      if (state.phase !== "uncertain") throw new Error("Only unresolved writes can inspect older evidence");
      const audit = state.inspection?.audit;
      const cursor = audit?.windows?.at(-1)?.body.nextBefore;
      if (audit?.status !== "available" || cursor === null || cursor === undefined) throw new Error("No unread audit cursor");
      const current = generation, requestID = state.requestId;
      state = {...state, phase: "inspecting", inspection: {...state.inspection, audit: {...audit, olderError: undefined}}};
      try {
        const response = await api.request(`/api/v1/audit/requests/${encodeURIComponent(requestID)}?before=${cursor}`);
        if (current !== generation) return;
        validateAuditWindow(response.body, requestID, cursor);
        state = {...state, phase: "uncertain", inspection: {...state.inspection, audit: {...audit, olderError: undefined, windows: [...audit.windows, {body: copy(response.body), readAt: new Date().toISOString()}]}}};
      } catch (error) {
        if (current === generation) state = {...state, phase: "uncertain", inspection: {...state.inspection, audit: {...audit, olderError: reason(error)}}};
      }
    },
    archive() {
      if(!canArchiveEditor(state))throw new Error("Pending or unresolved editor cannot be archived");
      generation++;state={...state,phase:"archived",archivedFrom:state.phase};
    },
    clear({confirmed = false} = {}) {
      if (!confirmed) throw new Error("Explicit confirmation required; drafts and unresolved result evidence will be discarded");
      generation++; api.clearToken(); state = {phase: "idle"};
    },
  };
}
