import {createNodes} from "./nodes.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

export function createNodeRefresh(api,options={}){
  const model=createNodes(api,{retainOnRefresh:true});
  const refresh=createRefreshLoop(async()=>{
    await model.load();const state=model.snapshot();
    return state.phase==="ready"&&state.snapshot.nodes.every(node=>node.status==="available");
  },options);
  return {model,refresh,clear(){refresh.stop();model.clear();}};
}
