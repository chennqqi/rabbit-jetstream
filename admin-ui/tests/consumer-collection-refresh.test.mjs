import test from "node:test";
import assert from "node:assert/strict";
import {createConsumerCollectionRefresh} from "../src/consumer-collection-refresh.mjs";
const settle=()=>new Promise(resolve=>setImmediate(resolve));
const query={q:"",mode:"",order:"asc",offset:0,limit:25};
function fixture(kind,requested=query){
  const timers=new Map(),calls=[];let id=0,total=1,revision='"1"',override;
  const body=params=>{
    const offset=Math.min(Number(params.get("offset")),total),limit=Number(params.get("limit"));
    const items=Array.from({length:Math.min(limit,total-offset)},(_,i)=>{
      const value={name:`c${offset+i}`,stream:"S",mode:"pull",filter_subjects:[],pending:9007199254740993n};
      return kind==="stream"?value:{name:value.name,stream:"S",expected:null,observed:value,status:"present",ownership:"unknown"};
    });
    return {items,total,offset,limit,...(kind==="queue"?{queue:"q",stream:"S",declaration_revision:revision,stream_status:"present",stream_ownership:"matching"}:{})};
  };
  const view=createConsumerCollectionRefresh({request:(path,options)=>{calls.push({path,options});return override?override():Promise.resolve({body:body(new URL(path,"http://localhost").searchParams)});}},kind,kind==="queue"?"q":"S",requested,{schedule:(fn,ms)=>{timers.set(++id,{fn,ms});return id;},unschedule:id=>timers.delete(id)});
  return {...view,timers,calls,page:()=>kind==="queue"?view.model.snapshot().page:view.model.snapshot().resource,setTotal:value=>total=value,setRevision:value=>revision=value,reply:fn=>override=fn,async tick(){assert.equal(timers.size,1);const [id,{fn}]=[...timers][0];timers.delete(id);fn();await settle();}};
}
test("Consumer collection refresh preserves query and unclamped offset across shrink/regrowth",async()=>{
  for(const kind of ["queue","stream"]){
    const requested={...query,q:"c",mode:"pull",order:"desc",offset:200},f=fixture(kind,requested);requested.offset=0;
    f.refresh.start();await settle();assert.equal(f.page().offset,1);f.setTotal(205);await f.tick();assert.equal(f.page().offset,200);assert.equal(f.page().items.length,5);
    assert.equal(f.calls[0].path,f.calls[1].path);assert.equal(new URL(f.calls[1].path,"http://localhost").searchParams.get("offset"),"200");
    assert.ok(f.calls.every(call=>!call.options.method&&call.path.includes("/consumers?")));f.clear();
  }
});
test("Consumer collection failure retains exact old page/time; recovery can change declaration revision",async()=>{
  for(const kind of ["queue","stream"]){
    const f=fixture(kind);f.refresh.start();await settle();const page=f.page(),readAt=f.model.snapshot().readAt;
    f.reply(async()=>{throw {status:503};});await f.tick();assert.equal(f.page(),page);assert.equal(f.model.snapshot().readAt,readAt);assert.equal([...f.timers.values()][0].ms,20000);
    assert.equal(kind==="queue"?page.items[0].observed.pending:page.items[0].pending,9007199254740993n);
    f.reply(null);f.setRevision('"2"');await f.tick();assert.equal([...f.timers.values()][0].ms,10000);
    if(kind==="queue")assert.equal(f.page().declaration_revision,'"2"');f.clear();
  }
});
test("Consumer collection missing/denied/disabled/rejected/invalid and Queue conflict clear old results",async()=>{
  for(const kind of ["queue","stream"]){
    for(const error of [{status:401},{status:403},{status:404,code:"not_found"},{status:404,code:"read_api_disabled"},{status:400},"invalid",...(kind==="queue"?[{status:409}]:[])]){
      const f=fixture(kind);f.refresh.start();await settle();f.reply(async()=>{if(error==="invalid")return {body:{}};throw error;});await f.tick();assert.equal(f.page(),null);assert.equal(f.model.snapshot().readAt,null);f.clear();
    }
  }
});
test("changed Consumer query cannot retain a previous query page even on unavailable response",async()=>{
  for(const kind of ["queue","stream"]){
    for(const change of [{q:"other"},{mode:"push"},{order:"desc"},{offset:25},{limit:50}]){
      const f=fixture(kind);f.refresh.start();await settle();f.refresh.stop();f.reply(async()=>{throw {status:503};});await f.model.load({...query,...change},{restore:true});assert.equal(f.page(),null);assert.equal(f.model.snapshot().readAt,null);f.clear();
    }
  }
});
test("Consumer collection disposal aborts and fences late completion; clicks remain single flight",async()=>{
  for(const kind of ["queue","stream"]){
    for(const failed of [false,true]){
      const f=fixture(kind);let finish,reject;f.reply(()=>new Promise((yes,no)=>{finish=yes;reject=no;}));f.refresh.start();await f.refresh.refresh();assert.equal(f.calls.length,1);
      f.clear();assert.equal(f.calls[0].options.signal.aborted,true);if(failed)reject({status:503});else finish({body:{}});await settle();assert.equal(f.page(),null);assert.equal(f.timers.size,0);assert.equal(f.model.snapshot().phase,"idle");
    }
  }
});
test("Consumer collection manual and hidden modes do not issue catch-up or periodic reads",async()=>{
  for(const kind of ["queue","stream"]){
    const f=fixture(kind);f.refresh.setInterval(0);f.refresh.start(true);assert.equal(f.calls.length,0);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);assert.equal(f.timers.size,0);
    f.refresh.visibility(true);await f.refresh.refresh();assert.equal(f.calls.length,1);f.refresh.visibility(false);await settle();assert.equal(f.calls.length,1);
    await f.refresh.refresh();assert.equal(f.calls.length,2);f.refresh.setInterval(10000);assert.equal(f.timers.size,1);f.clear();
  }
});
test("Queue Consumer pages reject unrenderable detail-link identities",async()=>{
  for(const field of ["name","stream"]){
    const f=fixture("queue");f.refresh.start();await settle();const page=f.page();
    if(field==="stream"){page.stream="a/b";page.items[0].stream="a/b";page.items[0].observed.stream="a/b";}
    else{page.items[0].name="a/b";page.items[0].observed.name="a/b";}
    f.reply(async()=>({body:page}));await f.tick();assert.equal(f.model.snapshot().failure,"invalid");assert.equal(f.page(),null);f.clear();
  }
});
