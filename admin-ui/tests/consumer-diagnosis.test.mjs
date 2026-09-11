import test from "node:test";
import assert from "node:assert/strict";
import {consumerDiagnosis,consumerNodeEvidence} from "../src/consumer-diagnosis.mjs";

const readAt="2026-09-10T12:00:00.000Z",now=Date.parse(readAt);
const base={phase:"ready",readAt,failure:null,resource:{stream:"S",name:"C",mode:"pull",ack_policy:"explicit",pending:10,ack_pending:5,max_ack_pending:5,redelivered:1,waiting:0}};
const run=(resource={})=>consumerDiagnosis({...base,resource:{...base.resource,...resource}},{now});

test("backlog hints preserve exact evidence without mutation or a health verdict",()=>{
  const before=structuredClone(base),result=consumerDiagnosis(base,{now});
  assert.deepEqual(result.findings,["pending-observed","ack-limit-reached","no-waiting-pull","redelivery-observed"]);
  assert.deepEqual(result.facts,{pending:"10",ack_pending:"5",redelivered:"1",waiting:"0",max_ack_pending:"5"});
  assert.deepEqual(base,before);assert.equal(result.health,undefined);
  const exact=run({pending:18446744073709551615n,ack_pending:9007199254740993n,max_ack_pending:9007199254740993n});
  assert.equal(exact.facts.pending,"18446744073709551615");assert.ok(exact.findings.includes("ack-limit-reached"));
  assert.ok(!run({ack_pending:9007199254740992n,max_ack_pending:9007199254740993n}).findings.includes("ack-limit-reached"));
});

test("stale, future, failed and incomplete reads cannot produce current hints",()=>{
  for(const patch of [{phase:"loading"},{phase:"error",failure:"unavailable"},{failure:"denied"}]) {
    const result=consumerDiagnosis({...base,...patch},{now});assert.equal(result.status,"stale");assert.deepEqual(result.findings,[]);assert.deepEqual(result.facts,{});
  }
  for(const options of [{now:now+30001},{now:now-1},{now,paused:true}])assert.equal(consumerDiagnosis(base,options).status,"stale");
  assert.equal(consumerDiagnosis(base,{now:now+30000}).status,"current");
  for(const patch of [{resource:null},{readAt:null},{readAt:"2026-02-30T00:00:00Z"}])assert.equal(consumerDiagnosis({...base,...patch},{now}).status,"unavailable");
});

test("unsupported counters and policies do not fabricate missing values or trigger limit hints",()=>{
  for(const value of [null,undefined,"5",true,NaN,Infinity,1.5,Number.MAX_SAFE_INTEGER+1,-2,9223372036854775808n]) {
    const result=run({ack_pending:value,max_ack_pending:value});assert.ok(result.missing.includes("ack_pending"));assert.ok(result.missing.includes("max_ack_pending"));assert.ok(!result.findings.includes("ack-limit-reached"));
  }
  for(const limit of [-1,-1n,0]) {const result=run({max_ack_pending:limit});assert.equal(result.facts.max_ack_pending,String(limit));assert.ok(!result.findings.includes("ack-limit-reached"));}
  for(const policy of ["none","future",undefined])assert.ok(!run({ack_policy:policy}).findings.includes("ack-limit-reached"));
  assert.ok(run({ack_policy:"all"}).findings.includes("ack-limit-reached"));
});

test("pull hints do not classify disconnected clients and zero counters are not health",()=>{
  assert.ok(!run({mode:"push"}).findings.includes("no-waiting-pull"));
  assert.ok(!run({waiting:1}).findings.includes("no-waiting-pull"));
  const quiet=run({pending:0,ack_pending:0,waiting:0,redelivered:0});assert.deepEqual(quiet.findings,[]);assert.equal(quiet.status,"current");assert.equal(quiet.health,undefined);
  assert.ok(!run({pending:18446744073709551616n}).findings.includes("pending-observed"));
});

