// Test-only fixed reference data. Never imported by admin-ui/src or a server binary.
export const selectedFixture={
  queue:"orders_events",stream:"RJSQ_orders_events",consumer:"RJSC_orders_events",
  instant:"2026-09-09T08:20:00.000Z",etag:'"12"',
  referenceSHA256:"91e82cd2ef7bdcbadeef899053c9c06257c14c6c06243d8442a2dcceeafca4ea",
};
const {queue,stream,consumer}=selectedFixture;
const plan={apiVersion:"rabbit-jetstream.io/plan/v1alpha1",queue,revision:"synthetic-content-revision-not-kv-12",
  stream:{name:stream,subjects:["orders.events"],storage:"file",replicas:3,maxAgeNanos:86400000000000,maxBytes:0,maxMessages:0},
  consumer:{name:consumer,stream,mode:"pull",filterSubjects:["orders.events"],ackWaitNanos:30000000000,maxDeliver:5},
  routing:[],dependencies:[],warnings:[]};
const document={apiVersion:"rabbit-jetstream.io/v1alpha1",kind:"Queue",metadata:{name:queue},spec:{subjects:["orders.events"],storage:"file",replicas:3,retention:{maxAge:"24h"},delivery:{ackWait:"30s",maxDeliver:5}}};

export function selectedFixtureResponse(method,target) {
  if(method!=="GET")return {status:405,body:{error:{code:"fixture_read_only",message:"Synthetic fixture rejects all writes"}}};
  const url=new URL(target,"http://fixture.invalid");
  if(url.search)return {status:404,body:{error:{code:"fixture_not_defined"}}};
  const responses={
    "/api/v1/session":{actor:"synthetic-visual-operator",role:"operator",permissions:["queue:preview","queue:apply","audit:read"],resource_read_policy:"authenticated",expires_at:null},
    [`/api/v1/queues/${queue}`]:{queue,revision:plan.revision,plan,document},
    [`/api/v1/streams/${stream}`]:{name:stream,subjects:["orders.events"],storage:"file",replicas:3,messages:12480,bytes:2048000,consumers:1,cluster:{name:"demo-cluster",leader:"nats-1",replicas:[{name:"nats-2",current:true,offline:false,lag:0,active_nanos:1000000},{name:"nats-3",current:false,offline:true}]}},
    [`/api/v1/streams/${stream}/consumers/${consumer}`]:{name:consumer,stream,mode:"pull",durable:consumer,filter_subjects:["orders.events"],pending:8420,ack_pending:240,ack_policy:"explicit",ack_wait_nanos:30000000000,max_deliver:5},
  };
  const body=responses[url.pathname];
  return body?{status:200,headers:url.pathname===`/api/v1/queues/${queue}`?{etag:selectedFixture.etag}:{},body:structuredClone(body)}:{status:404,body:{error:{code:"fixture_not_defined"}}};
}
