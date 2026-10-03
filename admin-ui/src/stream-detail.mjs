import {latestRead} from "./api.mjs";
import {streamDetailURL, consumerDetailURL} from "./routes.mjs";

// Separate reads: configuration/state and Consumer membership are not an atomic snapshot.
export function createStreamRead(api, name, consumers = false, {retainOnRefresh=false}={}) {
  streamDetailURL(name);
  const reader = latestRead(api), listeners = new Set();
  let state = {phase: "idle", resource: null, failure: null, readAt: null};
  let lastQuery=null;
  const emit = next => {state = next; for (const fn of listeners) fn();};
  return {
    snapshot: () => state,
    subscribe(fn) {listeners.add(fn); return () => listeners.delete(fn);},
    clear() {reader.cancel(); emit({phase: "idle", resource: null, failure: null, readAt: null});},
    async load({offset = 0, limit = 50, q = "", mode = "", order = "asc"} = {}) {
      if (!Number.isSafeInteger(offset) || offset < 0 || !Number.isSafeInteger(limit) || limit < 1 || limit > 200) throw new TypeError("Invalid pagination");
      if (typeof q !== "string" || !["", "pull", "push"].includes(mode) || !["asc", "desc"].includes(order)) throw new TypeError("Invalid query");
      reader.cancel();const query=JSON.stringify({offset,limit,q,mode,order});
      const previous=retainOnRefresh&&query===lastQuery?{resource:state.resource,readAt:state.readAt}:{resource:null,readAt:null};
      lastQuery=query;emit({phase: "loading", ...previous, failure: null});
      try {
        const result = await reader.run(`/api/v1/streams/${encodeURIComponent(name)}${consumers ? `/consumers?${new URLSearchParams({offset, limit, q, mode, order, sort: "name"})}` : ""}`);
        if (result.stale) return;
        const resource = result.result.body;
        if (consumers) {
          if (!resource || !Array.isArray(resource.items) || !Number.isSafeInteger(resource.total) || resource.total < 0 ||
            resource.offset !== Math.min(offset, resource.total) || resource.limit !== limit ||
            resource.items.length !== Math.min(limit, resource.total - resource.offset)) throw new Error("invalid-resource");
          const names = new Set();
          for (const row of resource.items) {
            if (!row || row.stream !== name || typeof row.name !== "string" || names.has(row.name) || !["pull", "push"].includes(row.mode)) throw new Error("invalid-resource");
            try {consumerDetailURL(name, row.name);} catch {throw new Error("invalid-resource");}
            names.add(row.name);
          }
        } else if (!resource || resource.name !== name || !(resource.subjects === null || Array.isArray(resource.subjects) && resource.subjects.every(v => typeof v === "string"))) throw new Error("invalid-resource");
        emit({phase: "ready", resource, failure: null, readAt: new Date().toISOString()});
      } catch (error) {
        const failure = error.status === 404 && error.code === "not_found" ? "missing" :
          error.status === 401 || error.status === 403 ? "denied" : error.status === 404 && error.code === "read_api_disabled" ? "disabled" : error.status === 400 ? "invalid-query" : error.message === "invalid-resource" || error.kind === "invalid-response" ? "invalid" : "unavailable";
        emit({phase: "error", ...(failure==="unavailable"?previous:{resource:null,readAt:null}), failure});
      }
    },
  };
}
