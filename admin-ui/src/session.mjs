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
  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    clear() {
      invalidate();
      emit({phase: "signed-out", identity: null, failure: null});
    },
    async signIn(token) {
      invalidate();
      const current = generation;
      if (typeof token !== "string" || !token.trim()) {
        emit({phase: "signed-out", identity: null, failure: {kind: "missing-token"}});
        return;
      }
      api.setToken(token);
      controller = new AbortController();
      emit({phase: "verifying", identity: null, failure: null});
      try {
        const {body} = await api.request("/api/v1/session", {signal: controller.signal});
        if (current !== generation) return;
        if (!body || typeof body.actor !== "string" || !body.actor ||
            !["operator", "auditor"].includes(body.role) ||
            !Array.isArray(body.permissions) || body.permissions.some(p => typeof p !== "string") ||
            !["authenticated", "anonymous"].includes(body.resource_read_policy) ||
            !(body.expires_at === null || (typeof body.expires_at === "string" && Number.isFinite(Date.parse(body.expires_at))))) {
          throw new Error("invalid-session");
        }
        if (body.expires_at !== null && Date.parse(body.expires_at) <= now()) {
          throw new Error("expired-session");
        }
        // Whitelist fields instead of retaining an arbitrary response payload.
        const identity = Object.freeze({
          actor: body.actor, role: body.role,
          permissions: Object.freeze([...body.permissions]),
          expires_at: body.expires_at, resource_read_policy: body.resource_read_policy,
        });
        emit({phase: "authenticated", identity, failure: null});
        if (identity.expires_at !== null) {
          const deadline = Date.parse(identity.expires_at);
          api.setTokenDeadline?.(deadline, () => expire(current));
          armExpiry(current, deadline);
        }
      } catch (error) {
        if (current !== generation) return;
        api.clearToken();
        const kind = error.status === 401 ? "credentials-rejected" :
          error.status === 403 ? "role-denied" :
          error.status === 404 && error.code === "session_api_disabled" ? "auth-disabled" :
          error.message === "expired-session" ? "expired" :
          error.message === "invalid-session" ? "invalid-response" : "unavailable";
        // Raw server error strings can contain sensitive data; do not render them.
        emit({phase: "signed-out", identity: null, failure: {kind, status: error.status}});
      }
    },
  };
}
