import test from "node:test";
import assert from "node:assert/strict";
import {createRequestID} from "../src/request-id.mjs";

test("request IDs use getRandomValues without requiring randomUUID",()=>{
  const provider={getRandomValues(bytes){bytes.forEach((_,index)=>bytes[index]=index);return bytes;}};
  assert.equal(createRequestID(provider),"000102030405060708090a0b0c0d0e0f");
  assert.throws(()=>createRequestID({randomUUID(){return "not-used";}}),/unavailable/);
});
