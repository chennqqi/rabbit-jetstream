import {latestRead} from "./api.mjs";
import {auditTimeNanos} from "./audit-time.mjs";
import {validateAuditWindow} from "./audit-window.mjs";
import {auditFields, auditQueryParams, readAuditQuery} from "./audit-query.mjs";

export function createAuditList(api) {
  const reader=latestRead(api),listeners=new Set();
  let state={phase:"idle",page:null,failure:null,readAt:null};
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){reader.cancel();emit({phase:"idle",page:null,failure:null,readAt:null});},
    async load(query){
      const params=auditQueryParams(query),normalized=readAuditQuery(params.toString());
      reader.cancel();emit({phase:"loading",page:null,failure:null,readAt:null});
      try {
        const response=await reader.run(`/api/v1/audit/windows?${params}`);if(response.stale)return;
        const page=validateAuditWindow(response.result.body,null,normalized.before);
        if(!page.filter || auditFields.some(key=>page.filter[key]!==normalized[key]))throw new Error("Invalid audit filter echo");
        const mapping={requestId:"requestId",resource:"resourceName",actor:"actor",phase:"phase",action:"action",outcome:"outcome"};
        if(page.items.some(event=>["time","resourceName","actor","phase","action","outcome"].some(key=>typeof event[key]!=="string")))throw new Error("Invalid audit event fields");
        if(page.items.some(event=>Object.keys(mapping).some(key=>normalized[key]&&event[mapping[key]]!==normalized[key])))throw new Error("Invalid audit filtered event");
        for(const event of page.items){
          if(normalized.from&&auditTimeNanos(event.time)<auditTimeNanos(normalized.from)||normalized.until&&auditTimeNanos(event.time)>=auditTimeNanos(normalized.until))throw new Error("Audit event outside requested time range");
        }
        emit({phase:"ready",page,failure:null,readAt:new Date().toISOString()});
      }catch(error){emit({phase:"error",page:null,readAt:null,failure:error.status===401||error.status===403?"denied":error.status===400?"query":error.status===503?"unavailable":"invalid-or-unavailable"});}
    },
  };
}
