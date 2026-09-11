import test from "node:test";
import assert from "node:assert/strict";
import {queueDiagnostic} from "../src/queue-diagnostic.mjs";
const state={phase:"preview-error",error:{status:400,code:"invalid_queue",requestId:"response-id",body:{error:{code:"invalid_queue",message:"spec.subjects contains duplicate orders.events"}}}};

test("cycle preview diagnostics require matching code and cannot unlock dispatched writes",()=>{
  const cycle={phase:"preview-error",error:{status:400,code:"dlq_dependency_cycle",requestId:"cycle-id",body:{error:{code:"dlq_dependency_cycle",message:"DLQ dependency cycle at downstream"}}}};
  assert.deepEqual(queueDiagnostic(cycle),{dependencyCycle:true,message:"DLQ dependency cycle at downstream",truncated:false,requestId:"cycle-id"});
  for(const phase of ["uncertain","inspecting","accepted","review","editing","conflict"])
    assert.equal(queueDiagnostic({...cycle,phase}),null);
  assert.equal(queueDiagnostic({...cycle,requestId:"dispatched"}),null);
  for(const status of [200,409,503])assert.equal(queueDiagnostic({...cycle,error:{...cycle.error,status}}),null);
  assert.equal(queueDiagnostic({...cycle,error:{...cycle.error,code:"invalid_queue"}}),null);
  const withIssues={...cycle,error:{...cycle.error,body:{error:{...cycle.error.body.error,issues_version:"rjs.queue-validation.v1",issues:[{path:"/spec/deadLetter",code:"cycle",message:"guessed field"}]}}}};
  assert.equal(queueDiagnostic(withIssues).fields,undefined);
});

test("versioned field diagnostics use server pointers without guessing or accepting malformed arrays",()=>{
  const issue={path:"/spec/bindings/1/keys/2",code:"duplicate",message:"Duplicate routing key"};
  const withIssues=(issues,version="rjs.queue-validation.v1")=>({...state,error:{...state.error,body:{error:{...state.error.body.error,issues_version:version,issues}}}});
  assert.deepEqual(queueDiagnostic(withIssues([issue])).fields,[issue]);
  for(const issues of [null,{},[null],[{...issue,path:"spec.foo"}],[{...issue,path:"/bad~2"}],[{...issue,code:"<bad>"}],[{...issue,message:""}],Array(257).fill(issue)])
    assert.equal(queueDiagnostic(withIssues(issues)).fields,undefined);
  assert.equal(queueDiagnostic(withIssues([issue],"future")).fields,undefined);
});

test("preview diagnostic requires matching explicit validation response and preserves server text",()=>{
  assert.deepEqual(queueDiagnostic(state),{message:state.error.body.error.message,truncated:false,requestId:"response-id"});
  for(const phase of ["editing","previewing","review","uncertain","inspecting","accepted","denied","load-error"])
    assert.equal(queueDiagnostic({...state,phase}),null);
  assert.equal(queueDiagnostic({...state,requestId:"already-dispatched"}),null);
  for(const status of [401,403,409,422,500,503])assert.equal(queueDiagnostic({...state,error:{...state.error,status}}),null);
  assert.equal(queueDiagnostic({...state,error:{...state.error,code:"other"}}),null);
  assert.equal(queueDiagnostic({...state,error:{...state.error,body:{error:{code:"other",message:"wrong"}}}}),null);
});

test("diagnostic malformed bodies fail closed; long text has an explicit display bound",()=>{
  for(const body of [null,{},[],{error:"string"},{error:{code:"invalid_queue",message:123}},{error:{code:"invalid_queue",message:"  "}}])
    assert.equal(queueDiagnostic({...state,error:{...state.error,body}}),null);
  const diagnostic=queueDiagnostic({...state,error:{...state.error,requestId:"x".repeat(257),body:{error:{code:"invalid_queue",message:"<script>literal</script>"+"x".repeat(20000)}}}});
  assert.equal(diagnostic.message.length,16384);assert.equal(diagnostic.truncated,true);assert.equal(diagnostic.requestId,null);
  assert.ok(diagnostic.message.startsWith("<script>literal</script>"));
});
