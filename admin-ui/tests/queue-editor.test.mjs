import {test} from "node:test";
import assert from "node:assert/strict";
import {createQueueEditor} from "../src/queue-editor.mjs";
import {APIError} from "../src/api.mjs";

const document = () => ({metadata: {name: "orders", labels: {owner: "team"}}, spec: {max: 9223372036854775807n}});
const response = body => ({body, headers: new Headers({ETag: '"7"'})});
function setup() {
  const calls = [];
  let put = async () => response({queue: "orders", status: "ready", blocked: false});
  let previewError = null;
  const api = {clearToken() { calls.push({clear: true}); }, async request(path, options = {}) {
    calls.push({path, ...options});
    if (options.method === "PUT") return put();
    if (options.method === "POST") { if (previewError) throw previewError; return response({plan: {queue: "orders"}, result: {queue: "orders", status: "ready", blocked: false, operations: []}, create_only: options.headers["If-None-Match"] === "*", base_revision: options.headers["If-Match"]}); }
    return response({queue: "orders", document: document()});
  }};
  return {editor: createQueueEditor(api), calls, api, setPut: fn => { put = fn; }, setPreviewError: error => { previewError = error; }};
}

test("DLQ cycle preview stays editable without rebase; dispatched rejection stays uncertain", async () => {
  const {editor,calls,setPreviewError,setPut}=setup();
  const cycle=new APIError("DLQ dependency cycle at orders",{kind:"http",status:400,code:"dlq_dependency_cycle"});
  await editor.load("orders");setPreviewError(cycle);await editor.preview();
  assert.equal(editor.snapshot().phase,"preview-error");
  assert.equal(editor.snapshot().error.code,"dlq_dependency_cycle");
  assert.deepEqual(editor.snapshot().draft,document());
  assert.equal(editor.snapshot().etag,'"7"');
  await assert.rejects(editor.readConflict());await assert.rejects(editor.apply());
  assert.equal(calls.filter(call=>call.method==="PUT").length,0);
  editor.edit(document());setPreviewError(null);await editor.preview();
  setPut(async()=>{throw cycle;});await editor.apply();
  assert.equal(editor.snapshot().phase,"uncertain");
  await assert.rejects(editor.apply());
  assert.equal(calls.filter(call=>call.method==="PUT").length,1);
});

test("canonical draft, original ETag and exact values survive preview/apply", async () => {
  const {editor, calls} = setup();
  await editor.load("orders");
  const snapshot = editor.snapshot(); snapshot.draft.metadata.labels.owner = "changed";
  assert.equal(editor.snapshot().draft.metadata.labels.owner, "team");
  editor.edit(snapshot.draft); await editor.preview(); await editor.apply();
  assert.equal(editor.snapshot().phase, "accepted");
  assert.deepEqual(calls.map(call => call.method || "GET"), ["GET", "POST", "PUT"]);
  assert.equal(calls[1].headers["If-Match"], '"7"');
  assert.equal(calls[2].headers["If-Match"], '"7"');
  assert.equal(calls[2].body.spec.max, 9223372036854775807n);
});

test("draft changes invalidate preview; duplicate submit cannot dispatch", async () => {
  const {editor, calls, setPut} = setup();
  await editor.load("orders"); await editor.preview(); editor.edit(document());
  await assert.rejects(editor.apply(), /preview/);
  await editor.preview();
  let finish; setPut(() => new Promise(resolve => { finish = resolve; }));
  const applying = editor.apply();
  await assert.rejects(editor.apply(), /preview/);
  finish(response({queue: "orders", status: "noop", blocked: false})); await applying;
  assert.equal(calls.filter(call => call.method === "PUT").length, 1);
});

test("uncertain response preserves draft and blocks replay or navigation", async () => {
  const {editor, setPut} = setup();
  await editor.load("orders"); await editor.preview();
  setPut(async () => { throw new APIError("audit failed", {status: 503, code: "audit_unavailable"}); });
  await editor.apply();
  assert.equal(editor.snapshot().phase, "uncertain");
  assert.equal(editor.snapshot().draft.spec.max, 9223372036854775807n);
  await assert.rejects(editor.apply()); await assert.rejects(editor.load("another", {discard: true}));
  assert.throws(() => editor.clear());
});

