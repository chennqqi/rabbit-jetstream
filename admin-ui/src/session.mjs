// Verified server identity only. No token is exposed in snapshots or storage.
export function createSession(api, {now = Date.now, schedule = setTimeout, unschedule = clearTimeout} = {}) {
  let generation = 0;
  let controller;
  let expiryTimer;
  let state = {phase: "signed-out", identity: null, failure: null};
  const listeners = new Set();
  const emit = (next) => {
    state = Object.freeze(next);
    for (const listener of listeners) listener();
  };
  const invalidate = () => { generation++; unschedule(expiryTimer); expiryTimer = undefined; controller?.abort(); api.clearToken(); };
  function expire(current) {
    if (current !== generation || state.phase !== "authenticated") return;
    unschedule(expiryTimer); expiryTimer = undefined;
    if (api.expireToken) api.expireToken(); else api.clearToken();
    // Retain verified identity and caller-owned drafts/evidence, but revoke UI access.
    emit({...state, phase: "expired", failure: {kind: "expired"}});
  }
  function armExpiry(current, deadline) {
    if (current !== generation || state.phase !== "authenticated") return;
    const remaining = deadline - now();
    if (remaining <= 0) {expire(current);return;}
    expiryTimer = schedule(() => armExpiry(current, deadline), Math.min(remaining, 60000));
    expiryTimer?.unref?.();
  }
  async function authenticate(current, token) {
    api.setToken(token);
    controller = new AbortController();
    const {body} = await api.request("/api/v1/session", {signal: controller.signal});
    if (current !== generation) return;
    if (!body || typeof body.actor !== "string" || !body.actor ||
        !["operator", "auditor"].includes(body.role) ||
        !Array.isArray(body.permissions) || body.permissions.some(p => typeof p !== "string") ||
        !(body.tenants === undefined || Array.isArray(body.tenants) && body.tenants.length > 0 && body.tenants.every(tenant => typeof tenant === "string" && /^[A-Za-z0-9._-]{1,128}$/.test(tenant)) && new Set(body.tenants).size === body.tenants.length) ||
        !(body.tenant_roles === undefined || body.tenant_roles && Object.getPrototypeOf(body.tenant_roles) === Object.prototype) ||
        !(body.tenant_permissions === undefined || body.tenant_permissions && Object.getPrototypeOf(body.tenant_permissions) === Object.prototype) ||
        !["authenticated", "anonymous"].includes(body.resource_read_policy) ||
        !(body.expires_at === null || (typeof body.expires_at === "string" && Number.isFinite(Date.parse(body.expires_at))))) {
      throw new Error("invalid-session");
    }
    if (body.expires_at !== null && Date.parse(body.expires_at) <= now()) throw new Error("expired-session");
    const tenants=Object.freeze([...(body.tenants ?? [])]);
    const activeTenant=tenants[0]??null;
    const tenantRoles=body.tenant_roles??Object.fromEntries(tenants.map(tenant=>[tenant,body.role])),tenantPermissions=body.tenant_permissions??Object.fromEntries(tenants.map(tenant=>[tenant,body.permissions]));
    if(tenants.some(tenant=>!["operator","auditor"].includes(tenantRoles[tenant])||!Array.isArray(tenantPermissions[tenant])||tenantPermissions[tenant].some(value=>typeof value!=="string"))||Object.keys(tenantRoles).some(key=>!tenants.includes(key))||Object.keys(tenantPermissions).some(key=>!tenants.includes(key)))throw new Error("invalid-session");
    if(activeTenant)api.setTenant?.(activeTenant);
    const identity = Object.freeze({
      actor: body.actor, role: body.role,
      permissions: Object.freeze([...body.permissions]),
      tenants, active_tenant: activeTenant,
      tenant_roles:Object.freeze({...tenantRoles}),tenant_permissions:Object.freeze(Object.fromEntries(Object.entries(tenantPermissions).map(([key,value])=>[key,Object.freeze([...value])]))),
      expires_at: body.expires_at, resource_read_policy: body.resource_read_policy,
    });
    emit({phase: "authenticated", identity, failure: null});
    if (identity.expires_at !== null) {
      const deadline = Date.parse(identity.expires_at);
      api.setTokenDeadline?.(deadline, () => expire(current));
      armExpiry(current, deadline);
    }
  }
  function reject(current, error) {
    if (current !== generation) return;
    api.clearToken();
    // Expiry is only ever derived from the server's explicit `token_expired`
    // signal; a generic 401 must not be labeled as expiry.
    const kind = error.code === "token_expired" ? "expired" :
      error.status === 401 ? "credentials-rejected" :
      error.status === 403 ? "role-denied" :
      error.status === 404 && ["session_api_disabled", "local_auth_disabled"].includes(error.code) ? "auth-disabled" :
      error.message === "expired-session" ? "expired" :
      error.message === "invalid-session" || error.message === "invalid-login-response" ? "invalid-response" : "unavailable";
    emit({phase: "signed-out", identity: null, failure: {kind, status: error.status}});
  }
  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    clear() {
      invalidate();
      emit({phase: "signed-out", identity: null, failure: null});
    },
    selectTenant(tenant) {
      if(state.phase!=="authenticated"||!state.identity?.tenants.includes(tenant))throw new TypeError("Tenant is not available to this session");
      if(state.identity.active_tenant===tenant)return;
      api.setTenant?.(tenant);
      emit({...state,identity:Object.freeze({...state.identity,active_tenant:tenant,role:state.identity.tenant_roles[tenant]??state.identity.role,permissions:state.identity.tenant_permissions[tenant]??state.identity.permissions})});
    },
    async signIn(token) {
      invalidate();
      const current = generation;
      if (typeof token !== "string" || !token.trim()) {
        emit({phase: "signed-out", identity: null, failure: {kind: "missing-token"}});
        return;
      }
      emit({phase: "verifying", identity: null, failure: null});
      try {
        await authenticate(current, token.trim());
      } catch (error) {
        reject(current, error);
      }
    },
    async signInWithPassword(username, password) {
      invalidate();
      const current = generation;
      if (typeof username !== "string" || !username.trim() || typeof password !== "string" || !password) {
        emit({phase: "signed-out", identity: null, failure: {kind: "missing-credentials"}});
        return;
      }
      controller = new AbortController();
      emit({phase: "verifying", identity: null, failure: null});
      try {
        const {body} = await api.request("/api/v1/auth/login", {method: "POST", body: {username: username.trim(), password}, signal: controller.signal});
        if (current !== generation) return;
        if (!body || typeof body.access_token !== "string" || !body.access_token || body.token_type !== "Bearer" || typeof body.expires_at !== "string" || !Number.isFinite(Date.parse(body.expires_at)) || !Array.isArray(body.tenants) || body.tenants.length === 0) throw new Error("invalid-login-response");
        await authenticate(current, body.access_token);
      } catch (error) {
        reject(current, error);
      }
    },
  };
}
