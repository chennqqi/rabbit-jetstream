// Exact RFC3339 instant comparison without rounding nanoseconds to milliseconds.
export function auditTimeNanos(value) {
  const match=/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|([+-])(\d{2}):(\d{2}))$/.exec(value);
  if(!match)throw new TypeError("Invalid audit timestamp");
  const [,date,hour,minute,second,fraction="",zone,sign,offsetHour="0",offsetMinute="0"]=match;
  const midnight=Date.parse(`${date}T00:00:00Z`);
  if(!Number.isFinite(midnight)||new Date(midnight).toISOString().slice(0,10)!==date||+hour>23||+minute>59||+second>59||+offsetHour>23||+offsetMinute>59)throw new TypeError("Invalid audit timestamp");
  const offset=zone==="Z"?0:(sign==="+"?1:-1)*(+offsetHour*60 + +offsetMinute)*60;
  return BigInt(midnight)*1000000n+BigInt(+hour*3600 + +minute*60 + +second-offset)*1000000000n+BigInt(fraction.padEnd(9,"0"));
}
