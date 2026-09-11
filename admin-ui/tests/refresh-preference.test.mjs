import test from "node:test";
import assert from "node:assert/strict";
import {readRefreshPreference,saveRefreshPreference,refreshPreferenceKey} from "../src/refresh-preference.mjs";
test("refresh preferences accept only exact allowlisted storage values",()=>{
  for(const value of [0,10,30,60])assert.equal(readRefreshPreference(()=>({getItem:()=>String(value)})),value);
  for(const value of [null,"", "010","1e1"," 10","-1","60000","{}"] )assert.equal(readRefreshPreference(()=>({getItem:()=>value})),10);
  assert.equal(readRefreshPreference(()=>{throw Error("denied");}),10);
});
test("refresh preference writes no credentials and reports storage failures",()=>{
  const writes=[];const storage=()=>({setItem:(...args)=>writes.push(args)});
  assert.equal(saveRefreshPreference(30,storage),true);assert.deepEqual(writes,[[refreshPreferenceKey,"30"]]);
  for(const value of ["10",null,undefined,NaN,1,{},true])assert.equal(saveRefreshPreference(value,storage),false);
  assert.equal(writes.length,1);assert.equal(saveRefreshPreference(0,()=>null),false);
  assert.equal(saveRefreshPreference(0,()=>{throw Error("denied");}),false);
});
