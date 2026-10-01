import {parseJSON,stringifyJSON} from "./api.mjs";
import {validateAuditWindow} from "./audit-window.mjs";
import {errorMutationEvidence} from "./mutation-evidence.mjs";
import {readMutationCapabilities,capabilityHeaders,observedCapabilityChange} from "./mutation-capabilities.mjs";
import {createRequestID} from "./request-id.mjs";

const copy=value=>parseJSON(stringifyJSON(value));
const etag=value=>typeof value==="string"&&/^"[1-9][0-9]*"$/.test(value)&&BigInt(value.slice(1,-1))<=18446744073709551615n;
const uint=value=>(typeof value==="bigint"&&value>=0n&&value<=18446744073709551615n)||(Number.isSafeInteger(value)&&value>=0);
const failure=error=>({kind:error.kind??"invalid-response",status:error.status??null,code:error.code??null,
  ...(errorMutationEvidence(error)?{mutation:errorMutationEvidence(error)}:{})});

export function validDeletePreview(value,name,revision,responseETag) {
  if(!value||value.queue!==name||value.stream!==`RJSQ_${name}`||value.base_revision!==revision||responseETag!==revision||!etag(revision)||
    typeof value.stream_present!=="boolean"||typeof value.blocked!=="boolean"||typeof value.requires_force!=="boolean"||
    typeof value.observed_at!=="string"||!Number.isFinite(Date.parse(value.observed_at)))return false;
  if(!value.stream_present)return value.ownership==="unobserved"&&value.messages===undefined&&value.consumers===undefined&&!value.blocked&&!value.requires_force;
  if(!["matching","unmarked","different"].includes(value.ownership)||!uint(value.messages)||!uint(value.consumers))return false;
  const force=BigInt(value.messages)>0n,blocked=value.ownership!=="matching"||force;
  return value.requires_force===force&&value.blocked===blocked&&(!blocked||(typeof value.reason==="string"&&!!value.reason));
}

