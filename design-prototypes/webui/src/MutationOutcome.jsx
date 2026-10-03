export function MutationOutcome({ operation, tr, fields }) {
  const messages = {
    conflict: [
      "版本冲突：本次修改未写入。原始草稿保留；对照当前值后明确选择，必须重新复核。",
      "Version conflict: this write was rejected. Draft retained; compare current values and explicitly choose before reviewing again.",
    ],
    forbidden: [
      "无写权限：未执行修改，草稿保留。模拟恢复权限后仍需重新复核。",
      "Write forbidden: no mutation; draft retained. Review again after simulated access recovery.",
    ],
    expired: [
      "会话已过期：未执行修改，草稿仅保存在当前页面内存。不要输入真实凭据。",
      "Session expired: no mutation; draft remains only in page memory. Do not enter real credentials.",
    ],
    unknown: [
      "结果未知：资源可能已经变更。禁止直接重提；先核查资源与审计。关闭窗口不会撤销请求。",
      "Outcome unknown: resources may have changed. Do not resubmit; inspect resources and audit first. Closing does not undo the request.",
    ],
    partial: [
      "部分生效：部分资源操作已完成，整体未成功。不得假设回滚；先核查实际资源。",
      "Partially applied: some resource operations completed, but the whole change did not succeed. Do not assume rollback; inspect resources first.",
    ],
    "partial-observed": [
      "已核查：Stream 年龄限制为请求值，Consumer 仍为旧观测，声明未推进。需要人工复核修复，不会自动重试。",
      "Inspected: Stream max age has the requested value, Consumers retain old observations, and declaration revision did not advance. Manual repair review required; no automatic retry.",
    ],
    verified: [
      "已核查模拟资源：声明及观测已确认；这不是重新提交。审计完整性需独立判断。",
      "Mock resources inspected: declaration and observations confirmed; no resubmission occurred. Audit completeness is a separate result.",
    ],
  };
  return (
    <section className="mutation-outcome">
      <p role="alert" className="state-message">
        {tr(...messages[operation.status])}
      </p>
      <p>
        {tr("模拟操作关联 ID", "Mock correlation ID")}:{" "}
        <code>{operation.id}</code>
      </p>
      {operation.auditMissing && (
        <p role="alert">
          {tr(
            "审计结果仍缺失：资源确认不能替代审计成功。",
            "Audit outcome still missing: resource confirmation is not audit success.",
          )}
        </p>
      )}
      {operation.status === "conflict" && (
        <div
          className="table-scroll"
          role="region"
          tabIndex={0}
          aria-label={tr(
            "冲突比较，可横向滚动",
            "Conflict comparison, horizontally scrollable",
          )}
        >
          <table>
            <thead>
              <tr>
                {[
                  tr("字段", "Field"),
                  tr("原值", "Base"),
                  tr("本地草稿", "Local draft"),
                  tr("当前值", "Current"),
                ].map((label) => (
                  <th key={label}>{label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {fields.map(([key, zh, en]) => (
                <tr key={key}>
                  <td>{tr(zh, en)}</td>
                  <td>
                    <code>{operation.base[key]}</code>
                  </td>
                  <td>
                    <code>{operation.request[key]}</code>
                  </td>
                  <td>
                    <code>{operation.current[key]}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p>
            {tr("原模拟 KV 版本", "Base mock KV revision")}:{" "}
            {operation.baseRevision} · {tr("当前", "Current")}:{" "}
            {operation.currentRevision}
          </p>
        </div>
      )}
      {operation.status === "partial-observed" && (
        <dl className="config-list">
          {fields.map(([key, zh, en]) => (
            <div key={key}>
              <dt>{tr(zh, en)}</dt>
              <dd>{operation.observed[key]}</dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  );
}
