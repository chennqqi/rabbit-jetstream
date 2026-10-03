import {createStreamRead} from "./stream-detail.mjs";
import {createConsumerDetail} from "./consumer-detail.mjs";
import {summaryConsumerScope} from "./summary-consumer.mjs";
import {createRefreshLoop} from "./refresh-loop.mjs";

export function createSummaryRefresh(api,plan,options={}){
  const scope=summaryConsumerScope(plan),stream=createStreamRead(api,plan.stream.name,false,{retainOnRefresh:true});
  const empty=Object.freeze({phase:"idle",resource:null,failure:null,readAt:null});
  const consumer=scope?createConsumerDetail(api,scope.stream,scope.name,{retainOnRefresh:true}):{snapshot:()=>empty,subscribe:()=>()=>{},clear(){}};
  const refresh=createRefreshLoop(async(selection)=>{
    const reads=[];
    if(selection===undefined||selection==="stream")reads.push(stream.load());
    if(scope&&(selection===undefined||selection==="consumer"))reads.push(consumer.load());
    await Promise.all(reads);
    return stream.snapshot().phase==="ready"&&(!scope||consumer.snapshot().phase==="ready");
  },options);
  return {scope,stream,consumer,refresh,clear(){refresh.stop();stream.clear();consumer.clear();}};
}
