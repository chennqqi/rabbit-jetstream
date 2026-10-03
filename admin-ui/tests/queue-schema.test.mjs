import test from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {createHash} from "node:crypto";
import {parseJSON,stringifyJSON,createAPI} from "../src/api.mjs";
import {compatibleQueueSchema,readQueueSchema,queueSchemaPath} from "../src/queue-schema.mjs";
import {createCapabilities,validCapabilities} from "../src/capabilities.mjs";
import {readMutationCapabilities,observedCapabilityChange} from "../src/mutation-capabilities.mjs";
import {compileQueueForm} from "../src/queue-form-schema.mjs";
import {createQueueDraft} from "../src/queue-draft.mjs";
import {editQueueField} from "../src/queue-fields.mjs";
import {createCreationContract} from "../src/creation-contract.mjs";
import {compileQueueCollections} from "../src/queue-collection-schema.mjs";

const raw=readFileSync(new URL("../../internal/topology/queue-schema.json",import.meta.url));
const schema=()=>parseJSON(raw.toString());
const schemaTag=`"rjs-queue-schema-v1:${createHash("sha256").update(raw).digest("hex")}"`;
const capTag=`"rjs-capabilities-v1:${"a".repeat(64)}"`;
function capabilities(){
  return {schemaVersion:"rjs.console-capabilities.v1",deployment:{profile:"unknown",source:"unspecified"},
    queue:{apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",supportedReplicas:[1,3,5],supportedStorage:["file","memory"],
      minimumPriority:0,maximumPriority:255,requiresExplicitReplicas:true,defaults:{storage:"file",delivery:{ackWait:"30s",maxDeliver:5}},
      schema:{id:schema().$id,version:"rjs.queue-schema.v1",url:queueSchemaPath,etag:schemaTag}},
    features:["queue-schema","queue-preview","capability-preconditions","conditional-queue-writes"],qualification:{status:"unreported"}};
}
function fixture(){
  let document=schema(),etag=schemaTag,fail=false;const calls=[];
  const api={request:async(path,options)=>{
    calls.push({path,options});
    if(path.endsWith("/capabilities"))return {body:capabilities(),headers:new Headers({ETag:capTag})};
    if(fail)throw {status:503};
    return {body:document,headers:new Headers({ETag:etag})};
  }};
  return {api,calls,document,fail(){fail=true;},tag(value){etag=value;}};
}

test("collection schema derives label types and routing cardinality without inserting rows or defaults",()=>{
  const source=schema(),before=stringifyJSON(source),rules=compileQueueCollections(source);
  assert.equal(rules.labels.value.type,"string");assert.equal(rules.labels.value.minLength,undefined);
  assert.equal(rules.routing.subjects.minimum,1);assert.equal(rules.routing.bindings.minimum,1);
  assert.deepEqual(rules.routing.bindings.keys,{direct:{minimum:1},topic:{minimum:1},fanout:{maximum:0}});
  assert.equal(rules.routing.bindings.key.format,"rjs-routing-key");assert.equal(stringifyJSON(source),before);
  source.properties.spec.oneOf[0].properties.subjects.minItems=2;
  source.properties.spec.properties.bindings.items.oneOf[1].properties.keys.minItems=3;
  source.properties.metadata.properties.labels.additionalProperties.maxLength=20;
  const changed=compileQueueCollections(source);
  assert.equal(changed.routing.subjects.minimum,2);assert.equal(changed.routing.bindings.keys.topic.minimum,3);
  assert.equal(changed.labels.value.maxLength,20);
});

