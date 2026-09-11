import {stringifyJSON} from "./api.mjs";

// Representation comparison only; semantic declaration diff comes from Go.
export function normalizationReview(submitted,normalized) {
  const rows=[],limit=256;
  let truncated=false;
  const object=value=>value!==null&&typeof value==="object"&&!Array.isArray(value);
  const escape=value=>value.replaceAll("~","~0").replaceAll("/","~1");
  function walk(from,to,path,hasFrom,hasTo){
    if(truncated)return;
    if((object(from)||!hasFrom)&&(object(to)||!hasTo)){
      const keys=[...new Set([...Object.keys(from??{}),...Object.keys(to??{})])].sort();
      if(keys.length){
        for(const key of keys)walk(from?.[key],to?.[key],`${path}/${escape(key)}`,hasFrom&&Object.hasOwn(from,key),hasTo&&Object.hasOwn(to,key));
        return;
      }
    }
    const before=hasFrom?stringifyJSON(from):undefined,after=hasTo?stringifyJSON(to):undefined;
    if(hasFrom===hasTo&&before===after)return;
    if(rows.length===limit){truncated=true;return;}
    rows.push({path:path||"/",kind:!hasFrom?"added":!hasTo?"omitted":"different",before,after});
  }
  walk(submitted,normalized,"",true,true);
  return {rows,truncated};
}
