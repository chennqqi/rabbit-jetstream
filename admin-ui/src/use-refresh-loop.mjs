import {useEffect, useRef, useState} from "react";

// Shared wiring for createRefreshLoop instances: the Settings interval
// preference, visibility pause, the one-second staleness clock and a
// deterministic teardown that always stops the loop before page cleanup.
// The callback must be read-only work and report false on failure; see
// refresh-loop.mjs for the scheduling rules.
export function useRefreshLoop({loop, seconds = 10, cleanup} = {}) {
  const [paused, setPaused] = useState(() => document.hidden);
  const [, setClock] = useState(0);
  const cleanupRef = useRef(cleanup);
  cleanupRef.current = cleanup;
  useEffect(() => {loop.setInterval(seconds * 1000);}, [loop, seconds]);
  useEffect(() => {
    const visibility = () => {setPaused(document.hidden); setClock(Date.now()); loop.visibility(document.hidden);};
    setPaused(document.hidden);
    document.addEventListener("visibilitychange", visibility);
    loop.start(document.hidden);
    const tick = setInterval(() => setClock(Date.now()), 1000);
    return () => {loop.stop(); clearInterval(tick); document.removeEventListener("visibilitychange", visibility); cleanupRef.current?.();};
  }, [loop]);
  return {paused, clock: Date.now()};
}