test("incompatible collection structures cannot authorize the v1 form adapter",()=>{
  for(const mutate of [
    s=>s.properties.metadata.properties.labels.additionalProperties.type="object",
    s=>s.properties.metadata.properties.labels.additionalProperties.enum=["restricted"],
    s=>s.properties.metadata.properties.labels.maxProperties=-1,
    s=>s.properties.metadata.properties.labels.patternProperties={".*":{type:"integer"}},
    s=>s.properties.spec.properties.subjects.minItems=2,
    s=>s.properties.spec.properties.subjects.items.type="integer",
    s=>s.properties.spec.properties.subjects.uniqueItems=false,
    s=>s.properties.spec.oneOf.pop(),
    s=>s.properties.spec.oneOf[0].properties.bindings.maxItems=1,
    s=>s.properties.spec.properties.bindings.items.oneOf[2].properties.keys.maxItems=1,
    s=>s.properties.spec.properties.bindings.items.oneOf[0].properties.keys.minItems=Number.MAX_SAFE_INTEGER+1,
    s=>s.properties.spec.properties.bindings.items.properties.exchange.type="object",
    s=>s.properties.spec.properties.bindings.items.properties.keys.items.type="array"
  ]){
    const document=schema();mutate(document);
    assert.throws(()=>compileQueueForm({schema:document,body:capabilities()}));
  }
});

test("initial creation requires ready schema, honors current bounds and preserves input through invalidation",async()=>{
  const f=fixture();let notify;
  f.api.subscribeCapabilityReads=listener=>{notify=listener;return()=>{};};
  f.document.properties.spec.properties.retention.properties.maxMessages.maximum=100;
  const contract=createCreationContract(f.api);
  const input={name:"created",subjects:"orders.events",replicas:"1",storage:"file",maxMessages:"99"};
  const original=JSON.stringify(input);
  assert.throws(()=>contract.prepare(input),{code:"schema-unavailable"});
  await contract.load();assert.equal(contract.snapshot().phase,"ready");
  assert.equal(contract.prepare(input).spec.retention.maxMessages,99n);
  assert.throws(()=>contract.prepare({...input,maxMessages:"101"}));
  assert.throws(()=>contract.prepare({...input,maxMessages:"0"}));
  assert.throws(()=>contract.prepare({...input,storage:""}));
  notify(null);assert.equal(contract.snapshot().phase,"error");assert.throws(()=>contract.prepare(input),{code:"schema-unavailable"});
  await contract.load();assert.equal(contract.snapshot().phase,"ready");
  assert.equal(JSON.stringify(input),original);assert.ok(f.calls.every(call=>!call.options?.method));
  contract.dispose();assert.throws(()=>contract.prepare(input),{code:"schema-unavailable"});
});

test("template preparation uses the creation contract and cannot bypass schema readiness",async()=>{
  const f=fixture(),contract=createCreationContract(f.api);
  const form={name:"templated",subjects:"template.events",replicas:"1",storage:"file",maxMessages:"9007199254740993",templateId:"priority",templatePriority:"2"};
  assert.throws(()=>contract.prepare(form),{code:"schema-unavailable"});
  await contract.load();const calls=f.calls.length;
  const draft=contract.prepare(form);
  assert.equal(draft.spec.maxPriority,2);assert.equal(draft.spec.retention.maxMessages,9007199254740993n);
  assert.equal(f.calls.length,calls,"template preparation performs no IO");
  contract.dispose();assert.throws(()=>contract.prepare(form),{code:"schema-unavailable"});
});

test("disposed creation contract cannot regain authority from a late schema response",async()=>{
  let resolve;
  const api={request:async path=>path.endsWith("/capabilities")?{body:capabilities(),headers:new Headers({ETag:capTag})}:new Promise(done=>resolve=done)};
  const contract=createCreationContract(api),pending=contract.load();
  await new Promise(done=>setTimeout(done,0));assert.ok(resolve);
  contract.dispose();const before=contract.snapshot();
  resolve({body:schema(),headers:new Headers({ETag:schemaTag})});await pending;
  assert.equal(contract.snapshot(),before);assert.equal(contract.snapshot().phase,"idle");
});

