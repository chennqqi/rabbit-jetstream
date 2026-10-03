import {latestRead} from "./api.mjs";
import {consumerDetailURL,streamDetailURL} from "./routes.mjs";

const ownership = ["matching", "different", "unmarked", "unknown"];
export function validateConsumerPage(page, queue, query) {
  if (!page || page.queue !== queue || typeof page.stream !== "string" || !page.stream ||
      typeof page.declaration_revision !== "string" || !/^"\d+"$/.test(page.declaration_revision) ||
      !["present", "missing"].includes(page.stream_status) || !ownership.includes(page.stream_ownership) ||
      !Array.isArray(page.items) || !Number.isSafeInteger(page.total) || page.total < 0 ||
      page.offset !== Math.min(query.offset, page.total) || page.limit !== query.limit ||
      page.items.length !== Math.min(page.limit, page.total - page.offset)) throw new Error("invalid-page");
  try{streamDetailURL(page.stream);}catch{throw new Error("invalid-page");}
  const names = new Set();
  for (const row of page.items) {
    if (!row || row.stream !== page.stream || typeof row.name !== "string" || !row.name || names.has(row.name) ||
        !ownership.includes(row.ownership) || !["present", "missing"].includes(row.status) ||
        !(row.expected === null || (row.expected && row.expected.name === row.name && row.expected.stream === row.stream)) ||
        !(row.observed === null || (row.observed && row.observed.name === row.name && row.observed.stream === row.stream)) ||
        (row.status === "missing") !== (row.observed === null) || (!row.expected && !row.observed) ||
        (page.stream_status === "missing" && row.observed !== null)) throw new Error("invalid-page");
    names.add(row.name);
    try{consumerDetailURL(row.stream,row.name);}catch{throw new Error("invalid-page");}
    for (const [resource, field] of [[row.expected, "filterSubjects"], [row.observed, "filter_subjects"]]) {
      if (!resource) continue;
      const filters = resource[field];
      if (!["pull", "push"].includes(resource.mode) || !(filters === null || (Array.isArray(filters) && filters.every(value => typeof value === "string")))) throw new Error("invalid-page");
    }
    if (row.observed?.filter_subject !== undefined && typeof row.observed.filter_subject !== "string") throw new Error("invalid-page");
  }
  return page;
}

export function consumerCounter(row, field) {
  const value = row.observed?.[field];
  return typeof value === "bigint" && value >= 0n ? value.toString() :
    Number.isSafeInteger(value) && value >= 0 ? String(value) : null;
}

export function createQueueConsumers(api, queue, {retainOnRefresh=false}={}) {
  const reader = latestRead(api), listeners = new Set();
  let state = {phase: "idle", query: {q: "", mode: "", order: "asc", offset: 0, limit: 50}, page: null, failure: null, readAt: null};
  const emit = next => { state = next; for (const fn of listeners) fn(); };
  return {
    snapshot: () => state,
    subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn); },
    clear() { reader.cancel(); emit({...state, phase: "idle", page: null, failure: null, readAt: null}); },
    async load(changes = {}, {restore = false} = {}) {
      const reset = !restore && ["q", "mode", "order", "limit"].some(k => Object.hasOwn(changes,k) && changes[k] !== state.query[k]);
      const query = {...state.query, ...changes, ...(reset ? {offset: 0} : {})};
      if (typeof query.q !== "string" || !["", "pull", "push"].includes(query.mode) || !["asc", "desc"].includes(query.order) ||
          !Number.isSafeInteger(query.offset) || query.offset < 0 || !Number.isSafeInteger(query.limit) || query.limit < 1 || query.limit > 200) throw new TypeError("Invalid query");
      reader.cancel();
      const sameQuery=["q","mode","order","offset","limit"].every(key=>query[key]===state.query[key]);
      const previous=retainOnRefresh&&sameQuery?{page:state.page,readAt:state.readAt}:{page:null,readAt:null};
      emit({phase: "loading", query, ...previous, failure: null});
      try {
        const response = await reader.run(`/api/v1/queues/${encodeURIComponent(queue)}/consumers?${new URLSearchParams(query)}`);
        if (response.stale) return;
        const page = validateConsumerPage(response.result.body, queue, query);
        // Keep the requested offset even when churn clamps the observed page.
        emit({phase: "ready", query, page, failure: null, readAt: new Date().toISOString()});
      } catch (error) {
        const kind = error.status === 401 || error.status === 403 ? "denied" : error.status === 409 ? "changed" :
          error.status === 404 && error.code === "not_found" ? "missing" : error.status === 400 ? "query" :
          error.status === 404 && error.code === "read_api_disabled" ? "disabled" :
          error.message === "invalid-page" || error.kind === "invalid-response" ? "invalid" : "unavailable";
        emit({phase: "error", query, ...(kind==="unavailable"?previous:{page:null,readAt:null}), failure: kind});
      }
    },
  };
}
