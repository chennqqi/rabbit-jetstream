import React, {useEffect, useRef} from "react";

export function SessionIdentity({identity, text, onClear, onTenantChange}) {
  const disclosure = useRef(null);
  const summary = useRef(null);
  useEffect(() => {
    const outside = event => {
      if (!disclosure.current?.contains(event.target)) disclosure.current?.removeAttribute("open");
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("focusin", outside);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("focusin", outside);
    };
  }, []);
  return <div className="session-controls">
    <details ref={disclosure} onKeyDown={event => {
      if (event.key === "Escape" && disclosure.current.open) {
        event.preventDefault();
        disclosure.current.open = false;
        summary.current.focus();
      }
    }}>
      <summary ref={summary}>{text.identity}</summary>
      <div className="identity-panel" tabIndex={0} role="region" aria-label={text.identity}>
        <h2>{text.identity}</h2>
        <dl><dt>{text.identity}</dt><dd>{identity.actor}</dd>
          <dt>{text.role}</dt><dd>{identity.role}</dd>
          <dt>{text.expires}</dt><dd>{identity.expires_at ?? text.unknown}</dd>
          <dt>{text.policy}</dt><dd>{identity.resource_read_policy}</dd></dl>
        {identity.tenants.length>0&&<label className="tenant-selector" htmlFor="active-tenant">{text.tenant}<select id="active-tenant" value={identity.active_tenant} onChange={event=>onTenantChange(event.target.value)}>{identity.tenants.map(tenant=><option key={tenant}>{tenant}</option>)}</select></label>}
      </div>
    </details>
    <button onClick={onClear}>{text.clear}</button>
  </div>;
}