test("import draft requires current creation schema and preserves supported exact settings",async()=>{
  const f=fixture(),contract=createCreationContract(f.api);
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"imported"},spec:{replicas:1,storage:"file",subjects:["orders.created"],maxPriority:0,retention:{maxMessages:9223372036854775807n}}};
  assert.throws(()=>contract.prepareImport(document),{code:"schema-unavailable"});await contract.load();
  const draft=contract.prepareImport(document);assert.deepEqual(draft,document);assert.notEqual(draft,document);
  for(const spec of [{...document.spec,replicas:2},{...document.spec,storage:"unknown"}])assert.throws(()=>contract.prepareImport({...document,spec}),{code:"import-settings"});
  assert.ok(f.calls.every(call=>!call.options?.method));contract.dispose();assert.throws(()=>contract.prepareImport(document),{code:"schema-unavailable"});
});

test("form descriptors derive choices, requiredness and exact bounds without changing omission semantics",()=>{
  const binding={schema:schema(),body:capabilities()},before=stringifyJSON(binding);
  const form=compileQueueForm(binding),field=key=>form.fields.find(field=>field.key===key);
  assert.equal(form.fields.length,9);assert.deepEqual(field("replicas").options,["1","3","5"]);
  assert.equal(field("replicas").required,true);assert.equal(field("deadLetter").required,false);
  assert.equal(field("maxMessages").maximum,9223372036854775807n);
  assert.equal(field("ackWait").defaultValue,"30s");assert.equal(stringifyJSON(binding),before);
  const changed={schema:schema(),body:capabilities()};
  changed.schema.properties.spec.properties.replicas.enum=[5,3,1];
  changed.schema.properties.spec.properties.retention.properties.maxMessages.minimum=7;
  const changedForm=compileQueueForm(changed);
  assert.deepEqual(changedForm.fields.find(field=>field.key==="replicas").options,["5","3","1"]);
  assert.equal(changedForm.fields.find(field=>field.key==="maxMessages").minimum,7);
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{replicas:1,subjects:["q"]}};
  const edited=parseJSON(editQueueField(document,"maxMessages","9007199254740993",form.fields));
  assert.equal(edited.spec.retention.maxMessages,9007199254740993n);assert.equal(edited.spec.delivery,undefined);
  assert.equal(parseJSON(editQueueField(edited,"maxMessages","",form.fields)).spec.retention.maxMessages,undefined);
  for(const mutate of [
    b=>b.schema.properties.spec.properties.retention.properties.maxAge.type="integer",
    b=>b.schema.properties.spec.properties.retention.properties.maxMessages.maximum=Number.MAX_SAFE_INTEGER+1,
    b=>b.schema.properties.spec.properties.delivery.properties.maxDeliver.minimum=100,
    b=>b.schema.properties.spec.properties.bindings.items.properties.type.enum=["future"]
  ]){
    const bad={schema:schema(),body:capabilities()};mutate(bad);
    if(bad.schema.properties.spec.properties.delivery.properties.maxDeliver.minimum===100)
      bad.schema.properties.spec.properties.delivery.properties.maxDeliver.maximum=10;
    assert.throws(()=>compileQueueForm(bad));
  }
});

test("form reads preserve create draft, fail closed, retry explicitly and ignore late reads after discard",async()=>{
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{replicas:1,subjects:["q"]}};
  const f=fixture(),model=createQueueDraft(f.api,"q",{document,requireCapabilities:true});
  const original=model.snapshot().raw;
  await model.loadForm();assert.equal(model.snapshot().form.phase,"ready");assert.equal(model.snapshot().raw,original);
  f.fail();await model.loadForm({refresh:true});assert.equal(model.snapshot().form.phase,"error");assert.equal(model.snapshot().raw,original);
  const count=f.calls.length;await model.loadForm();assert.equal(f.calls.length,count);
  let resolve;
  const api={clearToken(){},request:async path=>path.endsWith("/capabilities")?{body:capabilities(),headers:new Headers({ETag:capTag})}:new Promise(done=>resolve=done)};
  const retained=createQueueDraft(api,"q",{document,requireCapabilities:true}),pending=retained.loadForm();
  await new Promise(done=>setTimeout(done,0));assert.ok(resolve);
  retained.discard();const cleared=retained.snapshot();
  resolve({body:schema(),headers:new Headers({ETag:schemaTag})});await pending;
  assert.equal(retained.snapshot(),cleared);assert.equal(retained.snapshot().raw,"");
});

