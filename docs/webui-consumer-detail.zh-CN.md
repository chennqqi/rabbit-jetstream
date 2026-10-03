# 精确 Consumer 详情实现

[English](webui-consumer-detail.md) | [简体中文](webui-consumer-detail.zh-CN.md)

## 当前候选行为

候选界面的 Queue/Stream Consumer 集合及精确 Consumer 详情已接入真实认证 API。资源读取默认要求服务端授权，显式本机演示例外见[访问策略](webui-access.zh-CN.md)。下方旧说明记录实施里程碑，不代表当前访问策略或发布完成结论。最新浏览器证据见[执行记录](webui-development.zh-CN.md)；内嵌发布资源与候选仍保持独立。

独立 Consumer 详情现共享手动/10/30/60 秒刷新偏好。同一时刻仅一个精确 Stream/name GET，前次读取完成后再计时，失败退避最长 60 秒，隐藏时暂停。路由/身份变化停止旧定时器、取消请求并隔离迟到结果。网络/不可用故障保留此前精确观测和原读取时间，标记历史/旧数据。Consumer 缺失、访问拒绝、明确禁用读取或响应身份无效/不匹配时，清除观测与时间。恢复后用新数据替换，包括模式/计数变化。不把同名重建的 Consumer 推断为同一生命周期。

