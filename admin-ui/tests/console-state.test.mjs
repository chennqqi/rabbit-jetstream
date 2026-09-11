import test from "node:test";
import assert from "node:assert/strict";
import {hasAuthenticatedConsole} from "../src/console-state.mjs";

test("console navigation aids require the authenticated content lifecycle",()=>{
  const retainedIdentity={actor:"operator"};
  assert.equal(hasAuthenticatedConsole("authenticated",retainedIdentity),true);
  assert.equal(hasAuthenticatedConsole("expired",retainedIdentity),false);
  assert.equal(hasAuthenticatedConsole("signed-out",retainedIdentity),false);
  assert.equal(hasAuthenticatedConsole("authenticated",null),false);
});
