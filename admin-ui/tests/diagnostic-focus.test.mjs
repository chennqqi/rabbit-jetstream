import test from "node:test";
import assert from "node:assert/strict";
import {diagnosticControlID,focusDiagnostic} from "../src/diagnostic-focus.mjs";

test("diagnostic pointers map only to exact known controls, not guessed fields",()=>{
  for(const [path,id] of [
    ["/spec/replicas","queue-field-replicas"],["/spec/retention/maxMessages","queue-field-maxMessages"],
    ["/spec/delivery/ackWait","queue-field-ackWait"],["/spec/deadLetter/queue","queue-field-deadLetter"],
    ["/spec/subjects/2","queue-subject-2"],["/spec/bindings/1/exchange","binding-exchange-1"],
    ["/spec/bindings/1/type","binding-type-1"],["/spec/bindings/1/keys/2","binding-key-1-2"]
  ])assert.equal(diagnosticControlID(path),id);
  for(const path of ["/spec","/spec/retention","/spec/bindings/1/keys","/spec/subjects/01","/spec/subjects/-1",
    "/spec/subjects/999999999999999999999","/metadata/labels/a~1b",'#queue-draft, input',"/spec/unknown"])
    assert.equal(diagnosticControlID(path),"queue-draft");
});
test("focus is scoped, opens containing details and falls back for absent or disabled controls",()=>{
  let focused="",queries=[];
  const root={querySelector(id){queries.push(id);return controls[id]??null;}};
  const details={tagName:"DETAILS",open:false,parentElement:root};
  const control=(name,parent=details,disabled=false)=>({parentElement:parent,matches:()=>disabled,focus(){focused=name;}});
  const controls={"#queue-field-replicas":control("replicas"),"#queue-draft":control("json",root)};
  assert.equal(focusDiagnostic(root,"/spec/replicas"),true);assert.equal(focused,"replicas");assert.equal(details.open,true);
  assert.equal(focusDiagnostic(root,"/spec/subjects/9"),true);assert.equal(focused,"json");
  controls["#binding-key-1-2"]=control("disabled",details,true);
  focusDiagnostic(root,"/spec/bindings/1/keys/2");assert.equal(focused,"json");
  controls["#queue-draft"]=control("json",root,true);assert.equal(focusDiagnostic(root,"/spec"),false);
  assert.equal(focusDiagnostic(null,"/spec"),false);
  assert.ok(queries.every(query=>/^#[a-z0-9-]+$/.test(query)));
});