test("final write rejection cannot disprove a prior transparent transport replay", async () => {
  for (const status of [400, 401, 403, 409, 428]) {
    const {editor, setPut} = setup(); await editor.load("orders"); await editor.preview();
    setPut(async () => { throw new APIError("rejected", {status, kind: "http", code: status === 409 ? "conflict" : status === 401 ? "unauthorized" : "forbidden"}); }); await editor.apply();
    assert.equal(editor.snapshot().phase, "uncertain");
    assert.equal(editor.snapshot().etag, '"7"'); assert.ok(editor.snapshot().draft);
    await assert.rejects(editor.apply(), /preview/);
    await assert.rejects(editor.readConflict());
  }
});

test("receiving attempt with no effects cannot unlock an unknown PUT", async () => {
  const {editor,setPut,calls}=setup();await editor.load("orders");await editor.preview();
  const mutation={schemaVersion:"rjs.mutation-evidence.v1",scope:"receiving-attempt",phase:"audit_intent",resourceEffects:"none"};
  setPut(async()=>{throw new APIError("audit unavailable",{status:503,body:{error:{mutation}}});});
  await editor.apply();assert.equal(editor.snapshot().phase,"uncertain");
  assert.deepEqual(editor.snapshot().error.body.error.mutation,mutation);
  await assert.rejects(editor.apply());await assert.rejects(editor.readConflict());
  assert.equal(calls.filter(c=>c.method==="PUT").length,1);
});

test("malformed conflict stays uncertain, contradictory preview cannot authorize apply", async () => {
  const {editor, setPut, api} = setup();
  await editor.load("orders"); await editor.preview();
  setPut(async () => { throw new APIError("broken JSON", {status: 409, kind: "invalid-response"}); });
  await editor.apply(); assert.equal(editor.snapshot().phase, "uncertain");
  const other = createQueueEditor(api); await other.load("orders");
  api.request = async () => response({plan: {queue: "orders"}, result: {queue: "orders", status: "blocked", blocked: false, operations: []}, create_only: false, base_revision: '"7"'});
  await other.preview(); assert.equal(other.snapshot().phase, "preview-error"); await assert.rejects(other.apply());
});

test("explicit clear suppresses late results and clears memory token", async () => {
  const {editor, setPut, calls} = setup(); await editor.load("orders"); await editor.preview();
  let finish; setPut(() => new Promise(resolve => { finish = resolve; }));
  const applying = editor.apply(); editor.clear({confirmed: true});
  finish(response({queue: "orders", status: "ready", blocked: false})); await applying;
  assert.deepEqual(editor.snapshot(), {phase: "idle"}); assert.equal(calls.at(-1).clear, true);
});

test("create uses only create precondition, unrepresentable document is not editable", async () => {
  const {editor, calls} = setup(); editor.create(document()); await editor.preview(); await editor.apply();
  assert.equal(calls[0].headers["If-None-Match"], "*"); assert.equal(calls[1].headers["If-Match"], undefined);
  const other = createQueueEditor({request: async () => response({queue: "orders", document: null, document_error: "legacy plan"})});
  await other.load("orders"); assert.equal(other.snapshot().phase, "uneditable"); await assert.rejects(other.preview());
});

test("conflict read preserves three versions until explicit rebase and fresh preview", async () => {
  const {editor, api, calls, setPreviewError} = setup();
  await editor.load("orders");
  const local = editor.snapshot().draft; local.metadata.labels.owner = "local"; editor.edit(local);
  setPreviewError(new APIError("changed", {kind: "http", status: 409, code: "conflict"}));
  await editor.preview(); setPreviewError(null);
  const originalRequest = api.request;
  api.request = async (path, options) => {
    if (!options?.method) { const current = document(); current.metadata.labels.remote = "preserve"; return {body: {queue: "orders", document: current}, headers: new Headers({ETag: '"9007199254740993"'})}; }
    return originalRequest(path, options);
  };
  await editor.readConflict();
  let state = editor.snapshot();
  assert.equal(state.etag, '"7"'); assert.equal(state.base.metadata.labels.owner, "team"); assert.equal(state.draft.metadata.labels.owner, "local");
  assert.equal(state.comparison.document.metadata.labels.remote, "preserve");
  assert.throws(() => editor.rebase(local), /explicitly/);
  const merged = state.comparison.document; merged.metadata.labels.owner = "local";
  editor.rebase(merged, {confirmed: true});
  state = editor.snapshot(); assert.equal(state.etag, '"9007199254740993"'); assert.equal(state.draft.metadata.labels.remote, "preserve");
  await assert.rejects(editor.apply(), /preview/);
  await editor.preview();
  assert.equal(calls.at(-1).headers["If-Match"], '"9007199254740993"');
  assert.equal(calls.filter(call => call.method === "PUT").length, 0);
});

