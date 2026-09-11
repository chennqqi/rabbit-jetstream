export const refreshPreferenceKey="rjs.overview-refresh-seconds";
export const refreshChoices=Object.freeze([0,10,30,60]);
export const validRefreshPreference=value=>refreshChoices.includes(value);
export function readRefreshPreference(storage=()=>globalThis.localStorage){
  try{const saved=storage()?.getItem(refreshPreferenceKey);for(const value of refreshChoices)if(saved===String(value))return value;}catch{}
  return 10;
}
export function saveRefreshPreference(value,storage=()=>globalThis.localStorage){
  if(!validRefreshPreference(value))return false;
  try{const target=storage();if(!target)return false;target.setItem(refreshPreferenceKey,String(value));return true;}catch{return false;}
}