30 秒为新鲜度策略，不是健康判定。保留旧值时，原始观测同样属于历史数据。不获取消息、不 ACK、不写入、不枚举、不预览。Queue 摘要现以一个调度批次读取声明指定的精确主 Consumer 和 Stream，独立标记历史数据及时间，不推进声明或编辑器 ETag，见[摘要刷新](webui-refresh.zh-CN.md#queue-摘要刷新)。Queue/Stream Consumer 集合有独立查询范围刷新及自身观测时间，见[刷新契约](webui-refresh.zh-CN.md#consumer-集合刷新)。

## 历史实施里程碑

日期：2026-09-09。状态：源码已实现，尚未发布。[C-06 / API-11](webui-api-contracts.zh-CN.md) 的部分交付。

`GET /api/v1/streams/{stream}/consumers/{consumer}` 返回现有 Consumer 观测对象，不是分页信封。例如：

```sh
curl --fail http://127.0.0.1:8080/api/v1/streams/RJSQ_orders_events/consumers/RJSQC_orders_events
```

请使用实际管理服务地址和端口。运行中的冻结候选版尚不包含此路由。返回字段包括 `stream`、`name`、`mode`、`pending`、`ack_pending`、`filter_subjects` 及可选 `cluster`；完整 Schema 见 [OpenAPI](../api/openapi.yaml)。

- 按 Stream/名称精确定位，不枚举列表，不依赖已加载分页。分页查询参数不影响此详情路由。
- 请求上下文三秒超时。一次 pull 查询，仅 SDK 返回 `ErrNotPullConsumer` 时追加一次 push 查询。不拉取消息、不订阅、不确认消息、不修改资源、不写审计。
- pull/push 与列表复用相同映射。回退期间模式并发变化可能返回 503，不无限重试。
- 200 返回观测；Stream/Consumer 缺失返回 404 `not_found`；其他后端失败返回 503 `jetstream_unavailable`。失败不能变成空对象或零值观测。响应使用 `Cache-Control: no-store`。
- 计数保持 JSON 整数，客户端必须在 JavaScript 舍入前无损解码大整数。不代表 Queue 全局汇总、新鲜度时间戳或健康结论。
- 访问策略沿用现有未认证资源读取。**这不代表批准非回环开放**；D-05 策略仍待决。未修改安全配置、部署、凭据或冻结制品。

本地执行的验证：

```sh
go test -count=1 ./...
go vet ./...
```

均通过。新增后端用例覆盖 pull/push、资源缺失、取消/超时、回退期间消失或类型变化；测试 SDK 遇到枚举或修改会立即失败。HTTP 用例验证身份范围、分页独立性、错误信封、超时边界、no-store，以及无修改/审计调用。Schema 字段检查发现模型/属性与必填/可选漂移，但不是完整 JSON Schema 验证器。现有路由/引用测试也通过。

本轮未执行真实 NATS 资格验收或浏览器接入验证。原型仍使用夹具。Queue 关联集合随后已按下文实现；全局列表查询和真实 API 的 UI 接入仍待完成，WP-02 尚未完成。

## Queue 关联集合（v0.3 源码更新）

`GET /api/v1/queues/{queue}/consumers` 现已将声明中的主 Consumer、额外优先级 Consumer 与声明 Stream 上的实际 Consumer 合并，包含额外外部资源。不枚举全账号，不沿 DLQ 依赖查询。示例（针对本地重新构建的管理服务）：

```sh
curl --fail 'http://127.0.0.1:8080/api/v1/queues/orders_events/consumers?q=orders&mode=pull&order=asc&offset=0&limit=50'
```

分页包含 `queue`、`stream`、`declaration_revision`、`stream_status`、`stream_ownership`、`items`、`total`、`offset`、`limit`。每行包含精确 `stream`/`name`、可空 `expected`（ConsumerPlan）、可空 `observed`（Consumer）、`status`（`present|missing`）和 `ownership`。缺失预期 Consumer 保留声明配置，但**观测为 null，不伪造零值指标**。额外 Consumer 的预期配置为 null。显式优先级零仍保留优先级 Consumer 身份；优先级 0–7 产生八个预期身份，各出现一次。

归属值 `matching|different|unmarked|unknown` 表示 `rabbit-jetstream.io/queue` 元数据证据。缺失观测为 unknown。Stream 与 Consumer 证据分别检查：名称匹配或 Consumer 元数据匹配都不能覆盖 Stream 归属不匹配。归属不等于授权、配置收敛或健康。

查询规则：

- `q`：去首尾空白、不区分大小写的名称或有效筛选 Subject 字面子串；规范化后最多 256 UTF-8 字节。优先使用观测多 Subject，回退到观测单 Subject；只有缺失观测时使用预期 Subjects。
- `mode`：省略/空值表示全部，否则为 `pull|push`；存在观测时使用观测模式，否则使用预期模式。
- 名称排序：`order=asc|desc`，默认升序。暂不支持其他排序字段。先筛选、排序，再分页；`total` 是完整筛选结果总数，不是当前页条数。
- 默认 `offset=0`、`limit=50`；limit 为 1–200。offset 超过总数时截到总数。空 items 为 `[]`。不支持、重复、编码错误或非法参数在后端读取前返回 400。现有其他列表路由不变。

读取安全及错误：

- 请求上下文五秒超时；枚举一个 Stream，最多 2,000 条实际记录（下一条用于检测超限），另加最多 256 个预期身份。所有退出路径取消上下文。成本为有界枚举，**不是索引，也不代表生产负载验收通过**。
- Queue 声明不存在返回 404。Stream 不存在返回 200，并标记 Stream 及预期 Consumer 缺失。其他读取失败、不完整/重复/身份不一致枚举、损坏声明或超限返回 503，不返回部分成功列表。
- 观测后重新读取声明 KV 版本；声明变化/删除返回 409。带引号的 `declaration_revision` 是不透明声明版本，不是 Consumer 指标的 HTTP ETag。不承诺原子 broker 快照，读取之间仍可能发生资源变化。
- 响应仍为 no-store，沿用现有未认证资源读取策略。不引入消息投递、资源修改、审计写入、安全策略调整、部署或远程操作。

新增测试覆盖优先级成员、缺失/外部资源、归属不匹配、零优先级、损坏声明、读取失败/取消、版本变化、数量上限边界、201 条资源检索、观测/预期 Subject 筛选、排序、分页和非法查询。ConsumerPlan 字段与 OpenAPI 必填/属性声明进行对照检查。本地 `go test -count=1 ./...` 和 `go vet ./...` 通过。真实 NATS/负载验收与实际 WebUI 接入仍待完成；未替换冻结候选版或夹具原型。
