import {latestRead} from "./api.mjs";

const empty=()=>({phase:"idle",value:null,readAt:null,failure:null});
export function createOverview(api) {
  const readers={info:latestRead(api),queues:latestRead(api)}, listeners=new Set();
  let state={info:empty(),queues:empty()};
  const emit=(key,value)=>{state={...state,[key]:value};for(const fn of listeners)fn();};
  async function read(key) {
    const reader=readers[key];reader.cancel();const previous=state[key];emit(key,{...previous,phase:"loading"});
    try {
      const result=await reader.run(key==="info"?"/api/v1/info":"/api/v1/queues?offset=0&limit=1&sort=name&order=asc");
      if(result.stale)return;
      const value=result.result.body;
      if(key==="info") {
        if(!value || typeof value.name!=="string" || typeof value.version!=="string" || !value.jetstream || typeof value.jetstream!=="object" || Array.isArray(value.jetstream))throw new Error("invalid");
      } else if(!value || !Number.isSafeInteger(value.total)||value.total<0||value.offset!==0||value.limit!==1||!Array.isArray(value.items)||value.items.length!==Math.min(1,value.total))throw new Error("invalid");
      emit(key,{phase:"ready",value,readAt:new Date().toISOString(),failure:null});
    }catch(error){
      const denied=error.status===401||error.status===403,disabled=error.status===404&&error.code==="read_api_disabled";
      emit(key,{...(denied||disabled?empty():previous),phase:"error",failure:denied?"denied":disabled?"disabled":error.message==="invalid"?"invalid":"unavailable"});
    }
  }
  return {snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},
    load:()=>Promise.all([read("info"),read("queues")]),
    clear(){for(const key of Object.keys(readers)){readers[key].cancel();emit(key,empty());}},
  };
}
