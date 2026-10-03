import {test} from "node:test";
import assert from "node:assert/strict";
import {languageKey,readLanguage,saveLanguage} from "../src/language.mjs";

test("only supported explicit preferences override browser language",()=>{
  for(const language of ["en","zh"])assert.equal(readLanguage(language==="en"?"zh-CN":"en-US",()=>({getItem:key=>{assert.equal(key,languageKey);return language;}})),language);
  for(const value of [null,"", "ZH","<script>",'{"token":"secret"}']){
    assert.equal(readLanguage("zh-CN",()=>({getItem:()=>value})),"zh");
    assert.equal(readLanguage("fr",()=>({getItem:()=>value})),"en");
  }
  assert.equal(readLanguage("ZH-Hans",()=>undefined),"zh");
  assert.equal(readLanguage(undefined,()=>undefined),"en");
});
test("language persistence writes one allowlisted value and survives unavailable storage",()=>{
  const writes=[];const storage=()=>({setItem:(...args)=>writes.push(args)});
  assert.equal(saveLanguage("zh",storage),true);assert.deepEqual(writes,[[languageKey,"zh"]]);
  for(const invalid of [null,undefined,"token",{},"en-US"])assert.equal(saveLanguage(invalid,storage),false);
  assert.equal(writes.length,1);
  for(const blocked of [()=>{throw new Error("denied");},()=>({getItem(){throw new Error("denied");},setItem(){throw new Error("quota");}}),()=>undefined]){
    assert.equal(readLanguage("zh",blocked),"zh");assert.equal(saveLanguage("en",blocked),false);
  }
});
