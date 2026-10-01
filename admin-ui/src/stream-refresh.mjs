import {createStreamRead} from "./stream-detail.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

export function createStreamRefresh(api,name,options={}){
  const model=createStreamRead(api,name,false,{retainOnRefresh:true});
  const refresh=createRefreshLoop(async()=>{await model.load();return model.snapshot().phase==="ready";},options);
  return {model,refresh,clear(){refresh.stop();model.clear();}};
}
