import test from "node:test";
import assert from "node:assert/strict";
import {webcrypto} from "node:crypto";
import {createBrowserOIDC,decodeBrowserOIDC} from "../src/browser-oidc.mjs";

const value={schema:"rjs.browser-oidc.v1",authorizationEndpoint:"https://id.example/authorize",clientId:"management",callbackUrl:"https://console.example/admin/oidc/callback",scopes:["openid","profile"]};

test("browser OIDC config accepts only same-origin callback and fixed scopes",()=>{
  assert.equal(decodeBrowserOIDC(value,"https://console.example").clientId,"management");
  assert.throws(()=>decodeBrowserOIDC({...value,callbackUrl:"https://evil.example/admin/oidc/callback"},"https://console.example"));
  assert.throws(()=>decodeBrowserOIDC({...value,scopes:["openid","offline_access"]},"https://console.example"));
  assert.throws(()=>decodeBrowserOIDC({...value,authorizationEndpoint:"https://user:secret@id.example/authorize"},"https://console.example"));
});

function fixture(){
  const entries=new Map(),calls=[],signed=[];
  const location={origin:"https://console.example",pathname:"/admin/queues",search:"",hash:"",assigned:null,assign(url){this.assigned=url;}};
  const browser={location,sessionStorage:{getItem:key=>entries.get(key)??null,setItem:(key,item)=>entries.set(key,item),removeItem:key=>entries.delete(key)},history:{replaceState(_state,_title,path){location.pathname=path;location.search="";}},Event:class{constructor(type){this.type=type;}},dispatchEvent(event){calls.push({event:event.type});},crypto:webcrypto};
  const api={async request(path,options){calls.push({path,options});if(path==="/api/v1/oidc/config")return{body:value};if(path==="/api/v1/oidc/token")return{body:{schema:"rjs.browser-oidc-token.v1",token:"signed.id.token"}};throw Error("unexpected request");}};
  const session={async signIn(token){signed.push(token);},clear(){calls.push({clear:true});}};
  return{browser,entries,calls,signed,model:createBrowserOIDC(api,session,{browser,storage:()=>browser.sessionStorage,crypto:webcrypto,now:()=>1000})};
}

test("PKCE lifecycle keeps only one transient verifier, removes callback code, and stores no token",async()=>{
  const f=fixture();
  await f.model.start();
  const authorization=new URL(f.browser.location.assigned);
  assert.equal(authorization.origin,"https://id.example");
  assert.equal(authorization.searchParams.get("response_type"),"code");
  assert.equal(authorization.searchParams.get("code_challenge_method"),"S256");
  assert.equal(authorization.searchParams.get("redirect_uri"),value.callbackUrl);
  const pending=JSON.parse(f.entries.get("rjs.oidc.pkce.v1"));
  assert.equal(pending.returnPath,"/admin/queues");
  assert.ok(pending.verifier.length>=43);
  f.browser.location.pathname="/admin/oidc/callback";
  f.browser.location.search=`?code=one-time-code&state=${encodeURIComponent(pending.state)}`;
  assert.equal(await f.model.complete(),true);
  assert.equal(f.browser.location.pathname,"/admin/queues");
  assert.equal(f.browser.location.search,"");
  assert.equal(f.entries.size,0);
  assert.deepEqual(f.signed,["signed.id.token"]);
  const exchange=f.calls.find(call=>call.path==="/api/v1/oidc/token");
  assert.equal(exchange.options.body.verifier,pending.verifier);
  assert.equal(exchange.options.body.code,"one-time-code");
});

test("invalid callback still clears transient state and the URL before failing",async()=>{
  const f=fixture();
  await f.model.start();
  f.browser.location.pathname="/admin/oidc/callback";
  f.browser.location.search="?code=secret&state=wrong&unexpected=1";
  await assert.rejects(()=>f.model.complete(),/invalid OIDC callback/);
  assert.equal(f.browser.location.pathname,"/admin/queues");
  assert.equal(f.browser.location.search,"");
  assert.equal(f.entries.size,0);
  assert.deepEqual(f.signed,[]);
  assert.equal(f.calls.some(call=>call.path==="/api/v1/oidc/token"),false);
});

test("an older backend without the OIDC bootstrap route is treated as SSO unavailable",async()=>{
  const browser={location:{origin:"https://console.example"},sessionStorage:{},crypto:webcrypto};
  const model=createBrowserOIDC({request:async()=>{throw Object.assign(Error("route missing"),{status:404,code:"not_found"});}},{},{browser,crypto:webcrypto});
  assert.equal(await model.load(),null);
  assert.equal(model.config,null);
});