test("failed conflict read retains local draft; uncertain outcome cannot use rebase", async () => {
  const {editor, api, setPreviewError} = setup(); await editor.load("orders");
  setPreviewError(new APIError("changed", {kind: "http", status: 409, code: "conflict"})); await editor.preview();
  api.request = async () => { throw new APIError("missing", {status: 404}); };
  await editor.readConflict();
  assert.equal(editor.snapshot().phase, "conflict"); assert.equal(editor.snapshot().etag, '"7"'); assert.ok(editor.snapshot().draft);
  assert.throws(() => editor.rebase(document(), {confirmed: true}));
  const other = setup(); await other.editor.load("orders"); await other.editor.preview();
  other.setPut(async () => { throw new APIError("unknown", {status: 503}); }); await other.editor.apply();
  await assert.rejects(other.editor.readConflict()); assert.throws(() => other.editor.rebase(document(), {confirmed: true}));
});

test("late conflict read cannot restore cleared session or allow concurrent edits", async () => {
  const {editor, api, setPreviewError} = setup(); await editor.load("orders");
  setPreviewError(new APIError("changed", {kind: "http", status: 409, code: "conflict"})); await editor.preview();
  let finish; api.request = () => new Promise(resolve => { finish = resolve; });
  const reading = editor.readConflict();
  assert.throws(() => editor.edit(document())); await assert.rejects(editor.readConflict());
  editor.clear({confirmed: true}); finish(response({queue: "orders", document: document()})); await reading;
  assert.deepEqual(editor.snapshot(), {phase: "idle"});
});

test("uncertain inspection is read-only, keeps correlation and cannot unlock writes", async () => {
  const {editor, api, calls, setPut} = setup(); await editor.load("orders"); await editor.preview();
  setPut(async () => { throw new APIError("audit failed", {status: 503, kind: "http", code: "audit_unavailable", headers: new Headers({"X-Request-ID": "server-op"}), body: {error: {code: "audit_unavailable"}}}); });
  await editor.apply();
  const id = editor.snapshot().requestId;
  assert.match(id, /^[a-f0-9]{32}$/); assert.equal(calls.at(-1).headers["X-Request-ID"], id);
  assert.equal(editor.snapshot().error.requestId, "server-op");
  const reads = [];
  api.request = async (path, options) => {
    reads.push({path, options});
    if (path.includes("audit/requests")) return response({requestId: id, items: [{id: "event", requestId: id, phase: "outcome", sequence: 1}], streamPresent: true, firstSequence: 1, lastSequence: 1, scanned: 1, missing: 0, nextBefore: null});
    if (path.includes("consumers")) return response({queue: "orders", items: [], total: 0});
    return response({queue: "orders", document: document()});
  };
  await editor.inspectUncertain();
  const state = editor.snapshot();
  assert.equal(state.phase, "uncertain"); assert.equal(state.requestId, id); assert.equal(state.etag, '"7"');
  assert.equal(state.inspection.declaration.status, "available"); assert.equal(state.inspection.consumers.status, "available");
  assert.equal(reads.length, 3); assert.ok(reads.every(read => !read.options?.method));
  assert.equal(state.inspection.audit.status, "available");
  await assert.rejects(editor.apply()); assert.throws(() => editor.rebase(document(), {confirmed: true}));
});

test("inspection failures stay distinct; clearing suppresses late evidence", async () => {
  const {editor, api, setPut} = setup(); await editor.load("orders"); await editor.preview();
  setPut(async () => { throw new APIError("connection lost", {kind: "network"}); }); await editor.apply();
  api.request = async path => { throw new APIError("cannot read", {kind: "http", status: path.includes("consumers") ? 503 : 404, code: path.includes("consumers") ? "jetstream_unavailable" : "not_found"}); };
  await editor.inspectUncertain();
  assert.equal(editor.snapshot().inspection.declaration.status, "missing"); assert.equal(editor.snapshot().inspection.consumers.status, "unavailable");
  const pending = []; api.request = () => new Promise(resolve => pending.push(resolve));
  const reading = editor.inspectUncertain(); await assert.rejects(editor.inspectUncertain());
  editor.clear({confirmed: true}); pending.forEach(resolve => resolve(response({queue: "orders", items: []}))); await reading;
  assert.deepEqual(editor.snapshot(), {phase: "idle"});
});
