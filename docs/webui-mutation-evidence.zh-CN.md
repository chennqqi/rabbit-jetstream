# 写入接收端单次尝试证据

[English](webui-mutation-evidence.md) | [简体中文](webui-mutation-evidence.zh-CN.md)

候选实现补齐了[接口契约](webui-api-contracts.zh-CN.md) C-05 的增量失败元数据。原 HTTP 状态、错误码和消息不变。这不是持久结果台账、幂等协议、回滚证明或重试许可。

PUT/DELETE 到达审计意图处理阶段后，失败响应可包含 `error.mutation`：

```json
{"schemaVersion":"rjs.mutation-evidence.v1","scope":"receiving-attempt","phase":"audit_intent","resourceEffects":"none","intentId":"0123456789abcdef0123456789abcdef"}
```

| 阶段 | 资源影响 | 含义 |
| --- | --- | --- |
| `audit_intent` | `none` | 调用写入后端前，审计能力缺失或意图持久化失败。 |
| `backend` | `possible` | 后端返回错误，失败结果审计写入成功。包含冲突，不承诺已回滚。 |
| `audit_outcome` | `possible` | 后端已被调用，但结果审计失败；不能确定部分影响或执行完成。 |

影响范围仅限本次处理器调用内的 Queue/Stream/Consumer 及声明变更，排除审计/锁元数据和此前传输重放。没有生成意图时省略 `intentId`；标识存在不能证明已持久化，因为发布确认可能丢失。结果事件的 `intentId` 应匹配意图事件的 `id`；请求 ID 仍是可重复使用的关联标识，不是幂等键。

鉴权、解析、前置条件及读取失败不虚构阶段证据。缺失表示没有阶段信息。客户端忽略未知版本、范围、阶段/影响组合及非法标识。证据仅供展示和下载，不改变写入状态机。请求发出后的错误即使报告 `none` 仍是结果未知；回读和审计检查也不解锁重试。下载仅保留允许字段，排除原始错误文本及凭据，保留配置/审计数据敏感性提示。

## 验证

后端测试覆盖 PUT 和 DELETE 的审计能力缺失、意图持久化失败、结果持久化失败、后端错误、冲突及后端与结果审计同时失败，核对实际后端调用次数及意图/结果关联。读取失败不包含写入证据。前端测试覆盖严格解码、下载字段白名单，以及两个写入路径收到 `none` 后均不得重试。

本机浏览器场景（先在本机构建候选制品）：

```powershell
$env:RJS_TEST_RESPONSE_FAULT='attempt-evidence'
npm.cmd --prefix admin-ui run test:live
```

测试使用隔离的本机服务及自建资源。在真实 PUT/DELETE 成功转发后，替换为模拟的 `audit_intent/none` 错误，核验实际变更已经发生，但界面保持未知/锁定，并检查下载证据。这是客户端重放安全场景，不证明真实成功处理器发生了意图审计失败；后端阶段映射另有独立测试。不设置此环境变量时仍默认使用原非法 JSON 场景。

原始请求的持久结果查询、幂等重试、发布资格验收和内嵌 UI 晋级仍属于独立的未完成工作。
