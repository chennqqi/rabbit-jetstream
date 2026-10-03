import { useEffect, useRef } from "react";
import { queryConsumers } from "./model.js";

export function Consumers({
  tr,
  route,
  navigate,
  consumers,
  observed,
  pending,
  readState,
  setReadState,
  refreshing,
}) {
  const heading = useRef(null);
  const previousSelection = useRef(route.consumer);
  const result = queryConsumers(readState === "empty" ? [] : consumers, route);
  const selected = consumers.find((item) => item.name === route.consumer);
  useEffect(() => {
    if (route.consumer || previousSelection.current) heading.current?.focus();
    previousSelection.current = route.consumer;
  }, [route.consumer]);
  const updateQuery = (update) =>
    navigate({ ...update, consumer: "", page: 1 });
  return (
    <section className="surface tab-surface consumer-panel">
      <h2 ref={heading} tabIndex={-1}>
        {route.consumer
          ? tr("Consumer 详情", "Consumer details")
          : tr("Consumer 列表", "Consumer list")}
      </h2>
      <p className="muted">
        {tr(
          "当前 Queue · 一个 Stream · 托管 pull Consumers；持久 Consumer 不代表客户端在线。",
          "Current Queue · one Stream · managed pull Consumers; durable does not mean a connected client.",
        )}
      </p>
      <details className="fixture-controls">
        <summary>
          {tr("模拟场景（不连接真实服务）", "Mock scenarios (no real service)")}
        </summary>
        <label>
          {tr("Queue 样例", "Queue fixture")}
          <select
            aria-label={tr("Queue 样例", "Queue fixture")}
            value={route.fixture}
            onChange={(e) =>
              updateQuery({ fixture: e.target.value, q: "", mode: "all" })
            }
          >
            <option value="standard">
              {tr("普通队列 · 1 Consumer", "Standard · 1 Consumer")}
            </option>
            <option value="priority0">
              {tr("优先级 0 · 1 Consumer", "Priority 0 · 1 Consumer")}
            </option>
            <option value="priority7">
              {tr("优先级 0–7 · 8 Consumers", "Priority 0–7 · 8 Consumers")}
            </option>
          </select>
        </label>
        <label>
          {tr("读取场景", "Read scenario")}
          <select
            aria-label={tr("读取场景", "Read scenario")}
            value={readState}
            onChange={(e) => setReadState(e.target.value)}
          >
            <option value="success">{tr("成功", "Success")}</option>
            <option value="failed">
              {tr("刷新失败 · 保留旧值", "Refresh failed · retain old values")}
            </option>
            <option value="forbidden">{tr("无权限", "Forbidden")}</option>
            <option value="empty">
              {tr("预期资源缺失", "Expected resources missing")}
            </option>
          </select>
        </label>
      </details>
      <p className="muted">
        {tr("模拟最近观测", "Mock last observation")}: {observed}{" "}
        (Asia/Shanghai)
      </p>
      {pending && (
        <p role="status" className="state-message">
          {tr(
            "声明与观测不一致，等待新观测核对。下方保留上次观测值；可能等待生效，也可能存在漂移。",
            "Declaration and observation differ. Previous observations remain below; convergence may be pending or resources may have drifted. Refresh to verify.",
          )}
        </p>
      )}
      {readState === "failed" && (
        <p role="alert" className="state-message">
          {tr(
            "模拟读取失败：以下保留最后成功数据，已陈旧；不代表当前状态。选择成功场景后刷新重试。",
            "Mock read failed: retained last-success data is stale, not current. Select success and refresh to retry.",
          )}
        </p>
      )}
      {readState === "forbidden" ? (
        <p role="alert">
          {tr(
            "无权读取 Consumers，不是没有 Consumer。",
            "Consumer access forbidden; this is not an empty collection.",
          )}
        </p>
      ) : route.consumer ? (
        <>
          <button onClick={() => navigate({ consumer: "" })}>
            {tr("返回 Consumer 列表", "Back to Consumer list")}
          </button>
          {!selected || readState === "empty" ? (
            <p role="alert">
              {tr(
                "此 Consumer 未找到，可能已被删除；请返回列表刷新。",
                "Consumer not found; it may have been deleted. Return to the list and refresh.",
              )}
            </p>
          ) : (
            <dl className="config-list consumer-detail">
              {[
                ["Stream", selected.stream],
                ["Consumer", selected.name],
                [
                  tr("归属", "Ownership"),
                  tr("当前 Queue 托管", "Managed by this Queue"),
                ],
                [
                  tr("优先级", "Priority"),
                  selected.priority ?? tr("未启用", "Not enabled"),
                ],
                ["Filter subjects", selected.subjects.join(", ")],
                ["Mode", selected.mode],
                ["ACK policy", "explicit"],
                [tr("观测 ACK 等待", "Observed ACK wait"), selected.ackWait],
                [
                  tr("观测最大投递", "Observed max deliveries"),
                  selected.maxDeliver,
                ],
                [
                  tr("待投递", "Pending"),
                  selected.pending.toLocaleString("en-US"),
                ],
                [tr("待确认", "Ack pending"), selected.ackPending],
                [
                  tr("重投观测值", "Redelivered observation"),
                  selected.redelivered,
                ],
              ].map(([label, value]) => (
                <div key={label}>
                  <dt>{label}</dt>
                  <dd>
                    <code>{value}</code>
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </>
      ) : (
        <>
          <div className="consumer-toolbar">
            <label>
              {tr("名称或过滤 Subject", "Name or filter subject")}
              <input
                type="search"
                value={route.q}
                onChange={(e) => updateQuery({ q: e.target.value })}
              />
            </label>
            <label>
              {tr("投递模式", "Delivery mode")}
              <select
                value={route.mode}
                aria-label={tr("投递模式", "Delivery mode")}
                onChange={(e) => updateQuery({ mode: e.target.value })}
              >
                <option value="all">{tr("全部", "All")}</option>
                <option value="pull">pull</option>
                <option value="push">push</option>
              </select>
            </label>
          </div>
          <p className="muted">
            {tr(
              "模拟数据先筛选再分页；生产服务端筛选契约尚待实现。",
              "Mock data is filtered before paging; production server filtering is not implemented.",
            )}
          </p>
          <p role="status">
            {tr("匹配", "Matches")}: {result.total} ·{" "}
            {tr("预期托管", "Expected managed")}: {consumers.length}
          </p>
          {readState === "empty" ? (
            <p className="empty">
              {tr(
                "没有观测到 Consumer；预期托管资源缺失，不代表队列配置为零个 Consumer。",
                "No Consumers observed; expected managed resources are missing, not configured as zero.",
              )}
            </p>
          ) : result.total === 0 ? (
            <p className="empty">
              {tr("没有匹配的 Consumer。", "No matching Consumer.")}{" "}
              <button onClick={() => updateQuery({ q: "", mode: "all" })}>
                {tr("清除筛选", "Clear filters")}
              </button>
            </p>
          ) : (
            <div
              className="table-scroll"
              tabIndex={0}
              role="region"
              aria-label={tr(
                "Consumer 结果表，可横向滚动",
                "Consumer results, horizontally scrollable",
              )}
              aria-busy={refreshing}
            >
              <table className="consumer-table">
                <thead>
                  <tr>
                    {[
                      "Consumer",
                      "Filter subjects",
                      "Mode",
                      tr("待投递", "Pending"),
                      tr("待确认", "Ack pending"),
                      tr("ACK 等待", "ACK wait"),
                      tr("最大投递", "Max deliveries"),
                    ].map((label) => (
                      <th key={label}>{label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.items.map((item) => (
                    <tr key={`${item.stream}/${item.name}`}>
                      <td>
                        <button
                          className="text-button"
                          onClick={() => navigate({ consumer: item.name })}
                        >
                          {item.name}
                        </button>
                      </td>
                      <td>
                        <code>{item.subjects.join(", ")}</code>
                      </td>
                      <td>{item.mode}</td>
                      <td>{item.pending.toLocaleString("en-US")}</td>
                      <td>{item.ackPending}</td>
                      <td>{item.ackWait}</td>
                      <td>{item.maxDeliver}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <nav
            className="consumer-pagination"
            aria-label={tr("Consumer 分页", "Consumer pagination")}
          >
            <button
              disabled={result.page <= 1}
              onClick={() => navigate({ page: result.page - 1 })}
            >
              {tr("上一页", "Previous page")}
            </button>
            <span>
              {result.page} / {result.pages}
            </span>
            <button
              disabled={result.page >= result.pages}
              onClick={() => navigate({ page: result.page + 1 })}
            >
              {tr("下一页", "Next page")}
            </button>
          </nav>
        </>
      )}
    </section>
  );
}