test("observed contract changes retire form rules without changing draft or sending a read",async()=>{
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{replicas:1,subjects:["q"]}};
  const f=fixture();let notify;
  f.api.subscribeCapabilityReads=listener=>{notify=listener;return()=>{};};
  const model=createQueueDraft(f.api,"q",{document,requireCapabilities:true});
  await model.loadForm();const before=model.snapshot().raw,count=f.calls.length;
  notify({body:capabilities(),revision:capTag});assert.equal(model.snapshot().form.phase,"ready");
  notify(null);assert.equal(model.snapshot().form.phase,"error");
  assert.equal(model.snapshot().raw,before);assert.equal(f.calls.length,count);
  await model.loadForm({refresh:true});assert.equal(model.snapshot().form.phase,"ready");assert.equal(model.snapshot().raw,before);
});

test("late form metadata never overwrites edited JSON or archived evidence",async()=>{
  for(const archive of [false,true]){
    const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{replicas:1,subjects:["q"]}};
    let resolve;const api={request:async path=>path.endsWith("/capabilities")?
      {body:capabilities(),headers:new Headers({ETag:capTag})}:new Promise(done=>resolve=done)};
    const model=createQueueDraft(api,"q",{document,requireCapabilities:true}),pending=model.loadForm();
    await new Promise(done=>setTimeout(done,0));assert.ok(resolve);
    model.edit("{invalid retained input");
    if(archive)model.archive();
    const before=model.snapshot();
    resolve({body:schema(),headers:new Headers({ETag:schemaTag})});await pending;
    assert.equal(model.snapshot().raw,before.raw);assert.equal(model.snapshot().validation,before.validation);
    if(archive)assert.equal(model.snapshot(),before);
    else assert.equal(model.snapshot().form.phase,"ready");
  }
});

