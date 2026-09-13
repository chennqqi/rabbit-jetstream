import React, {useEffect, useRef, useState} from "react";
import {IconHome, IconList, IconDatabase, IconServer, IconFileText, IconPlus, IconMenu2, IconX, IconSettings, IconUsers, IconStack2, IconDownload, IconBell} from "@tabler/icons-react";
import {consoleNavigationLabels} from "./console-navigation-labels.mjs";

export function ConsoleNavigation({language, route, router, permissions}) {
  const text = consoleNavigationLabels(language);
  const [open, setOpen] = useState(false), toggle = useRef(null), navigation = useRef(null);
  useEffect(() => {
    setOpen(false);
    if (navigation.current?.contains(document.activeElement) && toggle.current?.offsetParent !== null) toggle.current?.focus();
  }, [route]);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 850px)");
    let ownedFocus = navigation.current?.contains(document.activeElement);
    const rememberFocus = event => { ownedFocus = navigation.current?.contains(event.target); };
    const changed = () => {
      const focused = navigation.current?.contains(document.activeElement) || (ownedFocus && document.activeElement === document.body);
      setOpen(false);
      if (focused) { if (media.matches) toggle.current?.focus(); else navigation.current?.querySelector('a[aria-current="page"]')?.focus(); }
    };
    document.addEventListener("focusin", rememberFocus); document.addEventListener("pointerdown", rememberFocus); media.addEventListener("change", changed);
    return () => { document.removeEventListener("focusin", rememberFocus); document.removeEventListener("pointerdown", rememberFocus); media.removeEventListener("change", changed); };
  }, []);
  const entries = [
    ["/admin/overview", text.entries.overview, IconHome, ["overview"]],
    ["/admin/queues", text.entries.queues, IconList, ["queues", "queue", "edit-queue", "delete-queue"]],
    ["/admin/streams", text.entries.streams, IconDatabase, ["streams", "stream", "consumer"]],
    ["/admin/consumers", text.entries.consumers, IconUsers, ["consumers"]],
    ["/admin/nodes", text.entries.nodes, IconServer, ["nodes", "node", "node-connections", "node-connection"]],
    ...(permissions.includes("audit:read") ? [["/admin/audit", text.entries.audit, IconFileText, ["audit"]]] : []),
    ...(permissions.includes("queue:apply") ? [["/admin/queues/new", text.entries.create, IconPlus, ["create-queue"]], ["/admin/queues/bulk-change", text.entries.bulk, IconStack2, ["bulk-change"]]] : []),
    ...(permissions.includes("diagnostics:create") ? [["/admin/diagnostics", text.entries.diagnostics, IconDownload, ["diagnostics"]]] : []),
    ["/admin/alerts", text.entries.alerts, IconBell, ["alerts"]], ["/admin/settings", text.entries.settings, IconSettings, ["settings"]],
    ...(permissions.includes("access:manage") ? [["/admin/access", text.entries.access, IconUsers, ["access"]]] : []),
    ["/admin/compatibility", text.entries.compatibility, IconFileText, ["compatibility"]],
  ];
  const current = entries.find(([, , , kinds]) => kinds.includes(route.kind))?.[1] ?? text.page;
  return <nav ref={navigation} className={`primary-nav${open ? " is-open" : ""}`} aria-label={text.primary} onKeyDown={event => { if (event.key === "Escape" && open) { event.preventDefault(); setOpen(false); toggle.current?.focus(); } }}>
    <div className="nav-brand"><strong>RJS</strong><span>Rabbit-JetStream</span></div>
    <button ref={toggle} className="navigation-toggle" type="button" aria-label={text.menu} aria-expanded={open} aria-controls="console-navigation-links" onClick={() => setOpen(value => !value)}>{open ? <IconX size={22} aria-hidden="true"/> : <IconMenu2 size={22} aria-hidden="true"/>}<span>{text.navigation} · {current}</span></button>
    <div id="console-navigation-links">{entries.map(([href, label, Icon, kinds]) => <a key={href} href={href} aria-current={kinds.includes(route.kind) ? "page" : undefined} onClick={event => { if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) { event.preventDefault(); setOpen(false); if (toggle.current?.offsetParent !== null) toggle.current?.focus(); router.navigate(href); } }}><Icon size={22} stroke={1.8} aria-hidden="true"/>{label}</a>)}</div>
  </nav>;
}
