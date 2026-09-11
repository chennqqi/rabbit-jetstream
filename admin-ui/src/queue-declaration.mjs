import {latestRead} from "./api.mjs";
import {queueDetailURL} from "./routes.mjs";

export function createQueueDeclaration(api,name) {
  queueDetailURL(name);
  const reader=latestRead(api),listeners=new Set();
  let state={phase:"idle"};
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  return {
    snapshot:()=>state,
    subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    clear(){reader.cancel();emit({phase:"idle"});},
    async load(){
      reader.cancel();emit({phase:"loading"});
      try {
        const result=await reader.run(`/api/v1/queues/${encodeURIComponent(name)}`);
        if(result.stale)return;
        const response=result.result,body=response.body;
        if(body?.queue!==name||typeof body.revision!=="string"||body.plan?.queue!==name||body.plan.revision!==body.revision||typeof body.plan.stream?.name!=="string")throw new Error("invalid-response");
        emit({phase:"ready",body,etag:response.headers.get("ETag"),readAt:new Date().toISOString()});
      }catch(error){emit({phase:"error",status:error.status,code:error.code});}
    },
  };
}
