// Shared production API transport. No DOM, storage or framework dependency.
// Go int64/uint64 values must be decoded before JSON.parse can round them.
export function parseJSON(text) {
  let index = 0;
  const whitespace = () => { while (/[\t\n\r ]/.test(text[index] || "x")) index++; };
  const fail = () => { throw new SyntaxError(`Invalid JSON at offset ${index}`); };
  function value(depth = 0) {
    if (depth > 100) throw new SyntaxError("JSON nesting limit exceeded");
    whitespace();
    const char = text[index];
    if (char === '"') {
      const start = index++;
      while (index < text.length) {
        if (text[index] === "\\") { index += 2; continue; }
        if (text[index++] === '"') return JSON.parse(text.slice(start, index));
      }
      return fail();
    }
    if (char === "{" || char === "[") {
      const object = char === "{";
      const result = object ? {} : [];
      const end = object ? "}" : "]";
      index++; whitespace();
      if (text[index] === end) { index++; return result; }
      while (index < text.length) {
        if (object) {
          whitespace();
          if (text[index] !== '"') return fail();
          const key = value(depth + 1);
          whitespace();
          if (text[index++] !== ":") return fail();
          const item = value(depth + 1);
          Object.defineProperty(result, key, {value: item, enumerable: true, configurable: true, writable: true});
        } else result.push(value(depth + 1));
        whitespace();
        if (text[index] === end) { index++; return result; }
        if (text[index++] !== ",") return fail();
      }
      return fail();
    }
    for (const [literal, result] of [["true", true], ["false", false], ["null", null]]) {
      if (text.startsWith(literal, index)) { index += literal.length; return result; }
    }
    const match = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/.exec(text.slice(index));
    if (!match) return fail();
    const token = match[0]; index += token.length;
    const number = Number(token);
    if (!/[.eE]/.test(token)) return Number.isSafeInteger(number) ? number : BigInt(token);
    if (!Number.isFinite(number) || (Number.isInteger(number) && !Number.isSafeInteger(number))) {
      throw new SyntaxError("Unsafe non-integer-encoded JSON number");
    }
    return number;
  }
  const result = value(); whitespace();
  if (index !== text.length) fail();
  return result;
}

export function stringifyJSON(value) {
  const active = new Set();
  function encode(item, arrayItem = false) {
    if (item === null) return "null";
    if (typeof item === "bigint") return item.toString();
    if (typeof item === "number" && (!Number.isFinite(item) || (Number.isInteger(item) && !Number.isSafeInteger(item)))) {
      throw new TypeError("Unsafe number: supply an exact BigInt instead");
    }
    if (typeof item !== "object") {
      const encoded = JSON.stringify(item);
      return encoded === undefined && arrayItem ? "null" : encoded;
    }
    if (active.has(item)) throw new TypeError("Circular JSON value");
    active.add(item);
    let result;
    if (Array.isArray(item)) result = `[${Array.from(item, entry => encode(entry, true)).join(",")}]`;
    else {
      if (Object.getPrototypeOf(item) !== Object.prototype && Object.getPrototypeOf(item) !== null) throw new TypeError("Expected a plain JSON object");
      result = `{${Object.keys(item).flatMap(key => {
        const encoded = encode(item[key]);
        return encoded === undefined ? [] : [`${JSON.stringify(key)}:${encoded}`];
      }).join(",")}}`;
    }
    active.delete(item);
    return result;
  }
  return encode(value);
}

export class APIError extends Error {
  constructor(message, details) { super(message); this.name = "APIError"; Object.assign(this, details); }
}

