import {consumerDetailURL} from "./routes.mjs";
import {consumerCounter} from "./queue-consumers.mjs";

export function summaryConsumerScope(plan) {
  const stream=plan?.stream?.name,name=plan?.consumer?.name;
  if(typeof stream!=="string"||typeof name!=="string"||plan.consumer.stream!==stream)return null;
  try{return {stream,name,url:consumerDetailURL(stream,name)};}catch{return null;}
}

export function summaryConsumerCounts(scope,state,{allowHistorical=false}={}) {
  const retained=allowHistorical&&(state?.phase==="loading"||state?.phase==="error"&&state.failure==="unavailable");
  const resource=state?.phase==="ready"||retained?state.resource:null;
  if(!scope||!resource||resource.stream!==scope.stream||resource.name!==scope.name)return {pending:null,ackPending:null};
  return {pending:consumerCounter({observed:resource},"pending"),ackPending:consumerCounter({observed:resource},"ack_pending")};
}
