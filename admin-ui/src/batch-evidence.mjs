import {stringifyJSON} from "./api.mjs";

export function batchEvidence(record){
  if(record?.schema!=="rjs.batch-execution-evidence.v1"||record.scope!=="archived-batch-not-write-authorization"||
    typeof record.archivedAt!=="string"||!Array.isArray(record.limitations)||!record.limitations.every(value=>typeof value==="string")||
    !record.plan||!Array.isArray(record.items))throw Error("Invalid batch evidence");
  return stringifyJSON(record)+"\n";
}
