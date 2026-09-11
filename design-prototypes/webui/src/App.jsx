import { useEffect, useRef, useState } from "react";
import {
  IconHome,
  IconList,
  IconDatabase,
  IconServer,
  IconFileText,
  IconSettings,
  IconRefresh,
  IconPencil,
  IconInfoCircle,
  IconAlertCircleFilled,
  IconSearch,
  IconArrowRight,
  IconUserCircle,
  IconMenu2,
  IconX,
  IconCircleFilled,
} from "@tabler/icons-react";
import {
  initialConfig,
  validateConfig,
  configDiff,
  replicas,
  consumerFixture,
  replicaState,
  parseRoute,
  routeHash,
} from "./model.js";
import { Consumers } from "./Consumers.jsx";
import { MutationOutcome } from "./MutationOutcome.jsx";
import {
  mutationScenarios,
  simulateMutation,
  inspectMutation,
  rebaseDraft,
  unresolvedMutation,
} from "./mutation.js";

const tabs = [
  ["overview", "概况", "Overview"],
  ["configuration", "配置", "Configuration"],
  ["routing", "路由", "Routing"],
  ["consumers", "消费者", "Consumers"],
  ["events", "事件", "Events"],
];
const fields = [
  ["retention", "保留时间", "Retention"],
  ["ackWait", "ACK 等待", "ACK wait"],
  ["maxDeliver", "最大投递", "Max deliveries"],
];
const nav = [
  ["home", "总览", "Overview", IconHome],
  ["queues", "Queues", "Queues", IconList],
  ["streams", "Streams", "Streams", IconDatabase],
  ["nodes", "节点", "Nodes", IconServer],
  ["audit", "审计", "Audit", IconFileText],
  ["settings", "设置", "Settings", IconSettings],
];
const clock = () =>
  new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date());
