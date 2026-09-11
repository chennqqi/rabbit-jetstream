// Isolated real-NATS/API qualification. Never connects to an existing service.
import assert from "node:assert/strict";
import {spawn} from "node:child_process";
import {createServer,createConnection} from "node:net";
import {once} from "node:events";
import {mkdtemp,mkdir,readFile,writeFile} from "node:fs/promises";
import {createHash,randomBytes} from "node:crypto";
import {cpus,totalmem,platform,release,arch} from "node:os";
import {performance} from "node:perf_hooks";
import path from "node:path";
import {fileURLToPath} from "node:url";
import {parseJSON} from "../../admin-ui/src/api.mjs";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"../.."),suffix=process.platform==="win32"?".exe":"";
const binaries=[`bin/nats-server-candidate${suffix}`,process.env.RJS_TEST_MANAGEMENT_BINARY||`bin/rjs-management-connections${suffix}`];
async function fingerprint(file){const bytes=await readFile(path.join(root,file));return {file,sha256:createHash("sha256").update(bytes).digest("hex")};}
const inputs=await Promise.all(binaries.map(fingerprint));
await mkdir(path.join(root,"artifacts"),{recursive:true});const evidence=await mkdtemp(path.join(root,"artifacts/connections-live-"));
const report={startedAt:new Date().toISOString(),inputs,checks:[],passed:false},children=[],sockets=[];
const token=randomBytes(24).toString("hex"),cleanEnv=Object.fromEntries(Object.entries(process.env).filter(([key])=>!/^(RJS_|OTEL_|NATS_)/i.test(key)));
const client=new AbortController();
async function port(){const server=createServer();server.listen(0,"127.0.0.1");await once(server,"listening");const value=server.address().port;await new Promise(resolve=>server.close(resolve));return value;}
function start(file,args,env={}){const child=spawn(path.join(root,file),args,{cwd:evidence,env:{...cleanEnv,...env},windowsHide:true,stdio:"ignore"});children.push(child);child.on("error",()=>{});return child;}
async function waitFor(read,condition){const until=Date.now()+15000;let last;while(Date.now()<until){if(children.some(child=>child.exitCode!==null||child.signalCode!==null))throw Error("Owned test service exited");try{last=await read();if(condition(last))return last;}catch{}await new Promise(resolve=>setTimeout(resolve,100));}throw Error(`Condition timed out (last status ${last?.status??"unavailable"})`);}
async function json(url,authenticated=true){const response=await fetch(url,{headers:authenticated?{Authorization:`Bearer ${token}`}:{},signal:AbortSignal.any([client.signal,AbortSignal.timeout(2000)])});const body=parseJSON(await response.text());return {status:response.status,body,cache:response.headers.get("cache-control")};}
async function postJSON(url,body,authenticated=true){const response=await fetch(url,{method:"POST",headers:{"Content-Type":"application/json",...(authenticated?{Authorization:`Bearer ${token}`}:{})},body:JSON.stringify(body),signal:AbortSignal.any([client.signal,AbortSignal.timeout(5000)])});const value=parseJSON(await response.text());return {status:response.status,body:value,cache:response.headers.get("cache-control")};}
async function connect(natsPort){
  const socket=createConnection({host:"127.0.0.1",port:natsPort});sockets.push(socket);socket.on("error",()=>{});
  return await new Promise((resolve,reject)=>{
    let pending="",id,ready=false;const pongWaiters=[];const timer=setTimeout(()=>{socket.destroy();reject(Error("Owned connection handshake timeout"));},3000);
    const ping=()=>new Promise((yes,no)=>{const timeout=setTimeout(()=>{const index=pongWaiters.findIndex(entry=>entry.yes===yes);if(index>=0)pongWaiters.splice(index,1);no(Error("Owned connection PING timeout"));},2000);pongWaiters.push({yes:()=>{clearTimeout(timeout);yes();}});socket.write("PING\r\n");});
    const fail=()=>{clearTimeout(timer);reject(Error("Owned connection closed before handshake"));};socket.once("error",fail);socket.once("close",fail);
    socket.on("data",chunk=>{pending+=chunk.toString();let end;while((end=pending.indexOf("\r\n"))>=0){const line=pending.slice(0,end);pending=pending.slice(end+2);
      if(line.startsWith("INFO ")){id=parseJSON(line.slice(5)).client_id;socket.write('CONNECT {"verbose":false,"name":"qualification-must-not-be-exposed"}\r\nPING\r\n');}
      else if(line==="PING")socket.write("PONG\r\n");
      else if(line==="PONG"){if(!ready){ready=true;clearTimeout(timer);socket.removeListener("error",fail);socket.removeListener("close",fail);resolve({socket,id,ping});}else pongWaiters.shift()?.yes();}
      else if(line.startsWith("-ERR")){clearTimeout(timer);reject(Error("Owned NATS handshake rejected"));}
    }});
  });
}
const percentile=(values,fraction)=>{const sorted=[...values].sort((a,b)=>a-b);return sorted[Math.min(sorted.length-1,Math.ceil(sorted.length*fraction)-1)];};
try{
  const ports=new Set();while(ports.size<3)ports.add(await port());const [natsPort,monitorPort,httpPort]=[...ports];
  const ownedName=`connections-${randomBytes(12).toString("hex")}`;
  start(binaries[0],["-js","-a","127.0.0.1","-p",String(natsPort),"-m",String(monitorPort),"-sd",path.join(evidence,"data"),"--name",ownedName]);
  await waitFor(()=>fetch(`http://127.0.0.1:${monitorPort}/healthz`,{signal:AbortSignal.timeout(1000)}),r=>r.ok);
  const identity=await json(`http://127.0.0.1:${monitorPort}/varz`,false);assert.equal(identity.body.server_name,ownedName);assert.ok(identity.body.server_id);
  start(binaries[1],[],{RJS_HTTP_ADDR:`127.0.0.1:${httpPort}`,RJS_DEPLOYMENT_PROFILE:"standalone",RJS_NATS_URL:`nats://127.0.0.1:${natsPort}`,RJS_NATS_MONITOR_URLS:`http://127.0.0.1:${monitorPort}`,RJS_ADMIN_TOKEN:token,RJS_AUDIT_TOKENS:token,RJS_CONTROLLER_ENABLED:"false",RJS_LOCAL_DEMO:"false",RJS_METADATA_REPLICAS:"1"});
  const base=`http://127.0.0.1:${httpPort}`;await waitFor(()=>fetch(`${base}/readyz`,{signal:AbortSignal.timeout(1000)}),r=>r.ok);
  const nodes=await json(`${base}/api/v1/nodes`);assert.equal(nodes.status,200);const node=nodes.body.nodes[0];assert.equal(node.id,identity.body.server_id);assert.equal(node.name,ownedName);
  const endpoint=`${base}/api/v1/nodes/${encodeURIComponent(node.id)}/connections`;
  assert.equal((await json(endpoint,false)).status,401);
  const baseline=await json(endpoint);assert.equal(baseline.status,200);const initialTotal=baseline.body.total;
  const connections=[];for(let i=0;i<201;i++)connections.push(await connect(natsPort));
  assert.ok(connections.every(v=>typeof v.id==="number"||typeof v.id==="bigint"));
  await waitFor(()=>json(endpoint),r=>r.status===200&&r.body.total===initialTotal+201);
  const rows=[];let total;
  for(let offset=0;;offset+=50){const r=await json(`${endpoint}?offset=${offset}&limit=50`);assert.equal(r.status,200);assert.equal(r.cache,"no-store");assert.equal(r.body.node_id,node.id);assert.equal(r.body.offset,offset);assert.equal(r.body.limit,50);assert.ok(r.body.observed_at&&r.body.read_at);total??=r.body.total;assert.equal(r.body.total,total);rows.push(...r.body.items);if(offset+50>=total)break;}
  assert.equal(rows.length,total);const ids=new Set(rows.map(v=>String(v.cid)));assert.equal(ids.size,total);
  report.baselineConnections=initialTotal;report.testConnections=201;report.observedConnections=total;report.pages=Math.ceil(total/50);
  for(let i=1;i<rows.length;i++)assert.ok(BigInt(rows[i].cid)>BigInt(rows[i-1].cid));
  for(const connection of connections)assert.ok(ids.has(String(connection.id)));
  for(const row of rows)for(const key of Object.keys(row))assert.ok(["cid","pending_bytes","in_msgs","out_msgs","in_bytes","out_bytes","subscriptions"].includes(key));
  report.checks.push("anonymous-denied","201-real-client-identities-reachable-across-pages","node-scoped-ordered-unique-cids-real-total","no-sensitive-client-fields");
  for(const connection of connections){
    const exact=await json(`${endpoint}/${connection.id}`);assert.equal(exact.status,200);assert.equal(exact.cache,"no-store");assert.equal(exact.body.node_id,node.id);assert.equal(String(exact.body.item.cid),String(connection.id));assert.ok(exact.body.observed_at&&exact.body.read_at);
    assert.deepEqual(Object.keys(exact.body).sort(),["item","node_id","observed_at","read_at"]);
    for(const key of Object.keys(exact.body.item))assert.ok(["cid","pending_bytes","in_msgs","out_msgs","in_bytes","out_bytes","subscriptions"].includes(key));
    assert.equal(JSON.stringify(exact.body).includes("qualification-must-not-be-exposed"),false);
  }
  assert.equal((await json(`${endpoint}/${connections[0].id}`,false)).status,401);
  const anonymousHead=await fetch(`${endpoint}/${connections[0].id}`,{method:"HEAD",signal:AbortSignal.timeout(2000)});assert.equal(anonymousHead.status,401);await anonymousHead.arrayBuffer();
  for(const suffix of ["0","18446744073709551616",`${connections[0].id}?subs=true`])assert.equal((await json(`${endpoint}/${suffix}`)).status,400);
  const absent=await json(`${endpoint}/18446744073709551615`);assert.equal(absent.status,404);assert.equal(absent.body.error.code,"connection_not_found");
  const missingNode=await json(`${base}/api/v1/nodes/missing/connections/${connections[0].id}`);assert.equal(missingNode.status,404);assert.equal(missingNode.body.error.code,"not_found");
  report.exactDetailReads=connections.length;
  report.checks.push("all-201-real-clients-exact-detail-no-total-or-sensitive-fields","detail-auth-cid-bounds-and-query-rejection","missing-node-distinct-from-missing-max-uint64-cid");
  const subscriptionClient=connections[0];
  let commands="";for(let i=0;i<1000;i++)commands+=`SUB qualification.scale.${i%10} ${String(i+1).padStart(4,"0")}\r\n`;
  subscriptionClient.socket.write(commands);await subscriptionClient.ping();
  const subscriptionEndpoint=`${endpoint}/${subscriptionClient.id}/subscriptions`;
  const apiMillis=[],pingMillis=[],rssBytes=[];let pingRunning=true,memoryRunning=true;
  const readRSS=async()=>{const response=await fetch(`http://127.0.0.1:${monitorPort}/varz`,{signal:AbortSignal.timeout(1000)}),body=parseJSON(await response.text());assert.equal(response.status,200);assert.ok(Number.isSafeInteger(body.mem)&&body.mem>0);return body.mem;};
  const baselineRSS=await readRSS();
  const pingLoop=(async()=>{while(pingRunning){const started=performance.now();await subscriptionClient.ping();pingMillis.push(performance.now()-started);await new Promise(resolve=>setImmediate(resolve));}})();
  const memoryLoop=(async()=>{while(memoryRunning){rssBytes.push(await readRSS());await new Promise(resolve=>setImmediate(resolve));}})();
  for(let i=0;i<50;i++){
    const apiStarted=performance.now(),result=await json(subscriptionEndpoint);apiMillis.push(performance.now()-apiStarted);assert.equal(result.status,200);assert.equal(result.body.items.length,1000);assert.equal(new Set(result.body.items.map(item=>item.sid)).size,1000);
  }
  pingRunning=false;memoryRunning=false;await Promise.all([pingLoop,memoryLoop]);assert.ok(pingMillis.length>=50);assert.ok(rssBytes.length>=10);
  const summarize=values=>({samples:values.length,min_millis:Math.min(...values),p50_millis:percentile(values,.5),p95_millis:percentile(values,.95),p99_millis:percentile(values,.99),max_millis:Math.max(...values)});
  const peakRSS=Math.max(...rssBytes);
  report.subscriptionScale={subscriptions:1000,reads:50,api:summarize(apiMillis),sameClientPing:summarize(pingMillis),natsRSS:{samples:rssBytes.length,baseline_bytes:baselineRSS,peak_bytes:peakRSS,peak_delta_bytes:Math.max(0,peakRSS-baselineRSS)},thresholds:{api_max_millis:2000,same_client_ping_max_millis:250,nats_rss_peak_delta_bytes:64*1024*1024},host:{platform:platform(),release:release(),arch:arch(),logical_cpus:cpus().length,total_memory_bytes:totalmem()},node:process.version};
  assert.ok(report.subscriptionScale.api.max_millis<report.subscriptionScale.thresholds.api_max_millis);assert.ok(report.subscriptionScale.sameClientPing.max_millis<report.subscriptionScale.thresholds.same_client_ping_max_millis);assert.ok(report.subscriptionScale.natsRSS.peak_delta_bytes<report.subscriptionScale.thresholds.nats_rss_peak_delta_bytes);
  report.checks.push("1000-subscription-complete-read-50-samples","1000-subscription-client-lock-ping-bounded","1000-subscription-nats-rss-peak-delta-bounded");
  const removed=connections.pop();removed.socket.destroy();
  await waitFor(()=>json(endpoint),r=>r.status===200&&r.body.total===total-1);
  const closedDetail=await json(`${endpoint}/${removed.id}`);assert.equal(closedDetail.status,404);assert.equal(closedDetail.body.error.code,"connection_not_found");
  const survivor=await json(`${endpoint}/${connections[0].id}`);assert.equal(survivor.status,200);assert.equal(String(survivor.body.item.cid),String(connections[0].id));
  report.checks.push("closed-cid-detail-missing-with-surviving-cid-still-readable");
  const last=await json(`${endpoint}?offset=${Math.floor((total-2)/50)*50}&limit=50`);assert.equal(last.status,200);assert.ok(!last.body.items.some(v=>String(v.cid)===String(removed.id)));
  const beyond=await json(`${endpoint}?offset=1000&limit=50`);assert.equal(beyond.status,200);assert.equal(beyond.body.offset,1000);assert.equal(beyond.body.total,total-1);assert.deepEqual(beyond.body.items,[]);
  assert.equal((await json(`${endpoint}?q=qualification`)).status,400);assert.equal((await json(`${base}/api/v1/nodes/missing/connections`)).status,404);
  report.checks.push("closed-client-decreases-real-total-and-disappears","out-of-range-page-preserves-offset-and-total","unsupported-search-rejected","missing-node-not-empty-success");
  const identitySearch=`${endpoint}/search`,nameBody={kind:"name",value:"qualification-must-not-be-exposed",offset:0,limit:200};
  const nameSearch=await postJSON(identitySearch,nameBody);assert.equal(nameSearch.status,200);assert.equal(nameSearch.body.total,connections.length);assert.equal(nameSearch.body.items.length,connections.length);assert.equal(JSON.stringify(nameSearch.body).includes("qualification-must-not-be-exposed"),false);
  report.checks.push("client-name-search-real-bounded-full-node-filter-no-name-echo");
  const beforeIdentityScale=await json(endpoint),additional=1000-beforeIdentityScale.body.total;assert.ok(additional>=0);
  for(let i=0;i<additional;i++)connections.push(await connect(natsPort));
  await waitFor(()=>json(endpoint),r=>r.status===200&&r.body.total===1000);
  const identityBody={kind:"account",value:"$G",offset:0,limit:200},identityStarted=performance.now();
  const identityFirst=await postJSON(identitySearch,identityBody);assert.equal(identityFirst.status,200);assert.equal(identityFirst.cache,"no-store");assert.equal(identityFirst.body.total,1000);assert.equal(identityFirst.body.items.length,200);assert.equal(JSON.stringify(identityFirst.body).includes("$G"),false);
  const identityLast=await postJSON(identitySearch,{...identityBody,offset:800});assert.equal(identityLast.status,200);assert.equal(identityLast.body.total,1000);assert.equal(identityLast.body.items.length,200);
  const overLimit=await connect(natsPort);connections.push(overLimit);await waitFor(()=>json(endpoint),r=>r.status===200&&r.body.total===1001);
  const identityOver=await postJSON(identitySearch,identityBody);assert.equal(identityOver.status,422);assert.equal(identityOver.body.error.code,"connection_search_limit");
  report.identityScale={identity_kind:"account",connections:1000,first_and_last_page_millis:performance.now()-identityStarted,over_limit_connections:1001,over_limit_status:identityOver.status};
  report.checks.push("identity-search-exact-1000-filtered-total-first-last-page-no-echo","identity-search-1001-explicit-limit-no-partial-result");
  assert.deepEqual(await Promise.all(binaries.map(fingerprint)),inputs);report.inputsVerifiedAt=new Date().toISOString();report.passed=true;
}catch(error){report.error=error.stack;process.exitCode=1;}
finally{
  client.abort();for(const socket of sockets)socket.destroy();
  for(const child of children.reverse()){if(child.exitCode===null&&child.signalCode===null){const done=once(child,"exit");child.kill();await done;}}
  report.finishedAt=new Date().toISOString();await writeFile(path.join(evidence,"report.json"),JSON.stringify(report,null,2));
  console.log(`${report.passed?"Passed":"FAILED"}: ${evidence}`);if(report.error)console.error(report.error);
}