test("Consumer replica hints use exact reported peers, not fabricated membership or health",()=>{
  const cluster={leader:"leader",replicas:[{name:"follower",current:false,offline:true,lag:9007199254740993n,active_nanos:1000}]};
  const result=run({cluster});
  assert.deepEqual(result.replicaFindings,[{code:"follower-offline",peer:"follower"},{code:"follower-not-current",peer:"follower"},{code:"follower-lag",peer:"follower",lag:"9007199254740993"}]);
  assert.equal(result.replicas.configured,null);assert.equal(result.replicas.coverage,null);
  assert.equal(result.replicas.rows[0].lag,null);assert.equal(result.health,undefined);
  assert.equal(run({cluster:{replicas:[]}}).replicaFindings[0].code,"leader-unreported");
  for(const value of [undefined,null]){const absent=run({cluster:value});assert.equal(absent.replicas.phase,"unreported");assert.deepEqual(absent.replicaFindings,[]);}
});

test("invalid, duplicate and stale replica evidence cannot produce current replica hints",()=>{
  for(const cluster of [{leader:"same",replicas:[{name:"same",offline:true}]},{replicas:[{name:"x"},{name:"x",offline:true}]},{replicas:null},"bad"]){
    const result=run({cluster});assert.equal(result.replicas.phase,"invalid");assert.deepEqual(result.replicaFindings,[]);
  }
  const cluster={leader:"leader",replicas:[{name:"peer",offline:"true",current:"false",lag:18446744073709551616n}]};
  assert.deepEqual(run({cluster}).replicaFindings,[]);
  const stale=consumerDiagnosis({...base,resource:{...base.resource,cluster}},{now:now+30001});
  assert.equal(stale.replicas,undefined);assert.equal(stale.replicaFindings,undefined);
});

test("Consumer peers correlate only to fresh uniquely named configured nodes",()=>{
  const replicas=run({cluster:{leader:"n1",replicas:[{name:"n2",current:true,offline:false,lag:0}]}}).replicas;
  const nodeState={phase:"ready",failure:null,readAt,snapshot:{nodes:[
    {id:"ID1",name:"n1",status:"available",sources:{varz:{available:true}}},
    {id:"ID2",name:"n2",status:"degraded",sources:{varz:{available:true}}},
  ]}};
  assert.deepEqual(consumerNodeEvidence(replicas,nodeState,{now}),{status:"current",readAt,rows:[
    {peer:"n1",association:"matched",nodeID:"ID1",monitoringStatus:"available"},
    {peer:"n2",association:"matched",nodeID:"ID2",monitoringStatus:"degraded"},
  ]});
  const unresolved=consumerNodeEvidence(replicas,{...nodeState,snapshot:{nodes:[]}},{now});
  assert.deepEqual(unresolved.rows.map(row=>row.association),["unresolved","unresolved"]);
  const ambiguous=consumerNodeEvidence(replicas,{...nodeState,snapshot:{nodes:[nodeState.snapshot.nodes[0],{...nodeState.snapshot.nodes[0],id:"other"}]}},{now});
  assert.equal(ambiguous.rows[0].association,"ambiguous");assert.equal(ambiguous.rows[0].nodeID,undefined);
});

test("stale or unavailable node evidence never becomes missing-node evidence",()=>{
  const replicas=run({cluster:{leader:"n1",replicas:[]}}).replicas;
  for(const state of [null,{phase:"error",failure:"unavailable",readAt,snapshot:{nodes:[]}},{phase:"ready",failure:null,readAt:null,snapshot:{nodes:[]}}]){
    const result=consumerNodeEvidence(replicas,state,{now});assert.equal(result.status,state?.readAt?"stale":"unavailable");assert.deepEqual(result.rows,[]);
  }
  assert.equal(consumerNodeEvidence(replicas,{phase:"ready",failure:null,readAt,snapshot:{nodes:[]}},{now:now+30001}).status,"stale");
});