function Dialog({ title, close, children, actions, busy = false }) {
  const ref = useRef(null);
  useEffect(() => {
    const el = ref.current,
      previous = document.activeElement;
    el.showModal();
    return () => {
      el.close();
      previous?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      aria-labelledby="dialog-title"
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
    >
      <div className="dialog-header">
        <h2 id="dialog-title">{title}</h2>
        <button
          className="icon-button"
          aria-label="关闭 / Close"
          disabled={busy}
          onClick={close}
        >
          <IconX aria-hidden="true" />
        </button>
      </div>
      <div className="dialog-body">{children}</div>
      <div className="dialog-actions">{actions}</div>
    </dialog>
  );
}
export function App() {
  const [lang, setLang] = useState("zh");
  const tr = (zh, en) => (lang === "zh" ? zh : en);
  const [route, setRoute] = useState(() => parseRoute(location.hash));
  const tab = route.screen === "queues" ? "overview" : route.screen;
  const page = route.screen === "queues" ? "list" : "detail";
  const [config, setConfig] = useState(initialConfig),
    [draft, setDraft] = useState(initialConfig),
    [revision, setRevision] = useState(12);
  const [observedConfig, setObservedConfig] = useState(initialConfig);
  const [editBase, setEditBase] = useState(initialConfig);
  const [editRevision, setEditRevision] = useState(12);
  const [scenario, setScenario] = useState("success");
  const [operation, setOperation] = useState(null);
  const writeTimer = useRef();
  const writeLock = useRef(false);
  const requestNumber = useRef(0);
  const [readState, setReadState] = useState("success");
  const consumers = consumerFixture(route.fixture, observedConfig);
  const primaryConsumer = consumers[0];
  const pending = configDiff(observedConfig, config).length > 0;
  const [observed, setObserved] = useState("2026-09-09 16:20:00"),
    [refreshing, setRefreshing] = useState(false),
    [notice, setNotice] = useState("");
  const [dialog, setDialog] = useState(null),
    [stage, setStage] = useState("edit"),
    [discard, setDiscard] = useState(false),
    [errors, setErrors] = useState({});
  const [events, setEvents] = useState([]),
    [mobileNav, setMobileNav] = useState(false);
  const timer = useRef(),
    tabRefs = useRef([]);
  const [now, setNow] = useState(Date.now());
  const [lastSuccess, setLastSuccess] = useState(
    Date.parse("2026-09-09T16:20:00+08:00"),
  );
  const [readFailed, setReadFailed] = useState(false);
  const stale = readFailed || now - lastSuccess > 30000;
  const dirty = configDiff(editBase, draft).length > 0;
  useEffect(() => () => clearTimeout(timer.current), []);
  useEffect(() => () => clearTimeout(writeTimer.current), []);
  useEffect(() => {
    const tick = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(tick);
  }, []);
  useEffect(() => {
    clearTimeout(timer.current);
    setRefreshing(false);
    if (readState !== "success") setReadFailed(true);
  }, [route.fixture, readState]);
  useEffect(() => {
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
    document.title = "orders_events · Rabbit-JetStream";
  }, [lang]);
  useEffect(() => {
    const change = () => {
      if (
        dialog === "edit" &&
        (dirty || stage === "submitting" || stage === "outcome")
      ) {
        history.replaceState(null, "", routeHash(route));
        return;
      }
      if (
        unresolvedMutation(operation) &&
        parseRoute(location.hash).fixture !== route.fixture
      ) {
        history.replaceState(null, "", routeHash(route));
        return;
      }
      setRoute(parseRoute(location.hash));
    };
    addEventListener("hashchange", change);
    return () => removeEventListener("hashchange", change);
  }, [dialog, dirty, stage, operation, route]);
  useEffect(() => {
    if (
      !(dialog === "edit" && dirty) &&
      !unresolvedMutation(operation) &&
      stage !== "submitting"
    )
      return;
    const guard = (e) => {
      e.preventDefault();
      e.returnValue = "";
    };
    addEventListener("beforeunload", guard);
    return () => removeEventListener("beforeunload", guard);
  }, [dialog, dirty, operation, stage]);
  const sample = tr(
    "这是交互原型，修改仅保存在当前页面内存，不影响真实队列。",
    "Interactive prototype. Changes exist only in page memory and never affect real queues.",
  );
  function choose(key) {
    navigate({ screen: key, consumer: "" });
  }
  function navigate(update) {
    if (update.fixture && unresolvedMutation(operation)) return;
    const next = { ...route, ...update };
    history.pushState(null, "", routeHash(next));
    setRoute(next);
  }
  function refresh() {
    if (refreshing || unresolvedMutation(operation)) return;
    setRefreshing(true);
    setNotice("");
    timer.current = setTimeout(() => {
      if (readState === "success") {
        setObserved(clock());
        setObservedConfig(config);
        setLastSuccess(Date.now());
        setNow(Date.now());
        setReadFailed(false);
      }
      setRefreshing(false);
      setNotice(readState === "success" ? "refresh" : "read-failed");
    }, 600);
  }
  function edit() {
    if (unresolvedMutation(operation)) {
      setStage("outcome");
      setDialog("edit");
      return;
    }
    setEditBase({ ...config });
    setEditRevision(revision);
    setDraft({ ...config });
    setOperation(null);
    setScenario("success");
    setErrors({});
    setStage("edit");
    setDiscard(false);
    setDialog("edit");
  }
  function close() {
    if (stage === "submitting") return;
    if (stage === "outcome" && unresolvedMutation(operation)) {
      setDialog(null);
      return;
    }
    if (dialog === "edit" && dirty) {
      setDiscard(true);
      return;
    }
    setDialog(null);
  }
  function review(e) {
    e.preventDefault();
    const found = validateConfig(draft);
    setErrors(found);
    if (!Object.keys(found).length) setStage("review");
  }
  function apply() {
    if (
      writeLock.current ||
      stage !== "review" ||
      !dirty ||
      Object.keys(validateConfig(draft)).length
    )
      return;
    writeLock.current = true;
    setStage("submitting");
    const result = simulateMutation({
      scenario,
      base: editBase,
      draft,
      revision: editRevision,
      id: `mock-op-${++requestNumber.current}`,
    });
    writeTimer.current = setTimeout(() => {
      writeLock.current = false;
      setOperation(result);
      setEvents((v) => [
        {
          time: clock(),
          revision:
            result.status === "accepted" ? editRevision + 1 : editRevision,
          id: result.id,
          outcome: result.status,
        },
        ...v,
      ]);
      if (result.status === "accepted") {
        setConfig(result.request);
        setRevision(editRevision + 1);
        setDialog(null);
        setStage("edit");
        setNotice("saved");
      } else {
        if (result.status === "conflict") {
          setConfig(result.current);
          setRevision(result.currentRevision);
        }
        setStage("outcome");
      }
    }, 600);
  }
  function inspectOutcome() {
    const checked = inspectMutation(operation, observedConfig);
    setOperation(checked);
    setConfig(checked.current);
    setRevision(checked.currentRevision);
    setObservedConfig(checked.observed);
    setObserved(clock());
    setLastSuccess(Date.now());
    setNow(Date.now());
    setReadFailed(false);
    setReadState("success");
    // Keep original intent/outcome entry; inspection is separate evidence, not a write retry.
    setEvents((v) => [
      {
        time: clock(),
        revision: checked.currentRevision,
        id: checked.id,
        outcome: checked.status,
        action: "resource.inspect",
        auditMissing: checked.auditMissing,
      },
      ...v,
    ]);
  }
  function recoverEdit(event) {
    // This button is replaced by the form's submit button during rendering.
    // Cancel the original click's default action so recovery cannot submit it.
    event.preventDefault();
    const current = operation.current || config;
    setDraft(
      operation.status === "conflict"
        ? rebaseDraft(operation.base, operation.request, current)
        : { ...operation.request },
    );
    setEditBase({ ...current });
    setEditRevision(operation.currentRevision ?? revision);
    setOperation(null);
    setScenario("success");
    setStage("edit");
    setErrors({});
  }
  function tabKey(e, index) {
    let next;
    if (e.key === "ArrowRight") next = (index + 1) % 5;
    else if (e.key === "ArrowLeft") next = (index + 4) % 5;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = 4;
    else return;
    e.preventDefault();
    choose(tabs[next][0]);
    tabRefs.current[next]?.focus();
  }
  function configList() {
    return (
      <dl className="config-list">
        <div>
          <dt>Stream</dt>
          <dd>
            <code>RJSQ_orders_events</code>
          </dd>
        </div>
        <div>
          <dt>{tr("存储", "Storage")}</dt>
          <dd>
            <code>file</code>
          </dd>
        </div>
        <div>
          <dt>{tr("副本", "Replicas")}</dt>
          <dd>3</dd>
        </div>
        {fields.map(([key, zh, en]) => (
          <div key={key}>
            <dt>{tr(zh, en)}</dt>
            <dd>
              <code>{config[key]}</code>
            </dd>
          </div>
        ))}
      </dl>
    );
  }
  return (
    <div className="app">
      <a
        className="skip-link"
        href="#main"
        onClick={(e) => {
          e.preventDefault();
          document.getElementById("main")?.focus();
        }}
      >
        {tr("跳到主要内容", "Skip to main content")}
      </a>
      <aside
        className={`sidebar ${mobileNav ? "is-open" : ""}`}
        aria-label={tr("主导航", "Navigation")}
      >
        <div className="side-brand">
          <strong>RJS</strong>
          <span>Rabbit-JetStream</span>
        </div>
        <nav>
          {nav.map(([key, zh, en, Icon]) => (
            <button
              key={key}
              className={key === "queues" ? "selected" : ""}
              aria-current={key === "queues" ? "page" : undefined}
              onClick={() => {
                setMobileNav(false);
                key === "queues" ? choose("queues") : setDialog("scope");
              }}
            >
              <Icon size={24} stroke={1.7} aria-hidden="true" />
              {tr(zh, en)}
            </button>
          ))}
        </nav>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <button
            className="mobile-menu icon-button"
            aria-label={tr("打开导航", "Open navigation")}
            aria-expanded={mobileNav}
            onClick={() => setMobileNav(!mobileNav)}
          >
            <IconMenu2 aria-hidden="true" />
          </button>
          <div className="header-brand">
            <strong>RJS</strong>
            <div>
              Rabbit-JetStream
              <small>
                {tr("嵌入式消息运维控制台", "Embedded messaging operations")}
              </small>
            </div>
          </div>
          <div className="cluster-context">
            <span>
              {tr("demo-cluster · 三节点 / R3", "demo-cluster · 3 nodes / R3")}
            </span>
            <span className="status offline">
              <IconCircleFilled size={12} aria-hidden="true" />
              {tr("降级", "Degraded")}
            </span>
          </div>
          <span className="preview-label">
            {tr("模拟数据 · 设计预览", "Mock data · Design preview")}
          </span>
          <time className="header-time">
            {observed} <span>(Asia/Shanghai)</span>
          </time>
          <label className="language">
            <span className="sr-only">
              {tr("界面语言", "Interface language")}
            </span>
            <select value={lang} onChange={(e) => setLang(e.target.value)}>
              <option value="zh">简体中文</option>
              <option value="en">English</option>
            </select>
          </label>
          <button
            className="access-button"
            aria-label={tr("原型访问说明", "Prototype access")}
            onClick={() => setDialog("access")}
          >
            <IconUserCircle size={28} aria-hidden="true" />
            <span>{tr("运维", "Operator")}</span>
          </button>
        </header>
        <main id="main" tabIndex={-1}>
          <p className="mobile-context">
            {tr(
              "模拟数据 · 设计预览 · 三节点 / R3",
              "Mock data · Design preview · 3 nodes / R3",
            )}
          </p>
          <div className="breadcrumbs">
            <button onClick={() => choose("queues")}>Queues</button>
            {page !== "list" && (
              <>
                <span>/</span>
                <span>orders_events</span>
              </>
            )}
          </div>
          {page === "list" ? (
            <section className="list-page">
              <h1>Queues</h1>
              <p className="muted">{sample}</p>
              <div className="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>{tr("队列名称", "Queue name")}</th>
                      <th>{tr("状态", "State")}</th>
                      <th>{tr("版本", "Revision")}</th>
                      <th>{tr("操作", "Action")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td>
                        <code>orders_events</code>
                      </td>
                      <td>{tr("降级", "Degraded")}</td>
                      <td>{revision}</td>
                      <td>
                        <button onClick={() => choose("overview")}>
                          {tr("查看详情", "View details")}
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          ) : (
            <>
              <div className="resource-heading">
                <div>
                  <div className="title-line">
                    <h1>orders_events</h1>
                    <span className="badge">{tr("降级", "Degraded")}</span>
                  </div>
                  <p className="revision">
                    {tr("模拟 KV 版本", "Mock KV revision")} {revision}
                    <span className="separator" />
                    {tr("最近观测", "Observed")} {observed}
                  </p>
                </div>
                <div className="heading-actions">
                  <button
                    disabled={refreshing || unresolvedMutation(operation)}
                    onClick={refresh}
                  >
                    <IconRefresh
                      size={21}
                      className={refreshing ? "spin" : ""}
                      aria-hidden="true"
                    />
                    {refreshing
                      ? tr("刷新中", "Refreshing")
                      : tr("刷新", "Refresh")}
                  </button>
                  <button
                    className="primary"
                    onClick={edit}
                    disabled={refreshing}
                  >
                    <IconPencil size={21} aria-hidden="true" />
                    {tr("编辑配置", "Edit configuration")}
                  </button>
                </div>
              </div>
              {unresolvedMutation(operation) && (
                <p role="alert" className="state-message">
                  {tr(
                    "有未解决的模拟写入结果；刷新不能代替核查，禁止直接重提。",
                    "Unresolved mock write outcome; refresh is not inspection. Do not resubmit.",
                  )}{" "}
                  <button
                    onClick={() => {
                      setStage("outcome");
                      setDialog("edit");
                    }}
                  >
                    {tr("查看未决操作", "Review unresolved operation")}
                  </button>
                </p>
              )}
              {pending && (
                <p role="status" className="state-message">
                  {tr(
                    "模拟声明与观测不一致；可能等待生效或存在漂移，需要核对资源。",
                    "Mock declaration and observation differ; convergence may be pending or resources may have drifted. Verify resources.",
                  )}
                </p>
              )}
              {readState !== "success" && (
                <p role="status" className="state-message">
                  {tr(
                    "Consumer 证据不完整；旧观测不可当作当前状态。",
                    "Consumer evidence incomplete; old observations are not current state.",
                  )}
                </p>
              )}
              <div className="warning">
                <IconAlertCircleFilled size={26} aria-hidden="true" />
                <span>
                  {tr("副本 nats-3 离线", "Replica nats-3 is offline")}
                  <span className="warning-separator"> · </span>
                  {tr("最近观测", "Observed")} {observed.slice(11)}
                </span>
              </div>
              <div className="tabs" role="tablist" aria-label="Queue details">
                {tabs.map(([key, zh, en], index) => (
                  <button
                    key={key}
                    id={`tab-${key}`}
                    role="tab"
                    aria-selected={key === tab}
                    aria-controls={`panel-${key}`}
                    tabIndex={key === tab ? 0 : -1}
                    ref={(el) => (tabRefs.current[index] = el)}
                    onClick={() => choose(key)}
                    onKeyDown={(e) => tabKey(e, index)}
                  >
                    {tr(zh, en)}
                  </button>
                ))}
              </div>
              <div
                role="tabpanel"
                id={`panel-${tab}`}
                aria-labelledby={`tab-${tab}`}
                tabIndex={0}
              >
                {tab === "overview" && (
                  <>
                    <div className="detail-grid">
                      <section className="evidence-panel surface">
                        <h2>{tr("关键指标", "Key metrics")}</h2>
                        <div className="metrics">
                          {[
                            [
                              tr("存储消息", "Stored messages"),
                              "12,480",
                              tr(
                                "关联 Stream 中存储的消息数",
                                "Messages stored in the associated Stream",
                              ),
                            ],
                            [
                              tr("待投递", "Pending"),
                              ["empty", "forbidden"].includes(readState)
                                ? "—"
                                : "8,420",
                              tr(
                                "指定 Consumer 的待投递数",
                                "Pending for the named Consumer",
                              ),
                            ],
                            [
                              tr("待确认", "Ack pending"),
                              ["empty", "forbidden"].includes(readState)
                                ? "—"
                                : "240",
                              tr(
                                "Consumer 待确认的消息数",
                                "Delivered, unacknowledged messages for the example Consumer",
                              ),
                            ],
                          ].map(([label, value, help]) => (
                            <div key={label}>
                              <span>{label}</span>
                              <strong>{value}</strong>
                              <p>{help}</p>
                            </div>
                          ))}
                        </div>
                        <p className="information metric-note">
                          <IconInfoCircle size={18} aria-hidden="true" />
                          {tr(
                            "统计口径不同，请勿相加。Consumer：",
                            "Different scopes; do not add. Consumer:",
                          )}
                          <button
                            className="text-button metric-consumer"
                            onClick={() =>
                              navigate({
                                screen: "consumers",
                                consumer: primaryConsumer.name,
                              })
                            }
                          >
                            {primaryConsumer.name}
                          </button>
                        </p>
                        <h2 className="replica-heading">
                          {tr("副本状态", "Replica state")}
                        </h2>
                        <div className="table-scroll">
                          <table className="replica-table">
                            <thead>
                              <tr>
                                <th>{tr("节点", "Node")}</th>
                                <th>{tr("角色", "Role")}</th>
                                <th>{tr("状态", "State")}</th>
                                <th>Lag</th>
                              </tr>
                            </thead>
                            <tbody>
                              {replicas.map((r) => (
                                <tr key={r.name}>
                                  <td>
                                    <code>{r.name}</code>
                                  </td>
                                  <td>{r.role}</td>
                                  <td>
                                    <span
                                      className={`status ${replicaState(r) === "current" ? "current" : "offline"}`}
                                    >
                                      <IconCircleFilled
                                        size={12}
                                        aria-hidden="true"
                                      />
                                      {
                                        {
                                          offline: tr("离线", "Offline"),
                                          current: tr("当前", "Current"),
                                          "catching-up": tr(
                                            "追赶中",
                                            "Catching up",
                                          ),
                                          unknown: tr("未知", "Unknown"),
                                        }[replicaState(r)]
                                      }
                                    </span>
                                  </td>
                                  <td>{r.lag ?? "—"}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                        <p className="information lag-note">
                          <IconInfoCircle size={18} aria-hidden="true" />
                          {tr(
                            "Lag 为副本复制进度差；离线副本的值未知，不代表零。",
                            "Lag is replication progress difference. Offline replica lag is unknown, not zero.",
                          )}
                        </p>
                      </section>
                      <section className="config-panel surface">
                        <h2>{tr("队列配置", "Queue configuration")}</h2>
                        {configList()}
                      </section>
                    </div>
                    <section className="diagnostic-strip">
                      <IconSearch size={28} stroke={1.8} aria-hidden="true" />
                      <div>
                        <h2>{tr("进一步排查", "Investigate further")}</h2>
                        <p>
                          {tr(
                            "查看该队列的消费者，了解投递与确认情况。",
                            "Inspect consumers to understand delivery and acknowledgement.",
                          )}
                        </p>
                      </div>
                      <button
                        className="text-button"
                        onClick={() => {
                          choose("consumers");
                          requestAnimationFrame(() =>
                            tabRefs.current[3]?.focus(),
                          );
                        }}
                      >
                        {tr("查看消费者", "View consumers")}
                        <IconArrowRight size={22} aria-hidden="true" />
                      </button>
                    </section>
                  </>
                )}
                {tab === "configuration" && (
                  <section className="surface tab-surface">
                    <h2>{tr("队列配置", "Queue configuration")}</h2>
                    <p className="muted">
                      {tr(
                        "资源名称、存储和副本在此原型中只读。",
                        "Name, storage and replicas are read-only in this prototype.",
                      )}
                    </p>
                    {configList()}
                    <p className="muted">
                      {tr(
                        "此处为期望声明，不是资源观测。0s 表示不设年龄上限；保留策略为 workqueue。模拟 KV 版本仅用于演示，真实 ETag 和内容版本必须分别读取。",
                        "Desired declaration, not observed resource state. 0s means no age limit; retention policy is workqueue. Mock KV revision is illustrative; real ETag and content revision must be read separately.",
                      )}
                    </p>
                    <button className="primary" onClick={edit}>
                      {tr("编辑配置", "Edit configuration")}
                    </button>
                  </section>
                )}
                {tab === "routing" && (
                  <section className="surface tab-surface">
                    <h2>{tr("Queue 路由计划", "Queue routing plan")}</h2>
                    <p className="muted">
                      {tr(
                        "仅展示声明映射；未创建独立 AMQP Exchange，也不会发布测试消息。",
                        "Declaration mapping only. No independent AMQP Exchange or test publishing.",
                      )}
                    </p>
                    <dl className="config-list">
                      {[
                        ["Queue", "orders_events"],
                        [
                          tr("声明 Subject", "Declaration subject"),
                          "orders.events",
                        ],
                        [
                          tr("生成 Subjects", "Generated subjects"),
                          consumers.flatMap((c) => c.subjects).join(", "),
                        ],
                        ["Stream", "RJSQ_orders_events"],
                      ].map(([key, value]) => (
                        <div key={key}>
                          <dt>{key}</dt>
                          <dd>
                            <code>{value}</code>
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </section>
                )}
                {tab === "consumers" && (
                  <Consumers
                    tr={tr}
                    route={route}
                    navigate={navigate}
                    consumers={consumers}
                    observed={observed}
                    pending={pending}
                    readState={readState}
                    setReadState={setReadState}
                    refreshing={refreshing}
                  />
                )}
                {tab === "events" && (
                  <section className="surface tab-surface">
                    <h2>{tr("管理审计事件", "Management audit events")}</h2>
                    <p className="muted">
                      {tr(
                        "本次页面会话的模拟管理事件，不是投递记录或持久审计。",
                        "Mock management events from this page session, not delivery history or durable audit.",
                      )}
                    </p>
                    {!events.length ? (
                      <p className="empty">
                        {tr(
                          "当前会话尚无配置变更。",
                          "No configuration changes in this session.",
                        )}
                      </p>
                    ) : (
                      <div className="table-scroll">
                        <table>
                          <thead>
                            <tr>
                              <th>{tr("时间", "Time")}</th>
                              <th>{tr("操作", "Action")}</th>
                              <th>{tr("版本", "Revision")}</th>
                              <th>{tr("结果", "Result")}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {events.map((event, index) => (
                              <tr key={index}>
                                <td>{event.time}</td>
                                <td>
                                  <code>{event.action || "queue.apply"}</code>
                                </td>
                                <td>{event.revision}</td>
                                <td>
                                  {event.outcome || "accepted"}
                                  {event.auditMissing
                                    ? tr(
                                        " · 审计结果缺失",
                                        " · audit outcome missing",
                                      )
                                    : ""}{" "}
                                  <code>{event.id}</code>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </section>
                )}
              </div>
              <p className="observation-note">
                {tr(
                  stale
                    ? "观测已陈旧（超过 30 秒或读取失败）；未知值不代表零。"
                    : "数据来自最近一次观测；未知值不代表零。",
                  stale
                    ? "Observation stale (over 30 seconds or read failed); unknown is not zero."
                    : "Last observed data; unknown does not mean zero.",
                )}
              </p>
            </>
          )}
        </main>
      </div>
      <div className={`notice ${notice ? "visible" : ""}`} role="status">
        {notice &&
          (notice === "saved"
            ? tr(
                "模拟配置已更新，未发送生产请求。",
                "Mock configuration updated; no production request was sent.",
              )
            : notice === "read-failed"
              ? tr(
                  "模拟读取未成功，保留上次观测时间和值。",
                  "Mock read did not succeed; previous observation time and values retained.",
                )
              : tr(
                  "模拟观测已刷新，副本离线状态保持不变。",
                  "Mock observation refreshed. The replica remains offline.",
                ))}
        {notice && (
          <button
            className="icon-button"
            aria-label={tr("关闭", "Close")}
            onClick={() => setNotice("")}
          >
            <IconX size={18} aria-hidden="true" />
          </button>
        )}
      </div>
      {dialog === "edit" && (
        <Dialog
          busy={stage === "submitting"}
          title={
            discard
              ? tr("放弃修改？", "Discard changes?")
              : stage === "outcome"
                ? tr("核查写入结果", "Inspect write outcome")
                : stage === "submitting"
                  ? tr("正在提交模拟修改", "Submitting mock change")
                  : stage === "review"
                    ? tr("复核配置变更", "Review configuration changes")
                    : tr("编辑配置", "Edit configuration")
          }
          close={close}
          actions={
            discard ? (
              <>
                <button onClick={() => setDiscard(false)}>
                  {tr("继续编辑", "Keep editing")}
                </button>
                <button
                  onClick={() => {
                    setDiscard(false);
                    setDialog(null);
                  }}
                >
                  {tr("放弃修改", "Discard changes")}
                </button>
              </>
            ) : stage === "submitting" ? (
              <button disabled>
                {tr("提交中，请勿重复提交", "Submitting; do not resubmit")}
              </button>
            ) : stage === "outcome" ? (
              <>
                {["unknown", "partial"].includes(operation.status) && (
                  <button className="primary" onClick={inspectOutcome}>
                    {tr(
                      "核查模拟资源与审计",
                      "Inspect mock resources and audit",
                    )}
                  </button>
                )}
                {operation.status === "conflict" && (
                  <button className="primary" onClick={recoverEdit}>
                    {tr(
                      "保留本地改动并重新编辑",
                      "Keep local changes and edit again",
                    )}
                  </button>
                )}
                {["forbidden", "expired"].includes(operation.status) && (
                  <button className="primary" onClick={recoverEdit}>
                    {tr(
                      "模拟恢复访问并重新编辑",
                      "Simulate access recovery and edit again",
                    )}
                  </button>
                )}
                {operation.status === "partial-observed" && (
                  <button className="primary" onClick={recoverEdit}>
                    {tr(
                      "按当前声明人工复核修复",
                      "Review repair against current declaration",
                    )}
                  </button>
                )}
                {operation.status === "verified" ? (
                  <button
                    onClick={() => {
                      setDraft(config);
                      setEditBase(config);
                      setDialog(null);
                      setStage("edit");
                    }}
                  >
                    {tr("完成核查", "Finish inspection")}
                  </button>
                ) : (
                  <button onClick={close}>
                    {tr("关闭结果", "Close outcome")}
                  </button>
                )}
              </>
            ) : stage === "review" ? (
              <>
                <button onClick={() => setStage("edit")}>
                  {tr("返回编辑", "Back to editor")}
                </button>
                <button className="primary" disabled={!dirty} onClick={apply}>
                  {tr("应用模拟修改", "Apply mock changes")}
                </button>
              </>
            ) : (
              <>
                <button onClick={close}>{tr("取消", "Cancel")}</button>
                <button className="primary" type="submit" form="config-form">
                  {tr("复核修改", "Review changes")}
                </button>
              </>
            )
          }
        >
          {discard ? (
            <p>
              {tr(
                "离开后将丢弃未保存草稿，是否继续？",
                "Unsaved changes will be discarded. Continue?",
              )}
            </p>
          ) : (
            <>
              <p className="prototype-note">
                <IconInfoCircle size={20} aria-hidden="true" />
                {sample}
              </p>
              <p className="muted">
                <code>orders_events</code> ·{" "}
                {tr("原模拟 KV 版本", "Base mock KV revision")} {editRevision}
              </p>
              <p className="muted">
                {tr(
                  "影响范围：当前 Queue 的 Stream / 托管 Consumers。提交后仍需刷新观测；本原型不模拟真实 ETag、权限或审计持久化。",
                  "Impact: this Queue's Stream / managed Consumers. Refresh after submission to observe; real ETags, authorization and durable audit are not simulated.",
                )}
              </p>
              {stage === "review" && (
                <ul className="impact-list">
                  {consumers.map((item) => (
                    <li key={item.name}>
                      <code>{item.name}</code>
                    </li>
                  ))}
                </ul>
              )}
              {stage === "outcome" ? (
                <MutationOutcome
                  operation={operation}
                  tr={tr}
                  fields={fields}
                />
              ) : stage === "edit" ? (
                <form id="config-form" onSubmit={review} noValidate>
                  <details className="fixture-controls">
                    <summary>
                      {tr("写入模拟场景", "Mock write scenarios")}
                    </summary>
                    <label>
                      {tr("下次提交场景", "Next submission scenario")}
                      <select
                        aria-label={tr(
                          "下次提交场景",
                          "Next submission scenario",
                        )}
                        value={scenario}
                        onChange={(e) => setScenario(e.target.value)}
                      >
                        {mutationScenarios.map(([key, zh, en]) => (
                          <option key={key} value={key}>
                            {tr(zh, en)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <p className="muted">
                      {tr(
                        "仅演示失败与恢复，不连接真实认证或服务。",
                        "Failure/recovery demonstration only; no real authentication or service.",
                      )}
                    </p>
                  </details>
                  {fields.map(([key, zh, en]) => (
                    <label className="form-field" key={key}>
                      {tr(zh, en)}
                      <input
                        name={key}
                        value={draft[key]}
                        aria-invalid={!!errors[key]}
                        aria-describedby={
                          errors[key] ? `error-${key}` : undefined
                        }
                        onChange={(e) =>
                          setDraft({ ...draft, [key]: e.target.value })
                        }
                      />
                      {errors[key] && (
                        <span className="field-error" id={`error-${key}`}>
                          {errors[key][lang]}
                        </span>
                      )}
                    </label>
                  ))}
                </form>
              ) : dirty ? (
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>{tr("字段", "Field")}</th>
                        <th>{tr("原值", "Before")}</th>
                        <th>{tr("新值", "After")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {configDiff(editBase, draft).map((change) => (
                        <tr key={change.key}>
                          <td>
                            {tr(
                              ...fields
                                .find((f) => f[0] === change.key)
                                .slice(1),
                            )}
                          </td>
                          <td>
                            <code>{change.before}</code>
                          </td>
                          <td>
                            <code>{change.after}</code>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p>{tr("没有配置变更。", "No configuration changes.")}</p>
              )}
            </>
          )}
        </Dialog>
      )}
      {(dialog === "scope" || dialog === "access") && (
        <Dialog
          title={
            dialog === "scope"
              ? tr("原型范围", "Prototype scope")
              : tr("原型访问说明", "Prototype access")
          }
          close={() => setDialog(null)}
          actions={
            <button onClick={() => setDialog(null)}>
              {tr("关闭", "Close")}
            </button>
          }
        >
          <p>
            {dialog === "scope"
              ? tr(
                  "当前仅实现 Queue 详情及编辑复核。此导航页面尚未实现，不会连接外部服务。",
                  "Only Queue details and edit/review are implemented. This page is not implemented and will not connect to external services.",
                )
              : tr(
                  "当前为模拟 operator 身份，不存在真实登录会话；请勿输入任何凭据。",
                  "The operator identity is simulated, not a real login session. Do not enter credentials.",
                )}
          </p>
        </Dialog>
      )}
    </div>
  );
}
