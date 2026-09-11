import {readAuditQuery} from "./audit-query.mjs";
export const defaultQueueQuery = Object.freeze({q: "", order: "asc", offset: 0, limit: 50});

export function readQueueQuery(search) {
  const params = new URLSearchParams(search);
  for (const [key] of params) {
    if (!["q", "order", "offset", "limit"].includes(key) || params.getAll(key).length !== 1) throw new TypeError("Invalid list URL");
  }
  const integer = (key, fallback, min, max) => {
    const raw = params.get(key);
    if (raw === null) return fallback;
    if (!/^\d+$/.test(raw)) throw new TypeError("Invalid list URL");
    const value = Number(raw);
    if (!Number.isSafeInteger(value) || value < min || value > max) throw new TypeError("Invalid list URL");
    return value;
  };
  const order = params.get("order") ?? "asc";
  if (!["asc", "desc"].includes(order)) throw new TypeError("Invalid list URL");
  return {q: params.get("q") ?? "", order, offset: integer("offset", 0, 0, Number.MAX_SAFE_INTEGER), limit: integer("limit", 50, 1, 200)};
}

export function queueListURL(query = defaultQueueQuery) {
  const params = new URLSearchParams(query);
  readQueueQuery(params.toString());
  return `/admin/queues?${params}`;
}

export function queueDetailURL(name) {
  if (typeof name !== "string" || !name || name === "." || name === ".." || /[/\\\u0000-\u001f]/.test(name)) throw new TypeError("Invalid Queue name");
  return `/admin/queues/by-name/${encodeURIComponent(name)}`;
}

export function consumerDetailURL(stream, name) {
  for (const value of [stream, name]) queueDetailURL(value);
  return `/admin/streams/${encodeURIComponent(stream)}/consumers/${encodeURIComponent(name)}`;
}

export function streamListURL(query = defaultQueueQuery) {
  return queueListURL(query).replace('/admin/queues?', '/admin/streams?');
}

export function streamDetailURL(name) {
  queueDetailURL(name);
  return `/admin/streams/${encodeURIComponent(name)}`;
}

export const defaultGlobalConsumerQuery=Object.freeze({q:"",queue:"",stream:"",mode:"",state:"",order:"asc",offset:0,limit:50,generation:""});
export function readGlobalConsumerQuery(search=""){
  const params=new URLSearchParams(search);
  for(const [key] of params)if(!Object.hasOwn(defaultGlobalConsumerQuery,key)||params.getAll(key).length!==1)throw new TypeError("Invalid global Consumer URL");
  const base=readQueueQuery(new URLSearchParams([...params].filter(([key])=>["q","order","offset","limit"].includes(key))).toString());
  const result={...defaultGlobalConsumerQuery,...base};
  for(const key of ["queue","stream","generation"]){const value=params.get(key)??"";if(value.length>256||/[\u0000-\u001f]/.test(value))throw new TypeError("Invalid global Consumer filter");result[key]=value;}
  result.mode=params.get("mode")??"";if(!["","pull","push"].includes(result.mode))throw new TypeError("Invalid global Consumer mode");
  result.state=params.get("state")??"";if(!["","present","missing","mismatched"].includes(result.state))throw new TypeError("Invalid global Consumer state");
  return result;
}
export function globalConsumersURL(query=defaultGlobalConsumerQuery){const params=new URLSearchParams();for(const [key,value] of Object.entries(query))if(value!=="")params.set(key,String(value));readGlobalConsumerQuery(params.toString());return `/admin/consumers?${params}`;}

export function nodeDetailURL(id) {
  queueDetailURL(id);
  return `/admin/nodes/${encodeURIComponent(id)}`;
}

export function readConnectionQuery(search="") {
  const params=new URLSearchParams(search);
  for(const [key] of params)if(!["offset","limit","cid"].includes(key)||params.getAll(key).length!==1)throw new TypeError("Invalid connection query");
  const value=readQueueQuery(new URLSearchParams([...params].filter(([key])=>key!=="cid")).toString());
  if(value.offset>1000000)throw new TypeError("Connection offset exceeds limit");
  const cid=params.get("cid");if(cid!==null&&(!/^\d+$/.test(cid)||BigInt(cid)<1n||BigInt(cid)>18446744073709551615n||value.offset!==0))throw new TypeError("Invalid connection CID filter");
  return {offset:value.offset,limit:value.limit,...(cid===null?{}:{cid:String(BigInt(cid))})};
}
export function nodeConnectionsURL(id,query={offset:0,limit:50}) {
  nodeDetailURL(id);
  if(new TextEncoder().encode(id).length>256||/[\s\p{Cc}]/u.test(id))throw new TypeError("Invalid node identity");
  const params=new URLSearchParams(query);readConnectionQuery(params.toString());
  return `${nodeDetailURL(id)}/connections?${params}`;
}
export function nodeConnectionURL(id,cid,query={offset:0,limit:50}) {
  nodeConnectionsURL(id,query);
  if(!["string","number","bigint"].includes(typeof cid)||typeof cid==="number"&&!Number.isSafeInteger(cid)||!/^\d+$/.test(String(cid))||BigInt(cid)<1n||BigInt(cid)>18446744073709551615n)throw new TypeError("Invalid connection identity");
  return `${nodeDetailURL(id)}/connections/${BigInt(cid)}?${new URLSearchParams(query)}`;
}

