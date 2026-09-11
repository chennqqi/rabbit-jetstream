import React,{useEffect,useRef,useState} from "react";
import {IconHome,IconList,IconDatabase,IconServer,IconFileText,IconPlus,IconMenu2,IconX,IconSettings,IconUsers,IconStack2,IconDownload,IconBell} from "@tabler/icons-react";

export function ConsoleNavigation({language,route,router,permissions}) {
  const zh=language==="zh";
  const [open,setOpen]=useState(false),toggle=useRef(null),navigation=useRef(null);
  useEffect(()=>{
    setOpen(false);
    if(navigation.current?.contains(document.activeElement)&&toggle.current?.offsetParent!==null)toggle.current?.focus();
  },[route]);
  useEffect(()=>{
    const media=window.matchMedia("(max-width: 850px)");
    let ownedFocus=navigation.current?.contains(document.activeElement);
    const rememberFocus=event=>{ownedFocus=navigation.current?.contains(event.target);};
    const changed=()=>{
      // CSS can hide the focused link before matchMedia's change callback runs.
      const focused=navigation.current?.contains(document.activeElement)||(ownedFocus&&document.activeElement===document.body);
      setOpen(false);
      if(focused){if(media.matches)toggle.current?.focus();else navigation.current?.querySelector('a[aria-current="page"]')?.focus();}
    };
    document.addEventListener("focusin",rememberFocus);
    document.addEventListener("pointerdown",rememberFocus);
    media.addEventListener("change",changed);return()=>{
      document.removeEventListener("focusin",rememberFocus);
      document.removeEventListener("pointerdown",rememberFocus);
      media.removeEventListener("change",changed);
    };
  },[]);
  const entries=[
    ["/admin/overview",zh?"总览":"Overview",IconHome,["overview"]],
    ["/admin/queues",zh?"Queue 列表":"Queue list",IconList,["queues","queue","edit-queue","delete-queue"]],
    ["/admin/streams",zh?"Stream 列表":"Stream list",IconDatabase,["streams","stream","consumer"]],
    ["/admin/consumers",zh?"Consumer 列表":"Consumer list",IconUsers,["consumers"]],
    ["/admin/nodes",zh?"节点列表":"Node list",IconServer,["nodes","node","node-connections","node-connection"]],
    ...(permissions.includes("audit:read")?[["/admin/audit",zh?"审计":"Audit",IconFileText,["audit"]]]:[]),
    ...(permissions.includes("queue:apply")?[["/admin/queues/new",zh?"创建 Queue":"Create Queue",IconPlus,["create-queue"]]]:[]),
    ...(permissions.includes("queue:apply")?[["/admin/queues/bulk-change",zh?"批量变更":"Bulk changes",IconStack2,["bulk-change"]]]:[]),
    ...(permissions.includes("diagnostics:create")?[["/admin/diagnostics",zh?"诊断包":"Diagnostics",IconDownload,["diagnostics"]]]:[]),
    ["/admin/alerts",zh?"运维告警":"Operational alerts",IconBell,["alerts"]],
    ["/admin/settings",zh?"访问与设置":"Access and settings",IconSettings,["settings"]],
    ...(permissions.includes("access:manage")?[["/admin/access",zh?"租户访问管理":"Tenant access",IconUsers,["access"]]]:[]),
    ["/admin/compatibility",zh?"兼容性":"Compatibility",IconFileText,["compatibility"]],
  ];
  return <nav ref={navigation} className={`primary-nav${open?" is-open":""}`} aria-label={zh?"主导航":"Primary navigation"} onKeyDown={event=>{if(event.key==="Escape"&&open){event.preventDefault();setOpen(false);toggle.current?.focus();}}}>
    <div className="nav-brand"><strong>RJS</strong><span>Rabbit-JetStream</span></div>
    <button ref={toggle} className="navigation-toggle" type="button" aria-label={zh?"导航菜单":"Navigation menu"} aria-expanded={open} aria-controls="console-navigation-links" onClick={()=>setOpen(value=>!value)}>{open?<IconX size={22} aria-hidden="true"/>:<IconMenu2 size={22} aria-hidden="true"/>}<span>{zh?"导航":"Navigation"} · {entries.find(([, , ,kinds])=>kinds.includes(route.kind))?.[1]??(zh?"页面":"Page")}</span></button>
    <div id="console-navigation-links">{entries.map(([href,label,Icon,kinds])=><a key={href} href={href} aria-current={kinds.includes(route.kind)?"page":undefined} onClick={event=>{if(event.button===0&&!event.ctrlKey&&!event.metaKey&&!event.shiftKey&&!event.altKey){event.preventDefault();setOpen(false);if(toggle.current?.offsetParent!==null)toggle.current?.focus();router.navigate(href);}}}><Icon size={22} stroke={1.8} aria-hidden="true"/>{label}</a>)}</div>
  </nav>;
}
