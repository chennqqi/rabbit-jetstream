export const initialConfig = Object.freeze({
  retention: "24h",
  ackWait: "30s",
  maxDeliver: 5,
});
export const replicas = Object.freeze([
  { name: "nats-1", role: "Leader", offline: false, current: true, lag: 0 },
  { name: "nats-2", role: "Follower", offline: false, current: true, lag: 0 },
  { name: "nats-3", role: "Follower", offline: true, lag: null },
]);
// Deliberately bounded prototype grammar, not a replacement for the Go API contract.
export function duration(value) {
  if (String(value).length > 100) return null;
  const match = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/.exec(String(value));
  if (!match) return null;
  const [whole, fraction = ""] = match[1].split(".");
  const scale = 10n ** BigInt(fraction.length);
  const result =
    BigInt(whole + fraction) *
    { ms: 1000000n, s: 1000000000n, m: 60000000000n, h: 3600000000000n }[
      match[2]
    ];
  if (result % scale !== 0n) return null;
  const nanos = result / scale;
  return nanos <= 9223372036854775807n ? nanos : null;
}
export function validateConfig(config) {
  const errors = {};
  for (const key of ["retention", "ackWait"])
    if (
      duration(config[key]) === null ||
      (key === "ackWait" && duration(config[key]) === 0n)
    )
      errors[key] = {
        zh: "请输入有效时长（ms、s、m、h）；ACK 等待必须大于零，保留时间可为 0s。",
        en: "Use a valid duration (ms, s, m, h); ACK wait must be positive, max age may be 0s.",
      };
  const count = String(config.maxDeliver);
  if (
    !/^\d+$/.test(count) ||
    !Number.isSafeInteger(Number(count)) ||
    Number(count) < 1
  )
    errors.maxDeliver = {
      zh: "请输入大于零的安全整数；更大的后端整数不在本原型编辑范围内。",
      en: "Use a positive safe integer; larger backend integers cannot be edited in this prototype.",
    };
  return errors;
}
export function configDiff(before, after) {
  return Object.keys(initialConfig)
    .filter((key) => {
      if (
        key !== "maxDeliver" &&
        duration(before[key]) !== null &&
        duration(after[key]) !== null
      )
        return duration(before[key]) !== duration(after[key]);
      if (
        key === "maxDeliver" &&
        /^\d+$/.test(String(before[key])) &&
        /^\d+$/.test(String(after[key]))
      )
        return BigInt(before[key]) !== BigInt(after[key]);
      return String(before[key]) !== String(after[key]);
    })
    .map((key) => ({ key, before: before[key], after: after[key] }));
}

export function replicaState(replica) {
  if (replica.offline === true) return "offline";
  if (replica.current === true) return "current";
  if (replica.current === false && replica.offline === false)
    return "catching-up";
  return "unknown";
}

export function consumerFixture(fixture = "standard", config = initialConfig) {
  const priorities =
    fixture === "priority7"
      ? Array.from({ length: 8 }, (_, i) => i)
      : fixture === "priority0"
        ? [0]
        : [null];
  return priorities.map((priority, i) => ({
    stream: "RJSQ_orders_events",
    name:
      priority === null
        ? "RJSQC_orders_events"
        : `RJSQC_orders_events_P${priority}`,
    priority,
    subjects: [
      priority === null ? "orders.events" : `rjs.q.orders_events.p.${priority}`,
    ],
    mode: "pull",
    managed: true,
    ackWait: config.ackWait,
    maxDeliver: config.maxDeliver,
    pending: i === 0 ? 8420 : 0,
    ackPending: i === 0 ? 240 : 0,
    redelivered: 0,
  }));
}

export function queryConsumers(
  items,
  { q = "", mode = "all", page = 1, limit = 5 } = {},
) {
  const term = q.trim().toLowerCase();
  const matches = items
    .filter(
      (item) =>
        (mode === "all" || item.mode === mode) &&
        [item.name, ...item.subjects].some((s) =>
          s.toLowerCase().includes(term),
        ),
    )
    .sort((a, b) => a.name.localeCompare(b.name));
  const pages = Math.max(1, Math.ceil(matches.length / limit));
  const selectedPage = Math.max(
    1,
    Math.min(pages, Number.isSafeInteger(Number(page)) ? Number(page) : 1),
  );
  return {
    items: matches.slice((selectedPage - 1) * limit, selectedPage * limit),
    total: matches.length,
    page: selectedPage,
    pages,
  };
}

export function parseRoute(hash) {
  const [screen, query = ""] = hash.replace(/^#/, "").split("?");
  const params = new URLSearchParams(query);
  return {
    screen: [
      "queues",
      "overview",
      "configuration",
      "routing",
      "consumers",
      "events",
    ].includes(screen)
      ? screen
      : "overview",
    q: params.get("q") || "",
    mode: ["pull", "push"].includes(params.get("mode"))
      ? params.get("mode")
      : "all",
    page: Math.max(
      1,
      Number.isSafeInteger(Number(params.get("page")))
        ? Number(params.get("page"))
        : 1,
    ),
    consumer: params.get("consumer") || "",
    fixture: ["priority0", "priority7"].includes(params.get("fixture"))
      ? params.get("fixture")
      : "standard",
  };
}

export function routeHash(route) {
  const params = new URLSearchParams();
  for (const key of ["q", "mode", "page", "consumer", "fixture"]) {
    if (
      route[key] &&
      !({ mode: "all", page: 1, fixture: "standard" }[key] === route[key])
    )
      params.set(key, route[key]);
  }
  return `#${route.screen}${params.size ? `?${params}` : ""}`;
}
