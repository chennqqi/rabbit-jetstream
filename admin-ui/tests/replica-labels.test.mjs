import test from "node:test";
import assert from "node:assert/strict";
import {replicaLabels,replicaRole} from "../src/replica-labels.mjs";

test("replica accessible names and roles follow the visible language",()=>{
  assert.deepEqual(replicaLabels("en"),{evidence:"Replica evidence",observations:"Replica observations",leader:"Leader",follower:"Follower"});
  assert.deepEqual(replicaLabels("zh"),{evidence:"副本观测证据",observations:"副本观测",leader:"Leader（主节点）",follower:"Follower（副本节点）"});
  assert.equal(replicaRole("leader","zh"),"Leader（主节点）");
  assert.equal(replicaRole("follower","en"),"Follower");
  assert.equal(replicaRole("unknown","zh"),undefined);
  assert.equal(replicaLabels("unexpected"),replicaLabels("en"));
  assert.equal(Object.isFrozen(replicaLabels("zh")),true);
});
