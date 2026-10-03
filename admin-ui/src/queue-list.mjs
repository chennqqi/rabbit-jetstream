import {latestRead} from "./api.mjs";

// These are desired declaration values, not live capacity or convergence
// evidence. Never display them when the embedded Plan contradicts its owner.
export function queueDeployment(item) {
  const plan=item?.plan,stream=plan?.stream;
  if(plan?.queue!==item?.queue||plan?.revision!==item?.revision||
      !["file","memory"].includes(stream?.storage)||
      !Number.isSafeInteger(stream?.replicas)||stream.replicas<1)return null;
  return {storage:stream.storage,replicas:stream.replicas};
}

const observationStates=new Set(["present","missing","degraded","unavailable"]);
const observationReasons=new Set(["declaration_inconsistent","stream_collection_unavailable","stream_missing","stream_identity_ambiguous","stream_configuration_mismatch","stream_replicas_not_current"]);
const exactCount=value=>typeof value==="bigint"&&value>=0n||Number.isSafeInteger(value)&&value>=0;
export function queueObservation(item){
  const value=item?.observation;
  if(value===undefined)return null;
  if(!value||!observationStates.has(value.state)||!Array.isArray(value.reasons)||value.reasons.some(reason=>!observationReasons.has(reason))||new Set(value.reasons).size!==value.reasons.length||
    value.stream!==undefined&&(typeof value.stream!=="string"||!value.stream)||value.messages!==undefined&&!exactCount(value.messages)||value.consumers!==undefined&&!Number.isSafeInteger(value.consumers)||value.consumers!==undefined&&value.consumers<0||
    ["missing","unavailable"].includes(value.state)&&(value.messages!==undefined||value.consumers!==undefined)||["present","degraded"].includes(value.state)&&(value.messages===undefined||value.consumers===undefined))throw new Error("invalid-observation");
  return {state:value.state,stream:value.stream,reasons:[...value.reasons],messages:value.messages,consumers:value.consumers};
}

export function createQueueList(api, resource = "queues") {
  if (!["queues", "streams"].includes(resource)) throw new TypeError("Invalid collection");
  const identity = resource === "queues" ? "queue" : "name";
  const reader = latestRead(api);
  const listeners = new Set();
  let state = {phase: "idle", query: {q: "", order: "asc", offset: 0, limit: 50}, page: null, failure: null, readAt: null};
  const emit = next => { state = next; for (const listener of listeners) listener(); };
  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    clear() { reader.cancel(); emit({...state, phase: "idle", page: null, failure: null, readAt: null}); },
    async load(changes = {}, {restore = false} = {}) {
      // A new filter/order/page size always starts at the first page.
      const reset = !restore && ["q", "order", "limit"].some(key => Object.hasOwn(changes, key) && changes[key] !== state.query[key]);
      const query = {...state.query, ...changes, ...(reset ? {offset: 0} : {})};
      if (typeof query.q !== "string" || !["asc", "desc"].includes(query.order) ||
          !Number.isSafeInteger(query.offset) || query.offset < 0 ||
          !Number.isSafeInteger(query.limit) || query.limit < 1 || query.limit > 200) throw new TypeError("Invalid list query");
      reader.cancel();
      const sameQuery=["q","order","offset","limit"].every(key=>query[key]===state.query[key]);
      const previous=sameQuery?{page:state.page,readAt:state.readAt}:{page:null,readAt:null};
      emit({phase: "loading", query, ...previous, failure: null});
      try {
        const params = new URLSearchParams({...query, sort: "name"});
        const response = await reader.run(`/api/v1/${resource}?${params}`);
        if (response.stale) return;
        const page = response.result.body;
        if (!page || !Array.isArray(page.items) || !Number.isSafeInteger(page.total) || page.total < 0 ||
            !Number.isSafeInteger(page.offset) || page.offset !== Math.min(query.offset, page.total) || page.limit !== query.limit ||
            page.items.length !== Math.min(page.limit, page.total - page.offset) ||
            page.items.some(item => !item || typeof item[identity] !== "string" || !item[identity] ||
              /[/\\\u0000-\u001f]/.test(item[identity]) || [".", ".."].includes(item[identity]) ||
              resource === "queues" && typeof item.revision !== "string") ||
            new Set(page.items.map(item => item[identity])).size !== page.items.length) throw new Error("invalid-page");
        if(resource==="queues")for(const item of page.items)queueObservation(item);
        // The response offset describes the observed page, not a new query.
        emit({phase: "ready", query, page, failure: null, readAt: new Date().toISOString()});
      } catch (error) {
        const kind = error.status === 401 ? "credentials-rejected" : error.status === 403 ? "role-denied" :
          error.status === 400 ? "invalid-query" : error.status === 404 && error.code === "read_api_disabled" ? "auth-disabled" :
          error.message === "invalid-page" ? "invalid-response" : "unavailable";
        const retained=["unavailable","invalid-response"].includes(kind)?previous:{page:null,readAt:null};
        emit({phase: "error", query, ...retained, failure: {kind, status: error.status}});
      }
    },
  };
}
