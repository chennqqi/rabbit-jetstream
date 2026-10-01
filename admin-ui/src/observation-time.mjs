import {auditTimeNanos} from "./audit-time.mjs";

// Preserve the source's RFC3339 representation and precision. Date.parse alone
// normalizes impossible dates and accepts values outside the wire contract.
export function validObservationTime(value){
  if(typeof value!=="string"||value.startsWith("0001-"))return false;
  try{auditTimeNanos(value);return true;}catch{return false;}
}
