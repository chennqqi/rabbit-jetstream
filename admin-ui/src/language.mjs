export const languageKey="rjs.language";
const valid=value=>value==="en"||value==="zh";
export function readLanguage(browserLanguage,storage=()=>globalThis.localStorage){
  try{const saved=storage()?.getItem(languageKey);if(valid(saved))return saved;}catch{}
  return typeof browserLanguage==="string"&&/^zh(?:-|$)/i.test(browserLanguage)?"zh":"en";
}
export function saveLanguage(language,storage=()=>globalThis.localStorage){
  if(!valid(language))return false;
  try{const target=storage();if(!target)return false;target.setItem(languageKey,language);return true;}catch{return false;}
}
