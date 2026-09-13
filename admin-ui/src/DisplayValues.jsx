import React from "react";
import {durationDisplay, readTimeDisplay} from "./display-values.mjs";
import {commonDisplayLabels} from "./common-display-labels.mjs";
export function ExactDuration({value, language}) { const display = durationDisplay(value); return display === null ? <>{commonDisplayLabels(language).unknown}</> : <span title={`${value} ns`}>{display}</span>; }
export function ReadTime({value, language}) { const display = readTimeDisplay(value); return display === null ? <>{commonDisplayLabels(language).unknown}</> : <time dateTime={value} title={value}>{display}</time>; }
