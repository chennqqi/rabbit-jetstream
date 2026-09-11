import React from "react";
import {durationDisplay,readTimeDisplay} from "./display-values.mjs";

export function ExactDuration({value,language}) {
  const display=durationDisplay(value);
  return display===null?<>{language==="zh"?"未知":"Unknown"}</>:<span title={`${value} ns`}>{display}</span>;
}

export function ReadTime({value,language}) {
  const display=readTimeDisplay(value);
  return display===null?<>{language==="zh"?"未知":"Unknown"}</>:<time dateTime={value} title={value}>{display}</time>;
}