test("next edit rereads form contract after acceptance; schema failure preserves fresh base and accepted receipt",async()=>{
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:"q"},spec:{replicas:1,subjects:["q"]}};
  let schemaReads=0,queueReads=0,writes=0,failSchema=false;
  const api={request:async(path,options)=>{
    if(path.endsWith("/capabilities")){
      const body=capabilities();
      return {body,headers:new Headers({ETag:capTag})};
    }
    if(path===queueSchemaPath){
      schemaReads++;if(failSchema)throw {status:503};
      return {body:schema(),headers:new Headers({ETag:schemaTag})};
    }
    if(options?.method==="PUT"){writes++;return {body:{queue:"q",status:"ready",blocked:false},headers:new Headers({ETag:'"8"'})};}
    if(path.endsWith("/preview"))return {body:{plan:{queue:"q"},result:{queue:"q",status:"ready",blocked:false,operations:[]},create_only:false,base_revision:'"7"'}};
    queueReads++;return {body:{queue:"q",document},headers:new Headers({ETag:queueReads===1?'"7"':'"8"'})};
  }};
  const model=createQueueDraft(api,"q",{requireCapabilities:true});
  await model.load();await model.loadForm();await model.preview();model.confirmApply(true);await model.apply();
  assert.equal(model.snapshot().phase,"accepted");
  const previousReads=schemaReads,requestId=model.snapshot().requestId;failSchema=true;
  await model.editNext();
  assert.equal(model.snapshot().phase,"editing");assert.equal(model.snapshot().form.phase,"error");
  assert.equal(model.snapshot().etag,'"8"');assert.equal(model.snapshot().raw,stringifyJSON(document));
  assert.equal(model.snapshot().acceptedOperations[0].requestId,requestId);
  assert.equal(schemaReads,previousReads+1);assert.equal(writes,1);
  failSchema=false;await model.loadForm({refresh:true});
  assert.equal(model.snapshot().form.phase,"ready");assert.equal(model.snapshot().modified,false);assert.equal(writes,1);
});
test("published schema metadata is recognized and exact int64 limits survive transport",async()=>{
  const f=fixture(),caps=capabilities();
  assert.equal(validCapabilities(caps),true);assert.equal(compatibleQueueSchema(schema(),caps),true);
  const result=await readQueueSchema(f.api,caps,capTag);
  assert.equal(result.properties.spec.properties.retention.properties.maxMessages.maximum,9223372036854775807n);
  assert.equal(f.calls[0].path,queueSchemaPath);
  assert.equal(f.calls[0].options.headers["X-RJS-If-Capabilities-Match"],capTag);
  assert.equal(stringifyJSON(f.document),stringifyJSON(schema()));
});
test("schema descriptor never permits arbitrary URLs and incompatible data fails closed",async()=>{
  for(const patch of [{url:"https://other.test/schema"},{version:"future"},{etag:'W/"weak"'},{id:"other"}]){
    const caps=capabilities(),f=fixture();Object.assign(caps.queue.schema,patch);
    assert.equal(validCapabilities(caps),false);await assert.rejects(readQueueSchema(f.api,caps,capTag));assert.equal(f.calls.length,0);
  }
  for(const mutate of [f=>f.tag('"other"'),f=>delete f.document.properties.spec.properties.retention,
    f=>f.document.properties.spec.properties.replicas.enum=[2],f=>f.fail()]){
    const f=fixture();mutate(f);await assert.rejects(readQueueSchema(f.api,capabilities(),capTag));
  }
});
test("preview bindings include schema and reject changed bodies even with unchanged opaque revision",async()=>{
  const f=fixture(),binding=await readMutationCapabilities(f.api,["queue-preview"]);
  assert.equal(binding.schema.$id,schema().$id);
  assert.equal(observedCapabilityChange(binding,{schema:{body:schema(),revision:schemaTag}}),false);
  f.document.description="changed annotation";
  assert.equal(observedCapabilityChange(binding,{schema:{body:f.document,revision:schemaTag}}),true);
  await assert.rejects(readMutationCapabilities(f.api,["queue-preview"],binding),{code:"capabilities_changed"});
  f.fail();await assert.rejects(readMutationCapabilities(f.api,["queue-preview"],binding),{code:"capabilities_unavailable"});
  assert.ok(f.calls.every(call=>!call.options?.method));
});
test("Settings removes schema on failed refresh and cannot restore a cleared model from a late schema read",async()=>{
  const f=fixture(),model=createCapabilities(f.api);await model.load();assert.equal(model.snapshot().phase,"ready");
  f.fail();await model.load();assert.equal(model.snapshot().phase,"error");assert.equal(model.snapshot().schema,undefined);
  let resolve;
  const delayed=createCapabilities({request:async path=>path.endsWith("/capabilities")?
    {body:capabilities(),headers:new Headers({ETag:capTag})}:new Promise(done=>resolve=done)});
  const pending=delayed.load();await new Promise(done=>setTimeout(done,0));assert.ok(resolve);
  delayed.clear();resolve({body:schema(),headers:new Headers({ETag:schemaTag})});await pending;assert.equal(delayed.snapshot().phase,"idle");
});
test("shared schema observations notify reviews but old credentials cannot notify the new session",async()=>{
  let resolve;const observations=[];
  const api=createAPI({origin:"http://local.test",fetch:()=>new Promise(done=>resolve=done)});
  api.subscribeCapabilityReads(value=>observations.push(value));api.setToken("first");
  const pending=api.request(queueSchemaPath);api.setToken("second");
  resolve(new Response(raw,{headers:{ETag:schemaTag}}));await pending;assert.equal(observations.length,0);
  const failed=api.request(queueSchemaPath);resolve(new Response("{}",{status:503}));await assert.rejects(failed);
  assert.deepEqual(observations,[null]);
});