export function createAPI({fetch: fetcher = globalThis.fetch, origin = globalThis.location?.origin, now = Date.now, requireMutationCapabilities=false} = {}) {
  if (!origin) throw new TypeError("API origin is required");
  let token = "";
  let credentialGeneration=0;
  const capabilityListeners=new Set();
  const notifyCapabilities=value=>{for(const listener of capabilityListeners){try{listener(value);}catch{/* Observers must not change transport outcomes. */}}};
  let deadline = null, expiredCallback = null;
  return {
    requireMutationCapabilities,
    subscribeCapabilityReads(listener){capabilityListeners.add(listener);return()=>capabilityListeners.delete(listener);},
    setToken(value) { credentialGeneration++;token = String(value).trim(); deadline = null; expiredCallback = null; },
    clearToken() { credentialGeneration++;token = ""; deadline = null; expiredCallback = null; },
    expireToken() { credentialGeneration++;token = ""; deadline = 0; expiredCallback = null; },
    setTokenDeadline(value, callback) { deadline = value; expiredCallback = callback; },
    async request(path, {method = "GET", body, headers: inputHeaders, signal, timeout = 10000} = {}) {
      if (deadline !== null && now() >= deadline) {
        credentialGeneration++;
        token = "";
        const notify = expiredCallback; expiredCallback = null;
        notify?.();
        throw new APIError("Verified credential expired before dispatch", {kind: "expired", code: "local_session_expired", status: 401, uncertain: false});
      }
      const url = new URL(path, origin);
      if (url.origin !== origin || !url.pathname.startsWith("/api/v1/") || url.username || url.password || url.hash) throw new TypeError("Only same-origin versioned API requests are allowed");
      method = method.toUpperCase();
      const requestGeneration=credentialGeneration;
      const capabilityRead=method==="GET"&&url.pathname==="/api/v1/console/capabilities"&&!url.search;
      const schemaRead=method==="GET"&&url.pathname==="/api/v1/console/queue-schema"&&!url.search;
      const write = method !== "GET" && method !== "HEAD";
      const headers = new Headers(inputHeaders);
      headers.set("Accept", "application/json");
      if (token) headers.set("Authorization", `Bearer ${token}`);
      const payload = body === undefined ? undefined : stringifyJSON(body);
      if (payload !== undefined) headers.set("Content-Type", "application/json");
      const controller = new AbortController();
      const abort = () => controller.abort(signal?.reason);
      if (signal?.aborted) abort();
      else signal?.addEventListener("abort", abort, {once: true});
      const timer = setTimeout(() => controller.abort(new DOMException("Request timed out", "TimeoutError")), timeout);
      let response;
      try {
        response = await fetcher(url.href, {method, headers, body: payload, signal: controller.signal, cache: "no-store", redirect: "error", credentials: "same-origin"});
        const raw = await response.text();
        const metadata = {status: response.status, headers: response.headers, raw, uncertain: write};
        let data;
        try { data = raw === "" && response.status === 204 ? null : parseJSON(raw); }
        catch { throw new APIError("API returned an invalid JSON response", {...metadata, kind: "invalid-response"}); }
        if (!response.ok) {
          throw new APIError(data?.error?.message || (typeof data?.error === "string" ? data.error : `HTTP ${response.status}`), {...metadata, kind: "http", body: data, code: data?.error?.code});
        }
        if(capabilityRead&&requestGeneration===credentialGeneration)notifyCapabilities({body:data,revision:response.headers.get("ETag")});
        if(schemaRead&&requestGeneration===credentialGeneration)notifyCapabilities({schema:{body:data,revision:response.headers.get("ETag")}});
        return {body: data, status: response.status, headers: response.headers};
      } catch (error) {
        if((capabilityRead||schemaRead)&&requestGeneration===credentialGeneration)notifyCapabilities(null);
        if (error instanceof APIError) throw error;
        throw new APIError(controller.signal.aborted ? "API request canceled or timed out" : "API connection failed", {kind: controller.signal.aborted ? "aborted" : "network", status: response?.status, uncertain: write});
      } finally {
        clearTimeout(timer);
        signal?.removeEventListener("abort", abort);
      }
    },
    async download(path,{signal,timeout=10000,maxBytes=8<<20}={}) {
      if(!Number.isSafeInteger(maxBytes)||maxBytes<1)throw new TypeError("Invalid download limit");
      if(deadline!==null&&now()>=deadline){credentialGeneration++;token="";const notify=expiredCallback;expiredCallback=null;notify?.();throw new APIError("Verified credential expired before dispatch",{kind:"expired",code:"local_session_expired",status:401,uncertain:false});}
      const url=new URL(path,origin);if(url.origin!==origin||!url.pathname.startsWith("/api/v1/")||url.search||url.username||url.password||url.hash)throw new TypeError("Only same-origin versioned API downloads without query are allowed");
      const headers=new Headers({Accept:"application/zip"});if(token)headers.set("Authorization",`Bearer ${token}`);
      const controller=new AbortController(),abort=()=>controller.abort(signal?.reason);if(signal?.aborted)abort();else signal?.addEventListener("abort",abort,{once:true});const timer=setTimeout(()=>controller.abort(new DOMException("Request timed out","TimeoutError")),timeout);
      let response;
      try{
        response=await fetcher(url.href,{method:"GET",headers,signal:controller.signal,cache:"no-store",redirect:"error",credentials:"same-origin"});
        if(!response.ok){const raw=await response.text();let body;try{body=parseJSON(raw);}catch{body=null;}throw new APIError(body?.error?.message||`HTTP ${response.status}`,{kind:"http",status:response.status,body,code:body?.error?.code,uncertain:false});}
        const type=response.headers.get("Content-Type")?.split(";",1)[0].trim().toLowerCase(),declared=response.headers.get("Content-Length");
        if(type!=="application/zip"||declared!==null&&(!/^\d+$/.test(declared)||Number(declared)>maxBytes))throw new APIError("API returned an invalid diagnostic download",{kind:"invalid-response",status:response.status,uncertain:false});
        const bytes=new Uint8Array(await response.arrayBuffer());if(bytes.byteLength<1||bytes.byteLength>maxBytes||declared!==null&&Number(declared)!==bytes.byteLength)throw new APIError("API returned an invalid diagnostic download",{kind:"invalid-response",status:response.status,uncertain:false});
        return {blob:new Blob([bytes],{type:"application/zip"}),size:bytes.byteLength,headers:response.headers};
      }catch(error){if(error instanceof APIError)throw error;throw new APIError(controller.signal.aborted?"API download canceled or timed out":"API connection failed",{kind:controller.signal.aborted?"aborted":"network",status:response?.status,uncertain:false});}
      finally{clearTimeout(timer);signal?.removeEventListener("abort",abort);}
    },
  };
}

// A late response must never replace a newer resource/route, even if the
// transport ignores cancellation. Callers must ignore results marked stale.
export function latestRead(api) {
  let sequence = 0, controller;
  return {
    cancel() { sequence++; controller?.abort(); },
    async run(path, options={}) {
      const current = ++sequence;
      controller?.abort(); controller = new AbortController();
      try {
        const result = await api.request(path, {...options,signal: controller.signal});
        return current === sequence ? {stale: false, result} : {stale: true};
      } catch (error) {
        if (current !== sequence) return {stale: true};
        throw error;
      }
    },
  };
}
