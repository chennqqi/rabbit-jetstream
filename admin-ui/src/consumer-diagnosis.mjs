import {validObservationTime} from "./observation-time.mjs";
import {replicaEvidence} from "./replicas.mjs";

function count(value,max=9223372036854775807n) {
  if(typeof value==="number"&&!Number.isSafeInteger(value))return null;
  if(typeof value!=="number"&&typeof value!=="bigint")return null;
  const number=BigInt(value);
  return number>=0n&&number<=max?number:null;
}

// Advisory comparisons of one exact read. No rates, root causes or health grade.
export function consumerDiagnosis(state,{now=Date.now(),paused=false}={}) {
  const resource=state.resource;
  const empty={status:"unavailable",facts:{},findings:[],missing:[]};
  if(!resource||!validObservationTime(state.readAt)||!Number.isFinite(now))return empty;
  const age=now-Date.parse(state.readAt);
  if(state.phase!=="ready"||state.failure||paused||age<0||age>30000)return {...empty,status:"stale"};
  const values={pending:count(resource.pending,18446744073709551615n),ack_pending:count(resource.ack_pending),redelivered:count(resource.redelivered),waiting:count(resource.waiting),max_ack_pending:resource.max_ack_pending===-1||resource.max_ack_pending===-1n?-1n:count(resource.max_ack_pending)};
  const facts={},missing=[],findings=[];
  for(const [key,value] of Object.entries(values)) {
    if(value===null)missing.push(key);else facts[key]=value.toString();
  }
  const {pending,ack_pending,redelivered,waiting,max_ack_pending}=values;
  if(pending!==null&&pending>0n)findings.push("pending-observed");
  if(["explicit","all"].includes(resource.ack_policy)&&ack_pending!==null&&max_ack_pending!==null&&max_ack_pending>0n&&ack_pending>=max_ack_pending)findings.push("ack-limit-reached");
  if(resource.mode==="pull"&&pending!==null&&pending>0n&&waiting===0n)findings.push("no-waiting-pull");
  if(redelivered!==null&&redelivered>0n)findings.push("redelivery-observed");
  const replicas=replicaEvidence({cluster:resource.cluster}),replicaFindings=[];
  if(replicas.phase==="reported") {
    if(!replicas.leaderReported)replicaFindings.push({code:"leader-unreported"});
    for(const row of replicas.rows) {
      if(row.role!=="follower")continue;
      if(row.offline===true)replicaFindings.push({code:"follower-offline",peer:row.name});
      if(row.current===false)replicaFindings.push({code:"follower-not-current",peer:row.name});
      const lag=row.lag===null?null:count(BigInt(row.lag),18446744073709551615n);
      if(lag!==null&&lag>0n)replicaFindings.push({code:"follower-lag",peer:row.name,lag:lag.toString()});
    }
  }
  return {status:"current",readAt:state.readAt,facts,findings,missing,replicas,replicaFindings};
}

// Correlate Consumer peer names only with independently observed configured
// monitoring endpoints. An unresolved name is never proof that a node is absent.
export function consumerNodeEvidence(replicas,nodeState,{now=Date.now(),paused=false}={}) {
  const empty={status:"unavailable",readAt:null,rows:[]};
  if(replicas?.phase!=="reported"||!nodeState?.snapshot||!validObservationTime(nodeState.readAt)||!Number.isFinite(now))return empty;
  const age=now-Date.parse(nodeState.readAt);
  if(nodeState.phase!=="ready"||nodeState.failure||paused||age<0||age>30000)return {...empty,status:"stale",readAt:nodeState.readAt};
  const nodes=nodeState.snapshot.nodes;
  if(!Array.isArray(nodes))return empty;
  const rows=replicas.rows.map(peer=>{
    const matches=nodes.filter(node=>node?.name===peer.name&&node?.sources?.varz?.available===true);
    if(matches.length!==1)return {peer:peer.name,association:matches.length>1?"ambiguous":"unresolved"};
    const node=matches[0];
    return {peer:peer.name,association:"matched",nodeID:typeof node.id==="string"&&node.id?node.id:null,monitoringStatus:["available","degraded","unavailable"].includes(node.status)?node.status:"unknown"};
  });
  return {status:"current",readAt:nodeState.readAt,rows};
}
