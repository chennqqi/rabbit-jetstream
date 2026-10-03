import {createQueueList} from "./queue-list.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

// One lifetime per resource/query. Replacing the route must dispose the old
// instance; neither a clamped response nor a late read may change its query.
export function createListRefresh(api,resource,query,options={}){
  const requestQuery={...query},model=createQueueList(api,resource);
  const refresh=createRefreshLoop(async()=>{
    await model.load(requestQuery,{restore:true});
    return model.snapshot().phase==="ready";
  },options);
  return {model,refresh,clear(){refresh.stop();model.clear();}};
}
