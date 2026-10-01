import assert from "node:assert/strict";

const origin=process.env.RJS_TEST_ORIGIN??"http://127.0.0.1:18223";
const token=process.env.RJS_TEST_TOKEN??"s4-visual-qa-token";
const tenant=process.env.RJS_TEST_TENANT??"local";
const headers={Authorization:`Bearer ${token}`,"X-RJS-Tenant":tenant};
const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),15000);
try{
  const stream=await fetch(`${origin}/api/v1/events`,{headers:{...headers,Accept:"text/event-stream"},signal:controller.signal});
  assert.equal(stream.status,200);assert.match(stream.headers.get("content-type")??"",/^text\/event-stream/);assert.equal(stream.headers.get("x-accel-buffering"),"no");
  const name=`sse_live_${Date.now()}`,requestID=`sse-live-${Date.now()}`;
  const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name},spec:{subjects:[`${name}.events`],replicas:1,storage:"file",retention:{maxMessages:100}}};
  const mutation=await fetch(`${origin}/api/v1/queues/${name}`,{method:"PUT",headers:{...headers,"Content-Type":"application/json","If-None-Match":"*","X-Request-ID":requestID},body:JSON.stringify(document)});
  assert.equal(mutation.status,200,await mutation.text());
  const reader=stream.body.getReader(),decoder=new TextDecoder();let buffer="",event;
  while(!event){const {value,done}=await reader.read();assert.equal(done,false,"event stream closed before invalidation");buffer+=decoder.decode(value,{stream:true});let separator;while((separator=/\r?\n\r?\n/.exec(buffer))){const frame=buffer.slice(0,separator.index);buffer=buffer.slice(separator.index+separator[0].length);const lines=frame.split(/\r?\n/),type=lines.find(line=>line.startsWith("event: "))?.slice(7),id=lines.find(line=>line.startsWith("id: "))?.slice(4),data=lines.find(line=>line.startsWith("data: "))?.slice(6);if(type==="invalidate"&&id&&data)event={id,data:JSON.parse(data)};}}
  assert.match(event.id,/^[1-9]\d*$/);assert.deepEqual(event.data,{resource:"audit"});
  const audit=await fetch(`${origin}/api/v1/audit/windows?requestId=${encodeURIComponent(requestID)}`,{headers:{...headers,Accept:"application/json"}});assert.equal(audit.status,200);const evidence=await audit.json();assert.ok(evidence.items.some(item=>item.requestId===requestID));
  process.stdout.write(JSON.stringify({status:"passed",transport:"authenticated-fetch-sse",tenant,eventId:event.id,requestId:requestID})+"\n");
}finally{clearTimeout(timer);controller.abort();}
