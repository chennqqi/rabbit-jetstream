import {createAuditList} from "./audit-list.mjs";
import {readAuditQuery,auditQueryParams} from "./audit-query.mjs";
import {stringifyJSON} from "./api.mjs";

// Controlled browser export. Never download a silently truncated traversal.
export function createAuditExport(api,{maxWindows=256,maxBytes=16*1024*1024}={}) {
  const reader=createAuditList(api),listeners=new Set();let generation=0;
  let state={phase:"idle",windows:0,events:0,content:null,failure:null};
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  const cancel=()=>{generation++;reader.clear();emit({phase:"idle",windows:0,events:0,content:null,failure:null});};
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},cancel,
    async start(query){
      cancel();const current=generation;
      const filter=readAuditQuery(auditQueryParams({...query,before:null}).toString());
      const windows=[];let before=null,bytes=0,events=0;
      const startedAt=new Date().toISOString();emit({...state,phase:"loading"});
      try {
        while(true){
          if(windows.length>=maxWindows)throw new Error("limit");
          await reader.load({...filter,before});if(current!==generation)return;
          const result=reader.snapshot();if(result.phase!=="ready")throw new Error("read");
          const window={before,readAt:result.readAt,page:result.page};
          bytes+=new TextEncoder().encode(stringifyJSON(window)).length;
          if(bytes>maxBytes)throw new Error("limit");
          windows.push(window);events+=result.page.items.length;
          emit({phase:"loading",windows:windows.length,events,content:null,failure:null});
          if(result.page.nextBefore===null)break;
          before=String(result.page.nextBefore);
        }
        const content=stringifyJSON({format:"rabbit-jetstream.audit-evidence.v1",filter,startedAt,finishedAt:new Date().toISOString(),
          coverage:"Observed retained lower boundary reached. Not an atomic snapshot, full history, or operation outcome attribution. New arrivals after the initial high-water mark are excluded; retention may remove records during traversal.",
          initialLastSequence:windows[0].page.lastSequence,windows});
        if(new TextEncoder().encode(content).length>maxBytes)throw new Error("limit");
        emit({phase:"ready",windows:windows.length,events,content,failure:null});
      }catch(error){if(current!==generation)return;emit({phase:"error",windows:windows.length,events,content:null,failure:error.message==="limit"?"limit":"read"});}
    },
  };
}
