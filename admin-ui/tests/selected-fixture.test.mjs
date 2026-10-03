import {test} from "node:test";
import assert from "node:assert/strict";
import {selectedFixture,selectedFixtureResponse as read} from "../../tests/admin-ui/selected-fixture.mjs";

test("Selected fixture matches the agreed counts and keeps KV revision distinct",()=>{
  const declaration=read("GET",`/api/v1/queues/${selectedFixture.queue}`);
  assert.equal(declaration.headers.etag,'"12"');assert.notEqual(declaration.body.revision,"12");
  const stream=read("GET",`/api/v1/streams/${selectedFixture.stream}`).body;
  assert.equal(stream.messages,12480);assert.equal(stream.replicas,3);assert.equal(stream.cluster.replicas[1].offline,true);assert.equal(stream.cluster.replicas[1].lag,undefined);
  const consumer=read("GET",`/api/v1/streams/${selectedFixture.stream}/consumers/${selectedFixture.consumer}`).body;
  assert.equal(consumer.pending,8420);assert.equal(consumer.ack_pending,240);
  declaration.body.plan.stream.replicas=1;
  assert.equal(read("GET",`/api/v1/queues/${selectedFixture.queue}`).body.plan.stream.replicas,3);
});
test("Selected fixture rejects writes and unspecified routes rather than proxying",()=>{
  for(const method of ["POST","PUT","DELETE","PATCH"])assert.equal(read(method,"/api/v1/queues/orders_events").status,405);
  for(const path of ["/api/v1/info","/api/v1/nodes","/api/v1/queues/orders_events?unknown=true"])assert.equal(read("GET",path).status,404);
});
