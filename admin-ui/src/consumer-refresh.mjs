import {createConsumerDetail} from "./consumer-detail.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

export function createConsumerRefresh(api,stream,name,options={}){
  const model=createConsumerDetail(api,stream,name,{retainOnRefresh:true});
  const refresh=createRefreshLoop(async()=>{await model.load();return model.snapshot().phase==="ready";},options);
  return {model,refresh,clear(){refresh.stop();model.clear();}};
}
