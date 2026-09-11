import {test} from "node:test";
import assert from "node:assert/strict";
import {replicaEvidence} from "../src/replicas.mjs";

test("Replica evidence preserves every reported member and never invents Leader metrics",()=>{
  const result=replicaEvidence({replicas:3,cluster:{leader:"n1",replicas:[{name:"n2",current:true,offline:false,lag:0,active_nanos:12},{name:"n3",current:false,offline:true,lag:9007199254740993n,active_nanos:1000000000n}]}},3);
  assert.equal(result.coverage,true);assert.equal(result.rows.length,3);
  assert.deepEqual(result.rows[0],{name:"n1",role:"leader",current:null,offline:null,lag:null,active:null});
  assert.equal(result.rows[1].lag,"0");assert.equal(result.rows[2].lag,"9007199254740993");assert.equal(result.rows[2].offline,true);
});
test("Absent topology is not an empty healthy group; missing members not synthesized",()=>{
  assert.equal(replicaEvidence({replicas:1},1).phase,"unreported");
  const missing=replicaEvidence({replicas:3,cluster:{replicas:[{name:"n2"}]}},5);
  assert.equal(missing.configMismatch,true);assert.equal(missing.leaderReported,false);assert.equal(missing.coverage,false);assert.equal(missing.rows.length,1);assert.equal(missing.rows[0].current,null);
});
test("Duplicate or invalid replica identities fail without partial rows",()=>{
  for(const cluster of [{leader:"n1",replicas:[{name:"n1"}]},{replicas:[{name:"n2"},{name:"n2"}]},{leader:123,replicas:[]},{replicas:null}]){
    const result=replicaEvidence({replicas:3,cluster});assert.equal(result.phase,"invalid");assert.deepEqual(result.rows,[]);
  }
});

test("replica counters respect uint64 lag and int64 activity wire limits",()=>{
  const row=(lag,active_nanos)=>replicaEvidence({cluster:{replicas:[{name:"peer",lag,active_nanos}]}}).rows[0];
  assert.equal(row(18446744073709551615n,9223372036854775807n).lag,"18446744073709551615");
  assert.equal(row(0,9223372036854775807n).active,"9223372036854775807");
  assert.equal(row(18446744073709551616n,9223372036854775808n).lag,null);
  assert.equal(row(18446744073709551616n,9223372036854775808n).active,null);
  for(const invalid of [-1,-1n,"1",Number.MAX_SAFE_INTEGER+1])assert.equal(row(invalid,invalid).lag,null);
});
