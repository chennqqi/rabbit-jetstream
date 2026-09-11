import React from "react";
import {ConsoleCapabilities} from "./ConsoleCapabilities.jsx";
import {refreshChoices} from "./refresh-preference.mjs";

export function Settings({api,identity,language,onLanguage,onClear,refreshSeconds,onRefresh,refreshSaved}) {
  const t=(en,zh)=>language==="zh"?zh:en;
  return <section aria-labelledby="settings-heading">
    <h2 id="settings-heading">{t("Access and settings","访问与设置")}</h2>
    <h3>{t("Verified session","已验证会话")}</h3>
    <dl>
      <dt>{t("Actor","身份")}</dt><dd>{identity.actor}</dd>
      <dt>{t("Role","角色")}</dt><dd>{identity.role}</dd>
      <dt>{t("Verified expiry","已验证到期时间")}</dt><dd>{identity.expires_at??t("Unknown; not proof of an unlimited credential lifetime","未知，不代表凭据永久有效")}</dd>
      <dt>{t("Resource-read policy","资源读取策略")}</dt><dd>{identity.resource_read_policy}</dd>
    </dl>
    <p>{t("This is the identity verified at sign-in, not continuous proof of authorization. The server authorizes each request. An anonymous read policy does not allow anonymous writes or audit access.","这是登录时验证的身份，不是持续授权证明。服务端会校验每个请求。匿名读取策略不代表允许匿名写入或访问审计。")}</p>
    <h3>{t("Reported permissions","服务端报告的权限")}</h3>
    {identity.permissions.length?<ul aria-label={t("Reported permissions","服务端报告的权限")}>{identity.permissions.map((permission,index)=><li key={`${permission}-${index}`}><code>{permission}</code></li>)}</ul>:<p>{t("No permissions reported.","未报告权限。")}</p>}
    <p>{t("Permissions do not prove feature availability, resource ownership, cluster health or production qualification. Some permitted workflows are not yet implemented in this candidate.","权限不证明功能可用、资源所有权、集群健康或生产资格。部分已授权流程尚未在候选界面实现。")}</p>
    <h3>{t("Local preferences and session","本机偏好与会话")}</h3>
    <p>{t("Language: English. Only language and read-page refresh preferences are saved locally.","语言：简体中文。仅在本机保存语言和读取页面刷新偏好。")}</p>
    <button onClick={onLanguage}>{t("Switch interface language","切换界面语言")}</button>
    <label htmlFor="overview-refresh-interval">{t("Read-page refresh interval","读取页面刷新间隔")}</label>
    <select id="overview-refresh-interval" value={refreshSeconds} onChange={event=>onRefresh(Number(event.target.value))}>{refreshChoices.map(seconds=><option key={seconds} value={seconds}>{seconds===0?t("Manual only","仅手动"):`${seconds} ${t("seconds","秒")}`}</option>)}</select>
    <p>{t("Applies to Overview, Queue summaries, Queue/Stream lists and their Consumer collections, Stream configuration/state, node pages and standalone Consumer detail. Manual mode still reads on entry. Hidden tabs pause; failures back off up to 60 seconds. Drafts, previews and mutations are never refreshed by this preference. Other pages retain their existing refresh behavior.","作用于总览、Queue 摘要、Queue/Stream 列表及其 Consumer 集合、Stream 配置/状态、节点页面及独立 Consumer 详情。手动模式进入页面时仍读取一次。隐藏标签页暂停，失败退避最长 60 秒。此偏好不刷新草稿、预览或写入，其他页面保持原有刷新行为。")}</p>
    {!refreshSaved&&<p role="status">{t("Local storage is unavailable. Refresh preference applies to this page session only and was not saved.","本机存储不可用，刷新偏好仅在当前页面会话生效，未保存。")}</p>}
    <p>{t("Credentials, drafts and request evidence stay in memory. Clearing this session discards them, preserves language and refresh preferences, and does not revoke credentials or cancel/roll back server operations.","凭据、草稿和请求证据仅在内存。清除此会话会丢弃它们，保留语言和刷新偏好，但不会撤销凭据或取消/回滚服务端操作。")}</p>
    <button onClick={onClear}>{t("Clear this session","清除此会话")}</button>
    <ConsoleCapabilities api={api} language={language}/>
    <h3>{t("Not yet provided","尚未提供")}</h3>
    <p>{t("Production qualification evidence is not supplied here. Reachable node count is not used to infer qualification.","此处尚不提供生产资格证据，不会通过可达节点数量推断资格。")}</p>
  </section>;
}
