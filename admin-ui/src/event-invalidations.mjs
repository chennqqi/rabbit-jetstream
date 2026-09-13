export async function runEventInvalidations(api,{signal,onInvalidate,wait=delay}={}){
  let lastEventID,backoff=1000;
  while(!signal?.aborted){
    try{
      const stream=await api.events({lastEventID,signal});
      backoff=1000;
      for await(const event of stream.events){if(signal?.aborted)return;lastEventID=event.id;await onInvalidate?.(event.resource);}
    }catch(error){
      if(signal?.aborted||error.kind==="aborted"||error.status===401||error.status===403)return;
      if(error.status===409){lastEventID=undefined;await onInvalidate?.("audit");await onInvalidate?.("alerts");}
    }
    if(!signal?.aborted){await wait(backoff,signal);backoff=Math.min(backoff*2,30000);}
  }
}

function delay(milliseconds,signal){return new Promise(resolve=>{const timer=setTimeout(done,milliseconds);function done(){signal?.removeEventListener("abort",done);clearTimeout(timer);resolve();}signal?.addEventListener("abort",done,{once:true});});}
