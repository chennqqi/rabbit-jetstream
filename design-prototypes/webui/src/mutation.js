import { configDiff, validateConfig } from "./model.js";

export const mutationScenarios = [
  ["success", "正常接受", "Accepted"],
  ["conflict", "并发冲突", "Concurrent conflict"],
  ["forbidden", "无写权限", "Write forbidden"],
  ["expired", "会话过期", "Session expired"],
  ["unknown", "响应丢失 · 结果未知", "Response lost · outcome unknown"],
  ["partial", "部分生效", "Partially applied"],
  [
    "audit-failed",
    "资源已变更 · 审计结果丢失",
    "Resource changed · audit outcome missing",
  ],
];

// Synthetic contract fixtures only. No requests, credentials or retry side effects.
export function simulateMutation({ scenario, base, draft, revision, id }) {
  if (!mutationScenarios.some(([key]) => key === scenario))
    throw new Error("Unknown mock scenario");
  if (
    Object.keys(validateConfig(draft)).length ||
    !configDiff(base, draft).length
  )
    throw new Error("A valid semantic change is required");
  const request = { ...draft, maxDeliver: Number(draft.maxDeliver) };
  const common = {
    id,
    scenario,
    base: { ...base },
    request,
    baseRevision: revision,
  };
  if (scenario === "conflict")
    return {
      ...common,
      status: "conflict",
      current: { ...base, ackWait: base.ackWait === "90s" ? "120s" : "90s" },
      currentRevision: revision + 1,
    };
  if (["forbidden", "expired"].includes(scenario))
    return { ...common, status: scenario };
  if (["unknown", "audit-failed"].includes(scenario))
    return { ...common, status: "unknown" };
  if (scenario === "partial") return { ...common, status: "partial" };
  return { ...common, status: "accepted" };
}

export function inspectMutation(operation, observed) {
  if (operation.status === "unknown")
    return {
      ...operation,
      status: "verified",
      current: operation.request,
      currentRevision: operation.baseRevision + 1,
      observed: operation.request,
      auditMissing: operation.scenario === "audit-failed",
    };
  if (operation.status === "partial")
    return {
      ...operation,
      status: "partial-observed",
      current: operation.base,
      currentRevision: operation.baseRevision,
      observed: { ...observed, retention: operation.request.retention },
    };
  throw new Error("Only unresolved outcomes can be inspected");
}

export function rebaseDraft(base, draft, current) {
  // Only after explicit confirmation; local changes win, untouched fields use current.
  const result = { ...current };
  for (const change of configDiff(base, draft))
    result[change.key] = draft[change.key];
  return result;
}

export const unresolvedMutation = (operation) =>
  ["unknown", "partial", "partial-observed"].includes(operation?.status);
