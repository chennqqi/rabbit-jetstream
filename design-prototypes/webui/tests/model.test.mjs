import test from "node:test";
import assert from "node:assert/strict";
import {
  initialConfig,
  validateConfig,
  configDiff,
  replicas,
  consumerFixture,
  queryConsumers,
  replicaState,
  parseRoute,
  routeHash,
} from "../src/model.js";
test("defaults and exact comparisons", () => {
  assert.deepEqual(validateConfig(initialConfig), {});
  assert.deepEqual(
    configDiff(initialConfig, { ...initialConfig, maxDeliver: "5" }),
    [],
  );
  assert.equal(
    configDiff(initialConfig, { ...initialConfig, ackWait: "45s" })[0].key,
    "ackWait",
  );
});
test("bounded duration and integer validation", () => {
  for (const value of [
    "",
    "-1s",
    "Infinityh",
    "24",
    "1e30h",
    "9007199254740992h",
  ])
    assert.ok(
      validateConfig({ ...initialConfig, retention: value }).retention,
      value,
    );
  for (const value of ["", "0", "-1", "1.5", "1e2", "9007199254740993"])
    assert.ok(
      validateConfig({ ...initialConfig, maxDeliver: value }).maxDeliver,
      value,
    );
  for (const value of ["1ms", "0.5s", "3m", "48h"])
    assert.deepEqual(validateConfig({ ...initialConfig, ackWait: value }), {});
});
test("semantic durations, zero age and exact nanoseconds", () => {
  assert.deepEqual(
    configDiff(initialConfig, {
      ...initialConfig,
      ackWait: "0.5m",
      retention: "1440m",
      maxDeliver: "005",
    }),
    [],
  );
  assert.deepEqual(
    validateConfig({ ...initialConfig, retention: "0s", maxDeliver: 1001 }),
    {},
  );
  assert.ok(validateConfig({ ...initialConfig, ackWait: "0s" }).ackWait);
  assert.ok(
    validateConfig({ ...initialConfig, ackWait: "0.0000000001s" }).ackWait,
  );
});
test("priority fixture includes primary priority zero in the same stream", () => {
  assert.equal(consumerFixture().length, 1);
  assert.equal(consumerFixture("priority0")[0].priority, 0);
  const rows = consumerFixture("priority7");
  assert.equal(rows.length, 8);
  assert.equal(new Set(rows.map((r) => r.stream)).size, 1);
  assert.equal(rows[7].name, "RJSQC_orders_events_P7");
  assert.equal(rows[7].subjects[0], "rjs.q.orders_events.p.7");
});
test("filter all records before pagination and clamp page", () => {
  const rows = consumerFixture("priority7");
  assert.equal(queryConsumers(rows, { page: 2 }).items.length, 3);
  const result = queryConsumers(rows, { q: "p.7", page: 2 });
  assert.equal(result.total, 1);
  assert.equal(result.page, 1);
  assert.equal(result.items[0].priority, 7);
  assert.equal(queryConsumers(rows, { mode: "push" }).total, 0);
  assert.equal(queryConsumers([], { page: 9 }).page, 1);
});
test("replica states remain independent", () => {
  assert.equal(replicaState({ offline: false, current: false }), "catching-up");
  assert.equal(replicaState({ offline: true, current: true }), "offline");
  assert.equal(replicaState({}), "unknown");
  assert.equal(replicaState({ offline: false, current: true }), "current");
});
test("safe route roundtrip retains list and selected record context", () => {
  const route = {
    screen: "consumers",
    q: "p.7 & ?",
    mode: "pull",
    page: 2,
    consumer: "RJSQC_orders_events_P7",
    fixture: "priority7",
  };
  assert.deepEqual(parseRoute(routeHash(route)), route);
  assert.equal(parseRoute("#queues").screen, "queues");
  assert.equal(parseRoute("#consumers?page=NaN").page, 1);
  assert.equal(parseRoute("#consumers?fixture=bad").fixture, "standard");
});
test("offline lag remains unknown", () => {
  assert.equal(replicas[2].lag, null);
  assert.equal(replicas[2].offline, true);
});
