const count=value=>Number.isSafeInteger(value)&&value>0?value:null;
const integer=(value,max=18446744073709551615n)=>typeof value==="bigint"&&value>=0n&&value<=max?value.toString():Number.isSafeInteger(value)&&value>=0?String(value):null;
export function replicaEvidence(stream,desired) {
  const configured=count(stream?.replicas),declared=count(desired);
  const result={configured,declared,configMismatch:configured!==null&&declared!==null&&configured!==declared,phase:"unreported",rows:[],coverage:null};
  const cluster=stream?.cluster;if(cluster===undefined||cluster===null)return result;
  if(typeof cluster!=="object"||Array.isArray(cluster)||!Array.isArray(cluster.replicas)||cluster.leader!==undefined&&typeof cluster.leader!=="string")return {...result,phase:"invalid"};
  const rows=[],names=new Set();
  if(cluster.leader){names.add(cluster.leader);rows.push({name:cluster.leader,role:"leader",current:null,offline:null,lag:null,active:null});}
  for(const peer of cluster.replicas){
    if(!peer||typeof peer.name!=="string"||!peer.name||names.has(peer.name))return {...result,phase:"invalid"};
    names.add(peer.name);rows.push({name:peer.name,role:"follower",current:typeof peer.current==="boolean"?peer.current:null,offline:typeof peer.offline==="boolean"?peer.offline:null,lag:integer(peer.lag),active:integer(peer.active_nanos,9223372036854775807n)});
  }
  return {...result,phase:"reported",rows,leaderReported:!!cluster.leader,coverage:configured===null?null:rows.length===configured};
}
