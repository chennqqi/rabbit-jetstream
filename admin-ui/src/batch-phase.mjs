const labels = new Map([
  ["not-prepared", ["Not prepared", "未准备"]],
  ["idle", ["Not started", "未开始"]],
  ["loading", ["Loading declaration", "读取声明中"]],
  ["load-error", ["Declaration read failed", "声明读取失败"]],
  ["uneditable", ["Declaration cannot be edited", "声明不可编辑"]],
  ["editing", ["Editing", "编辑中"]],
  ["review", ["Awaiting separate confirmation", "待独立确认"]],
  ["accepted", ["Apply accepted; not health proof", "提交已接受，非健康证明"]],
  ["uncertain", ["Write outcome unknown", "写入结果未知"]],
  ["submitting", ["Submitting once", "单次提交中"]],
  ["previewing", ["Previewing", "预览中"]],
  ["blocked", ["Preview blocked", "预览被阻塞"]],
  ["conflict", ["Conflict", "冲突"]],
  ["denied", ["Denied", "拒绝"]],
  ["preview-error", ["Preview failed", "预览失败"]],
  ["archived", ["Archived, read-only", "已归档，只读"]],
  ["inspecting", ["Inspecting unknown outcome", "检查未知结果中"]],
  ["reading-conflict", ["Reading conflict evidence", "读取冲突证据中"]],
  ["reading-next", ["Reading next edit base", "读取下一次编辑基线中"]],
]);

// Presentation only: never replace the machine phase in retained evidence.
export function batchPhaseLabel(phase, language) {
  const label = labels.get(phase)?.[language === "zh" ? 1 : 0];
  if (label) return label;
  const unknown = language === "zh" ? "未知状态" : "Unknown state";
  return typeof phase === "string" && phase ? `${unknown} (${phase})` : unknown;
}
