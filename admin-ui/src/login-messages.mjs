// Bilingual catalog for the login, session and topbar shell. English and
// Chinese must keep identical key structure; tests/login-messages.test.mjs
// enforces the parity so strings cannot drift between languages.
export const loginMessages = {
  en: {
    title: "Management console", candidate: "Development candidate — production workflows are still being integrated.",
    username: "Username", password: "Password", login: "Sign in", token: "Recovery bearer token", signIn: "Verify recovery token", recovery: "Recovery or automation token", verifying: "Verifying…", clear: "Clear local session",
    privacy: "The short-lived access token stays in page memory. Reloading clears it. Credentials are never saved in localStorage.",
    identity: "Verified identity", role: "Role", expires: "Verified expiry", unknown: "Unknown", policy: "Resource-read policy", tenant: "Active tenant",
    retainedDraft: "Retained draft and request evidence", deletionEvidence: "Deletion evidence",
    clearSubmitting: "A submission is pending; session cannot be cleared yet.", clearUnknown: "Write outcome is unknown. Clearing discards request evidence and does not cancel or roll back the server operation. Record the request ID first. Clear anyway?", clearDirty: "Clear session and discard all drafts and retained in-memory evidence?", clearConfirm: "Clear session",
    switchSubmitting: "A submission is pending; tenant cannot be changed yet.", switchDirty: "Changing tenant discards this tenant's drafts and in-memory evidence without canceling or rolling back dispatched operations. Save required evidence first. Continue?", switchConfirm: "Switch tenant",
    skipContent: "Skip to page content", ssoLogout: "Sign out from SSO", switchLanguage: "简体中文", storageUnavailable: "Local storage is unavailable. Language changed for this page, but this choice could not be saved.", ssoSignIn: "Sign in with SSO", ssoFailure: "SSO is unavailable or the callback was invalid. Start sign-in again.",
    expiredEvidence: "The verified credential expired; new requests are blocked. Drafts and request evidence remain in page memory. Already dispatched operations are not canceled or rolled back. Record necessary evidence before clearing the session and signing in again.", handoffBlocked: "Operation pending or outcome unknown; no handoff performed.", deletionOwnsQueue: "Deletion review or request evidence is retained for this Queue. Return to deletion to cancel the review; submitted operations require saving evidence and clearing the session.", unavailablePage: "This page is not implemented or the URL is invalid; no create or update is performed.",
    errors: {"missing-token": "Enter a recovery token.", "missing-credentials": "Enter your username and password.", "credentials-rejected": "Username/password or recovery credentials were rejected.", "role-denied": "This identity has no permitted management role.", "auth-disabled": "Local account authentication is not configured.", expired: "The verified credential has expired.", "invalid-response": "The server returned an invalid identity response.", unavailable: "Identity verification is unavailable. Check the server and connection, then retry explicitly."},
  },
  zh: {
    title: "管理控制台", candidate: "开发候选版本 — 生产工作流仍在接入中。",
    username: "用户名", password: "密码", login: "登录", token: "恢复用 Bearer Token", signIn: "验证恢复 Token", recovery: "恢复或自动化 Token", verifying: "正在验证…", clear: "清除本机会话",
    privacy: "短时效 Access Token 仅保存在页面内存，刷新即清除；凭据不会保存到 localStorage。",
    identity: "已验证身份", role: "角色", expires: "已验证到期时间", unknown: "未知", policy: "资源读取策略", tenant: "当前租户",
    retainedDraft: "保留的草稿与请求证据", deletionEvidence: "删除证据",
    clearSubmitting: "提交正在进行，暂不能清除会话。", clearUnknown: "写入结果未知。清除会丢弃请求证据，不会取消或回滚服务端操作。请先保存请求标识，确认清除？", clearDirty: "清除会话将丢弃所有草稿与保留的内存证据，确认继续？", clearConfirm: "清除会话",
    switchSubmitting: "提交正在进行，暂不能切换租户。", switchDirty: "切换租户会丢弃当前租户的草稿与内存证据，不会取消或回滚已发出的操作。请先保存必要证据，确认切换？", switchConfirm: "切换租户",
    skipContent: "跳到页面内容", ssoLogout: "退出企业 SSO", switchLanguage: "English", storageUnavailable: "本机存储不可用，语言仅在本页生效，无法记住此次选择。", ssoSignIn: "使用企业 SSO 登录", ssoFailure: "SSO 登录不可用或回调无效，请重新开始登录。",
    expiredEvidence: "已验证的凭据已到期，已停止新请求。草稿与请求证据仍保留在本页内存中；已发出的操作不会被取消或回滚。请先保存必要证据，再清除会话并重新登录。", handoffBlocked: "当前操作进行中或结果未知，交接未执行。", deletionOwnsQueue: "此 Queue 保留了删除审阅或请求证据。请先返回删除页面取消审阅；已提交的操作须先保存证据并清除会话。", unavailablePage: "该页面尚未实现或 URL 无效；不会自动执行创建或修改。",
    errors: {"missing-token": "请输入恢复 Token。", "missing-credentials": "请输入用户名和密码。", "credentials-rejected": "用户名密码或恢复凭据被拒绝。", "role-denied": "该身份没有允许的管理角色。", "auth-disabled": "服务端尚未配置本地账户认证。", expired: "已验证的凭据已过期。", "invalid-response": "服务端返回了无效的身份响应。", unavailable: "身份验证不可用，请检查服务端及连接后手动重试。"},
  },
};