export function readConsumerQuery(search = "") {
  const params = new URLSearchParams(search), list = new URLSearchParams();
  for (const [key, value] of params) {
    if (!["cq", "cmode", "corder", "coffset", "climit"].includes(key) || params.getAll(key).length !== 1) throw new TypeError("Invalid Consumer URL");
    if (key !== "cmode") list.set(key.slice(1), value);
  }
  const mode = params.get("cmode") ?? "";
  if (!["", "pull", "push"].includes(mode)) throw new TypeError("Invalid Consumer mode");
  return {...readQueueQuery(list.toString()), mode};
}

export function queueConsumersURL(name, query) {
  const params = new URLSearchParams(Object.entries(query).map(([key,value]) => [`c${key}`,value]));
  readConsumerQuery(params.toString());
  return `${queueDetailURL(name)}?${params}`;
}

export const queueTabs=["summary","configuration","routing","consumers","events"];
export function queueTabURL(name,tab,consumerQuery,eventBefore) {
  if(!queueTabs.includes(tab))throw new TypeError("Invalid Queue tab");
  const params=new URLSearchParams(consumerQuery?Object.entries(consumerQuery).map(([key,value])=>[`c${key}`,value]):[]);
  if(eventBefore!==undefined&&eventBefore!==null){if(!/^\d+$/.test(String(eventBefore))||BigInt(eventBefore)>18446744073709551615n)throw new TypeError("Invalid event cursor");params.set("ebefore",String(eventBefore));}
  params.set("tab",tab);return `${queueDetailURL(name)}?${params}`;
}

export function legacyRouteURL(location) {
  if(!["/admin/","/admin/index.html"].includes(location.pathname)||location.search)return null;
  return new Map([["#overview","/admin/overview"],["#queues","/admin/queues"],["#nodes","/admin/nodes"]]).get(location.hash)??null;
}

function readBaseRoute(location) {
  const pathname=legacyRouteURL(location)??location.pathname, search=location.search??"";
  try {
    if (pathname === "/admin/" || pathname === "/admin/queues") return {kind: "queues", query: readQueueQuery(search)};
    if (pathname === "/admin/streams") return {kind: "streams", query: readQueueQuery(search)};
    if (pathname === "/admin/consumers") return {kind:"consumers",query:readGlobalConsumerQuery(search)};
    if (pathname === "/admin/audit") return {kind: "audit", query: readAuditQuery(search)};
    if (pathname === "/admin/queues/new") return search ? {kind: "invalid"} : {kind: "create-queue"};
    if (pathname === "/admin/queues/bulk-change") return search ? {kind: "invalid"} : {kind: "bulk-change"};
    const match = /^\/admin\/queues\/by-name\/([^/]+)(\/edit|\/delete)?$/.exec(pathname);
    if (match) {
      const name = decodeURIComponent(match[1]);
      if (!name || name === "." || name === ".." || /[/\\\u0000-\u001f]/.test(name)) return {kind: "invalid"};
      if (match[2]) return search || (match[2]==="/delete"&&!/^[A-Za-z0-9_-]+$/.test(name)) ? {kind: "invalid"} : {kind: match[2]==="/delete"?"delete-queue":"edit-queue", name};
      const params=new URLSearchParams(search),tab=params.get("tab");
      if(params.getAll("tab").length>1 || tab!==null&&!queueTabs.includes(tab))return {kind:"invalid"};
      params.delete("tab");
      const eventBefore=params.get("ebefore");
      if(params.getAll("ebefore").length>1||eventBefore!==null&&(!/^\d+$/.test(eventBefore)||BigInt(eventBefore)>18446744073709551615n))return {kind:"invalid"};
      params.delete("ebefore");
      return {kind: "queue", name, ...(tab!==null?{tab}:{}), ...(eventBefore!==null?{eventBefore}:{}), ...(params.size ? {consumerQuery: readConsumerQuery(params.toString())} : {})};
    }
    const streamMatch = /^\/admin\/streams\/([^/]+)$/.exec(pathname);
    if (streamMatch) {
      const name = decodeURIComponent(streamMatch[1]);
      streamDetailURL(name);
      const params = new URLSearchParams(search);
      const query = readConsumerQuery(new URLSearchParams([...params].map(([key, value]) => [`c${key}`, value])).toString());
      return {kind: "stream", name, query};
    }
    const connection=/^\/admin\/nodes\/([^/]+)\/connections\/([^/]+)$/.exec(pathname);
    if(connection){const id=decodeURIComponent(connection[1]),cid=decodeURIComponent(connection[2]),query=readConnectionQuery(search);nodeConnectionURL(id,cid,query);return {kind:"node-connection",id,cid:String(BigInt(cid)),query};}
    const connections=/^\/admin\/nodes\/([^/]+)\/connections$/.exec(pathname);
    if(connections){const id=decodeURIComponent(connections[1]),query=readConnectionQuery(search);nodeConnectionsURL(id,query);return {kind:"node-connections",id,query};}
    if (search) return {kind: "invalid"};
    if (pathname === "/admin/settings") return {kind:"settings"};
    if (pathname === "/admin/access") return {kind:"access"};
    if (pathname === "/admin/diagnostics") return search?{kind:"invalid"}:{kind:"diagnostics"};
    if (pathname === "/admin/alerts") return search?{kind:"invalid"}:{kind:"alerts"};
    if (pathname === "/admin/compatibility") return search?{kind:"invalid"}:{kind:"compatibility"};
    if (pathname === "/admin/overview") return {kind: "overview"};
    if (pathname === "/admin/nodes") return {kind: "nodes"};
    const node = /^\/admin\/nodes\/([^/]+)$/.exec(pathname);
    if (node) {
      const id = decodeURIComponent(node[1]); nodeDetailURL(id);
      return {kind: "node", id};
    }
    const consumer = /^\/admin\/streams\/([^/]+)\/consumers\/([^/]+)$/.exec(pathname);
    if (consumer) {
      const stream = decodeURIComponent(consumer[1]), name = decodeURIComponent(consumer[2]);
      consumerDetailURL(stream, name);
      return {kind: "consumer", stream, name};
    }
    return {kind: "not-found"};
  } catch { return {kind: "invalid"}; }
}

