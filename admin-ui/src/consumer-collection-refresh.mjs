import {createQueueConsumers} from "./queue-consumers.mjs";
import {createStreamRead} from "./stream-detail.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

export function createConsumerCollectionRefresh(api,kind,name,query,options={}){
  if(!["queue","stream"].includes(kind))throw new TypeError("Invalid Consumer collection scope");
  const requested={...query},model=kind==="queue"?createQueueConsumers(api,name,{retainOnRefresh:true}):createStreamRead(api,name,true,{retainOnRefresh:true});
  const refresh=createRefreshLoop(async()=>{
    await model.load(requested,{restore:true});return model.snapshot().phase==="ready";
  },options);
  return {model,refresh,clear(){refresh.stop();model.clear();}};
}
