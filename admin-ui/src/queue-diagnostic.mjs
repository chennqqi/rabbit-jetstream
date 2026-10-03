// Only a current, explicit read-only preview validation response is eligible.
// Never interpret a write rejection as proof that the original write failed.
export function queueDiagnostic(state) {
  const error=state.error,body=error?.body?.error;
  if(state.phase!=="preview-error"||state.requestId||error?.status!==400||
    !["invalid_queue","dlq_dependency_cycle"].includes(error?.code)||body?.code!==error.code||
    typeof body.message!=="string"||!body.message.trim())return null;
  const dependencyCycle=error.code==="dlq_dependency_cycle";
  const limit=16384;
  const fields=!dependencyCycle&&body.issues_version==="rjs.queue-validation.v1"&&Array.isArray(body.issues)&&body.issues.length<=256&&
    body.issues.every(issue=>issue&&typeof issue.path==="string"&&issue.path.length<=512&&
      /^(?:\/(?:[^~\/]|~[01])*)+$/.test(issue.path)&&typeof issue.code==="string"&&/^[a-z_]{1,64}$/.test(issue.code)&&
      typeof issue.message==="string"&&issue.message.length>0&&issue.message.length<=4096)
    ?body.issues.map(({path,code,message})=>({path,code,message})):null;
  return {...(dependencyCycle?{dependencyCycle:true}:{}),...(fields?{fields}:{}),message:body.message.slice(0,limit),truncated:body.message.length>limit,
    requestId:typeof error.requestId==="string"&&error.requestId.length<=256?error.requestId:null};
}
