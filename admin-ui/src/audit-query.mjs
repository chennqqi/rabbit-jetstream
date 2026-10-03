import {auditTimeNanos} from "./audit-time.mjs";
export const auditFields = ["requestId", "resource", "actor", "phase", "action", "outcome", "from", "until"];
export function readAuditQuery(search = "") {
  const params = new URLSearchParams(search), query = Object.fromEntries(auditFields.map(key => [key, ""]));
  query.before = null;
  for (const [key,value] of params) {
    if (params.getAll(key).length !== 1 || ![...auditFields,"before"].includes(key)) throw new TypeError("Invalid audit query");
    if (key === "before") {
      if (!/^\d+$/.test(value) || BigInt(value) > 18446744073709551615n) throw new TypeError("Invalid audit cursor");
      query.before = BigInt(value).toString();
    } else {
      if (new TextEncoder().encode(value).length > 256 || /[\u0000-\u001f\u007f-\u009f]/.test(value)) throw new TypeError("Invalid audit filter");
      query[key] = value;
    }
  }
  if (!["", "intent", "outcome"].includes(query.phase)) throw new TypeError("Invalid audit phase");
  const from=query.from?auditTimeNanos(query.from):null,until=query.until?auditTimeNanos(query.until):null;
  if(from!==null&&until!==null&&from>=until)throw new TypeError("Invalid audit time range");
  return query;
}
export function auditQueryParams(query) {
  const params = new URLSearchParams(Object.entries(query).filter(([,value]) => value !== null && value !== ""));
  readAuditQuery(params.toString());
  return params;
}
export function auditURL(query) {return `/admin/audit?${auditQueryParams(query)}`;}
