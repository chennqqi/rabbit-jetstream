import test from "node:test";
import assert from "node:assert/strict";
import {queueListLabels} from "../src/queue-list-labels.mjs";

test("Queue and Stream list catalogs preserve distinct evidence wording",()=>{
  for(const language of ["en","zh"]){const queues=queueListLabels(language,"queues"),streams=queueListLabels(language,"streams");assert.notEqual(queues.note,streams.note);assert.notEqual(queues.caption,streams.caption);assert.equal(Object.keys(queues.observations).length,4);assert.equal(Object.keys(queues.errors).length,6);}
  assert.equal(queueListLabels("other").title,"Queues");
});
