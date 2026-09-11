// A completion-based read scheduler: never overlaps batches or queues catch-up
// work. The callback must perform read-only work and report false on failure.
export function createRefreshLoop(read,{schedule=setTimeout,unschedule=clearTimeout,interval=10000}={}){
  let stopped=true,hidden=false,running=false,timer=null,failures=0,generation=0,attempted=false;
  const cancel=()=>{if(timer!==null)unschedule(timer);timer=null;};
  const arm=()=>{cancel();if(!stopped&&!hidden&&!running&&interval>0)timer=schedule(()=>{timer=null;void refresh();},Math.min(60000,interval*2**Math.min(failures,6)));};
  async function refresh(selection){
    if(stopped||hidden||running)return;
    cancel();running=true;attempted=true;const current=generation;
    try{const success=await read(selection);if(current===generation)failures=success===false?failures+1:0;}
    catch{if(current===generation)failures++;}
    finally{running=false;arm();}
  }
  return {
    start(isHidden=false){if(!stopped)return;stopped=false;hidden=isHidden;void refresh();},
    refresh,
    setInterval(value){if(![0,10000,30000,60000].includes(value))return false;interval=value;arm();return true;},
    visibility(isHidden){hidden=isHidden;cancel();if(!hidden){if(!attempted&&interval===0)void refresh();else arm();}},
    stop(){stopped=true;generation++;cancel();},
  };
}
