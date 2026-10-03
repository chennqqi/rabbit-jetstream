export function hasAuthenticatedConsole(phase,identity){
  return phase==="authenticated"&&!!identity;
}