export function readRoute(location) {
  const match=/^\/admin\/tenants\/([^/]+)(\/.*)?$/.exec(location.pathname);
  if(!match)return readBaseRoute(location);
  try {
    const tenant=decodeURIComponent(match[1]);
    if(!/^[A-Za-z0-9._-]{1,128}$/.test(tenant)||encodeURIComponent(tenant)!==match[1])return {kind:"invalid"};
    const route=readBaseRoute({pathname:`/admin${match[2]??"/"}`,search:location.search??"",hash:location.hash??""});
    return ["invalid","not-found"].includes(route.kind)?route:{...route,tenant};
  } catch { return {kind:"invalid"}; }
}

export function createRouter(browser = window) {
  const read=()=>{
    const legacy=legacyRouteURL(browser.location);
    if(legacy)browser.history.replaceState(browser.history.state??null,"",legacy);
    return readRoute(browser.location);
  };
  let state = read();
  const listeners = new Set();
  const changed = () => { state = read(); for (const listener of listeners) listener(); };
  return {
    snapshot: () => state,
    subscribe(listener) {
      if (listeners.size === 0) {browser.addEventListener("popstate", changed);browser.addEventListener("hashchange", changed);}
      listeners.add(listener);
      return () => { listeners.delete(listener); if (listeners.size === 0) {browser.removeEventListener("popstate", changed);browser.removeEventListener("hashchange", changed);} };
    },
    navigate(path, {replace = false} = {}) {
      const target = new URL(path, browser.location.origin);
      if(state.tenant&&!target.pathname.startsWith("/admin/tenants/"))target.pathname=`/admin/tenants/${encodeURIComponent(state.tenant)}${target.pathname.slice("/admin".length)}`;
      if (target.origin !== browser.location.origin || !target.pathname.startsWith("/admin/") || target.username || target.password || target.hash || readRoute(target).kind === "invalid") throw new TypeError("Invalid navigation");
      if (`${browser.location.pathname}${browser.location.search}` === `${target.pathname}${target.search}`) return false;
      browser.history[replace ? "replaceState" : "pushState"](null, "", `${target.pathname}${target.search}`);
      changed();
      return true;
    },
    setTenant(tenant,{replace=true}={}) {
      if(!/^[A-Za-z0-9._-]{1,128}$/.test(tenant))throw new TypeError("Invalid tenant identity");
      const current=new URL(browser.location.href);
      const existing=/^\/admin\/tenants\/[^/]+(\/.*)?$/.exec(current.pathname);
      const inner=existing?existing[1]??"/":current.pathname.slice("/admin".length)||"/";
      const target=`/admin/tenants/${encodeURIComponent(tenant)}${inner}${current.search}`;
      if(`${current.pathname}${current.search}`===target)return false;
      browser.history[replace?"replaceState":"pushState"](null,"",target);changed();return true;
    },
  };
}
