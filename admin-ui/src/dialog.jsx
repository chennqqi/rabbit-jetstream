import React, {useCallback, useEffect, useRef, useState} from "react";

const labels = {
  en: {confirm: "Confirm", cancel: "Cancel", ok: "OK"},
  zh: {confirm: "确认", cancel: "取消", ok: "知道了"},
};

let dialogSequence = 0;

// Promise-based in-app replacement for window.confirm/window.alert/
// window.prompt. Native dialogs cannot carry the evidence-safety guidance
// this console requires and they block the renderer. One dialog per host;
// opening a second resolves the pending one as cancelled (prompt: null) so a
// suspended flow can never resume silently.
export function useDialog() {
  const [request, setRequest] = useState(null);
  const pending = useRef(null), restore = useRef(null);
  const open = useCallback((mode, message, options = {}) => new Promise(resolve => {
    const previous = pending.current;
    const request = {id: ++dialogSequence, mode, message, options, resolve};
    // settle lives on the request so the rendered dialog closes exactly the
    // instance it was opened with; a stale settle after a replacement opened
    // is a no-op.
    request.settle = value => {
      if (pending.current !== request) return;
      pending.current = null;
      setRequest(null);
      request.resolve(value);
      const focus = restore.current;
      if (focus && typeof focus.focus === "function") focus.focus();
    };
    pending.current = request;
    if (!previous) restore.current = document.activeElement;
    setRequest(request);
    previous?.settle(mode === "prompt" ? null : false);
  }), []);
  const confirm = useCallback((message, options = {}) => open("confirm", message, options), [open]);
  const alert = useCallback((message, options = {}) => open("alert", message, options), [open]);
  const prompt = useCallback((message, options = {}) => open("prompt", message, options), [open]);
  return {dialog: request, confirm, alert, prompt};
}

export function DialogHost({dialog, language}) {
  const cancel = useRef(null), accept = useRef(null), input = useRef(null);
  useEffect(() => {
    if (!dialog) return;
    (dialog.mode === "confirm" ? cancel : accept).current?.focus();
    const key = event => {
      if (event.key === "Escape") {event.preventDefault(); dialog.settle(dialog.mode === "prompt" ? null : false);}
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [dialog]);
  if (!dialog) return null;
  const text = labels[language] ?? labels.en;
  const danger = dialog.options?.tone === "danger";
  return <div className="dialog-backdrop">
    <section key={dialog.id} className="dialog-panel" role="alertdialog" aria-modal="true" aria-labelledby="rjs-dialog-message">
      <p id="rjs-dialog-message">{dialog.message}</p>
      {dialog.mode === "prompt" && <input ref={input} defaultValue={dialog.options?.defaultValue ?? ""} aria-label={dialog.options?.inputLabel ?? dialog.message} spellCheck="false"/>}
      <div className="dialog-actions">
        {dialog.mode !== "alert" && <button ref={cancel} type="button" onClick={() => dialog.settle(dialog.mode === "prompt" ? null : false)}>{text.cancel}</button>}
        <button ref={accept} type="button" className={danger ? "dialog-confirm-danger" : undefined} onClick={() => dialog.settle(dialog.mode === "prompt" ? (input.current ? input.current.value : null) : true)}>{dialog.mode === "alert" ? text.ok : (dialog.options?.confirmLabel ?? text.confirm)}</button>
      </div>
    </section>
  </div>;
}