// Session-owned, single-attempt deletion. Readback never authorizes a retry.
export function createQueueDelete(api,name,{canStart=()=>true,requireCapabilities=api.requireMutationCapabilities===true}={}) {
  if(!/^[A-Za-z0-9_-]+$/.test(name))throw new Error("Invalid Queue name");
  let generation=0,state={name,phase:"idle",typed:"",force:false};
  const listeners=new Set(),url=`/api/v1/queues/${encodeURIComponent(name)}`;
  const emit=next=>{state=next;for(const fn of listeners)fn();};
  const editable=()=>["idle","review","preview-error"].includes(state.phase);
  const allowed=()=>state.phase==="review"&&state.typed===name&&state.acknowledged&&canStart()&&
    (!state.preview.stream_present||state.preview.ownership==="matching")&&(!state.preview.requires_force||state.force);
  const unsubscribeCapabilities=api.subscribeCapabilityReads?.(observation=>{
    if(state.phase==="review"&&observedCapabilityChange(state.capabilities,observation)){
      generation++;emit({...state,phase:"preview-error",preview:undefined,typed:"",force:false,acknowledged:false,error:{code:"capabilities_changed",kind:"capability-change"}});
    }
  });
  return {
    snapshot:()=>state,subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},canSubmit:allowed,
    async preview(){
      if(!editable()||!canStart())return;
      const current=++generation;
      emit({name,phase:"previewing",typed:"",force:false,acknowledged:false});
      try {
        const capabilities=requireCapabilities?await readMutationCapabilities(api,["queue-delete-preview","conditional-queue-writes"]):undefined;
        if(current!==generation)return;
        const declaration=await api.request(url),revision=declaration.headers.get("ETag");
        if(current!==generation)return;
        if(declaration.body?.queue!==name||declaration.body?.plan?.queue!==name||declaration.body?.plan?.stream?.name!==`RJSQ_${name}`||!etag(revision))throw new Error("Invalid deletion base");
        const response=await api.request(`${url}/delete-preview`,{headers:{"If-Match":revision,...capabilityHeaders(capabilities)}});
        if(current!==generation)return;
        if(!validDeletePreview(response.body,name,revision,response.headers.get("ETag")))throw new Error("Invalid deletion preview");
        if(requireCapabilities)await readMutationCapabilities(api,["queue-delete-preview","conditional-queue-writes"],capabilities);
        if(current!==generation)return;
        emit({name,phase:"review",typed:"",force:false,acknowledged:false,etag:revision,preview:copy(response.body),capabilities});
      }catch(error){if(current===generation)emit({name,phase:"preview-error",typed:"",force:false,error:failure(error)});}
    },
    type(value){if(state.phase==="review")emit({...state,typed:value,acknowledged:false});},
    force(value){if(state.phase==="review")emit({...state,force:!!value,typed:"",acknowledged:false});},
    acknowledge(value){if(state.phase==="review")emit({...state,acknowledged:!!value});},
    cancel(){if(!editable())return;generation++;emit({name,phase:"idle",typed:"",force:false});},
    async submit(){
      if(!allowed())return;
      const current=generation;let dispatched=false;
      emit({...state,phase:"submitting",typed:"",acknowledged:false});
      try {
        if(requireCapabilities){
          if(!state.capabilities)throw Object.assign(new Error("Missing preview capabilities"),{code:"capabilities_unavailable"});
          await readMutationCapabilities(api,["queue-delete-preview","conditional-queue-writes"],state.capabilities);
        }
        if(current!==generation)return;
        const requestId=createRequestID();emit({...state,requestId});dispatched=true;
        const response=await api.request(`${url}?force=${state.force}`,{method:"DELETE",headers:{"If-Match":state.etag,...capabilityHeaders(state.capabilities),"X-RJS-Confirm-Queue":name,"X-Request-ID":requestId}});
        if(current!==generation)return;
        const result=response.body;
        if(result?.queue!==name||result?.stream!==state.preview.stream||!["deleted","noop"].includes(result?.status)||result.blocked!==false||result.forced!==state.force||!uint(result.messages))throw new Error("Unrecognized deletion result");
        emit({...state,phase:"accepted",result:copy(result),responseRequestId:response.headers.get("X-Request-ID")});
      }catch(error){if(current===generation)emit(dispatched?{...state,phase:"uncertain",error:failure(error)}:{...state,phase:"preview-error",preview:undefined,typed:"",force:false,acknowledged:false,error:failure(error)});}
    },
    async inspect(){
      if(!["uncertain","accepted"].includes(state.phase))return;
      const current=generation,phase=state.phase,requestId=state.requestId;
      emit({...state,phase:"inspecting",returnPhase:phase});
      const read=async(path,validate)=>{
        try{const response=await api.request(path);validate(response.body);return {status:"available",body:copy(response.body),etag:response.headers.get("ETag"),readAt:new Date().toISOString()};}
        catch(error){return {status:error.kind==="http"&&error.status===404&&error.code==="not_found"?"missing":"unavailable",error:failure(error),readAt:new Date().toISOString()};}
      };
      const [declaration,stream,audit]=await Promise.all([
        read(url,body=>{if(body?.queue!==name)throw new Error("identity");}),
        read(`/api/v1/streams/${encodeURIComponent(state.preview.stream)}`,body=>{if(body?.name!==state.preview.stream)throw new Error("identity");}),
        read(`/api/v1/audit/requests/${encodeURIComponent(requestId)}`,body=>validateAuditWindow(body,requestId)),
      ]);
      if(audit.status==="available")audit.windows=[{body:copy(audit.body),readAt:audit.readAt}];
      if(current===generation)emit({...state,phase,returnPhase:undefined,
        inspectionHistory:state.inspection?[...(state.inspectionHistory??[]),state.inspection]:(state.inspectionHistory??[]),
        inspection:{declaration,stream,audit}});
    },
    async inspectOlderAudit(){
      if(!["uncertain","accepted"].includes(state.phase))return;
      const audit=state.inspection?.audit,cursor=audit?.windows?.at(-1)?.body.nextBefore;
      if(audit?.status!=="available"||cursor===null||cursor===undefined)return;
      const current=generation,phase=state.phase,requestId=state.requestId;
      emit({...state,phase:"inspecting",returnPhase:phase,inspection:{...state.inspection,audit:{...audit,olderError:undefined}}});
      try{
        const response=await api.request(`/api/v1/audit/requests/${encodeURIComponent(requestId)}?before=${cursor}`);
        if(current!==generation)return;
        validateAuditWindow(response.body,requestId,cursor);
        emit({...state,phase,returnPhase:undefined,inspection:{...state.inspection,audit:{...audit,olderError:undefined,
          windows:[...audit.windows,{body:copy(response.body),readAt:new Date().toISOString()}]}}});
      }catch(error){if(current===generation)emit({...state,phase,returnPhase:undefined,inspection:{...state.inspection,audit:{...audit,olderError:failure(error)}}});}
    },
    discard(){if(state.phase==="submitting")throw new Error("Cannot discard a pending deletion");unsubscribeCapabilities?.();generation++;emit({name,phase:"idle",typed:"",force:false});},
  };
}
