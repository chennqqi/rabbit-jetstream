# Queue 只读变更预览

[English](webui-preview.md) | [简体中文](webui-preview.zh-CN.md)

已观测到的循环现返回 HTTP 400 和 `dlq_dependency_cycle`，区别于 broker 不可用及来源修订冲突。其他依赖失败保留原有分类；超过遍历上限不是存在循环的证明。预览失败保留可编辑草稿，不允许提交或冲突合并。已发送 PUT 的拒绝响应仍使浏览器保留未知状态：此错误码不是请求结果证明，也不授权自动重试。

## DLQ 依赖 Stream 校验

源码实现现还会遍历最多 100 个已保存目标声明，拒绝返回拟提交来源项、重复目标、缺失／不一致／无法还原声明及更长链路。遍历、直接目标 Stream 读取和全部观测声明修订复查共用五秒期限（或更短的调用方期限），不读取第 101 个目标。这取代下方此前仅直接循环的限制。不据此推断下游 Stream／Consumer 健康，仅比较直接目标 Stream。修订复查可发现观测到的变化，但不能使并发跨 Queue 更新具备原子性。

预览和提交现在不再仅要求已保存目标声明。目标的 Queue／修订／Stream 身份须一致，且能忠实重建规范文档。实时目标 Stream 必须存在，其归属、路由 Subject、配置及 RJS 元数据须通过与普通变更相同的协调比较，与已保存计划一致；允许无关元数据。读取 Stream 后再次检查目标声明 KV 修订，观测期间变化则失败。正式提交在预览后、修改 DLQ 基础设施／来源资源前再次检查。

该检查不创建、接管或修复目标。目标缺失、归属不同、发生漂移或无法还原时，需要负责人另行调查／协调；此前仅凭声明存在可通过的目标现可能被拒绝。这些是在调用方上下文内的有界读取，不是跨 Queue 事务，也不能防止观测之后的外部修改。该检查不验收目标 Consumer、副本健康或实际消息投递。既有双 Queue 循环与优先级兼容检查保留；不代表已对现存声明完成任意传递循环检测。

## 诊断字段定位

已识别的服务端字段诊断新增支持键盘的“定位字段或 JSON”按钮。精确标量字段、Subject 索引、Binding exchange/type 及路由键索引仅映射到当前编辑器内已知控件；定位前展开所在折叠区。组合约束、未知路径及不存在/禁用的控件回退到原始 JSON 文本框，不猜测子字段、不插入选择器、不修改草稿，也不请求 API。编辑后旧预览诊断及定位按钮立即清除。此定位仅用于符合条件的当前只读预览错误，不用于结果未知的写操作。

## 结构化 Queue 校验诊断

Queue 语义校验新增 `issues_version: rjs.queue-validation.v1` 和 `issues`，每项包含 JSON Pointer `path`、稳定规则 `code` 及可读 `message`。保留原有 `invalid_queue` 和汇总文字。预览及 PUT 在后端/审计操作前拒绝非法文档。语法/类型解析错误不猜测路径。先填充标准默认值再校验，仅在校验成功后排序，因此数组路径指向用户提交顺序。组合约束可指向 `/spec` 或 `/spec/retention`；必填值缺失时路径可指向尚不存在的字段。

编辑器仅在当前只读预览返回 400 时展示已识别且长度受限的诊断数组。未知/非法版本回退到原有受限文本；不解析错误散文、不注入 HTML、不自动修改字段，也不据此推断写操作结果。此项是结构化诊断，不是完整 Schema 驱动表单。

已有 Stream/Consumer 必须携带与生成计划相同且非空的 `rabbit-jetstream.io/queue` 标记。缺失或不同标记会以归属原因阻止协调，不提供自动接管。预览返回 blocked 结果且不写入；提交重新观测后返回 409，不修改资源或持久化声明（仍可能产生正常审计及队列锁活动）。以前未标记的资源现需操作者明确调查，不能自动覆盖。匹配的元数据只是观测证据，不替代 Broker 访问控制，也不是防止外部写者在读取后改变归属的原子保护。

资源元数据协调将 `rabbit-jetstream.io/` 键与生成计划同步；其他已观测键会保留，除非计划明确提供该键。预览与提交共用此规则。元数据变化值采用带引号的字符串表示，包括显式空值 `""`；缺失键使用不带引号的标记 `(absent)`。这修复了之前漏报的标签移除/空值漂移，变化字符串的消费者需注意表示方式调整。外部元数据保留基于当前观测，不是与外部并发写入的原子合并。

新增响应字段 `declaration_review` 将声明变化与观测资源协调分开。`status=available` 包含针对原始 `base_revision` 的 `diff` 及规范化目标 `document`；`status=create` 有目标文档，但不伪造旧声明或差异。`status=unavailable` 返回 `target_unrepresentable`、`base_unrepresentable` 或 `base_identity_mismatch`，不猜测无损转换；目标无法重建时不提供文档。旧服务端可能省略整个字段。数值配置保持精确，时长/单位写法及默认值可能归一化，不替换已提交草稿。声明差异为空不代表观测资源无需修复。读取比较基线时及返回前均检查原始 KV 版本，但不锁定声明或保留未来提交资格。最新真实服务验证见[执行记录](webui-development.zh-CN.md)，下方原始验证说明描述初次实现。

状态：源码已实现，尚未发布；属于 C-05 的一部分，不代表 WebUI 总目标完成。

`POST /api/v1/queues/{queue}/preview` 接收与 apply 相同的版本化 Queue 文档及原始条件请求头。要求 operator 权限；auditor 返回 403，缺失/无效凭据返回 401，未配置授权时以 404 禁用该路由。针对本地重新构建服务的示例（使用实际地址/端口及本地提供的 operator token）：

```sh
curl --fail http://127.0.0.1:8080/api/v1/queues/orders/preview \
  -H "Authorization: Bearer ${RJS_OPERATOR_TOKEN}" \
  -H 'Content-Type: application/json' -H 'If-None-Match: *' \
  --data '{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["orders.events"],"replicas":1}}'
```

编辑时，将 `If-None-Match: *` 换为**编辑器最初加载时**的 `If-Match` 值，不能仅为让陈旧草稿通过而换成新版本。

返回 `plan`、`result`（含 operations/changes/impact/blocked）、`observed_at`、`base_revision`、`create_only`。计划内容版本与 `base_revision` 中带引号的声明 KV 版本不同；仅创建时 base revision 为空。被阻断的计划是成功的建议性读取：**200 且 `result.blocked=true`**，不是成功写入。`ready`、`noop` 同样只是计划结果，不代表健康或本次预览执行过写入。

实现复用 apply 的前置条件、DLQ 依赖校验及 topology 差异计算，观测前后核对声明。不获取写锁、不创建元数据桶、不保存声明、不确保 DLQ 基础设施、不修改 Stream/Consumer、不记录审计意图/结果。DLQ 校验沿用 apply 的顺序与限制，包括现有的双 Queue 环检查，不宣称支持通用依赖图环检测。与 apply 一样，不安全计划先于 DLQ 依赖检查返回。

请求体限制 1 MiB，读取上下文五秒超时，响应 no-store。文档错误/超限返回 400；名称不符、声明版本陈旧/创建冲突返回 409；缺失/非法前置条件返回 428；编辑目标或 DLQ 依赖缺失可能返回 404；后端错误返回 503。这些错误不修改资源。完整响应码已写入 [OpenAPI](../api/openapi.yaml)；生成 Plan 暂为通用 Schema，完整 Schema 仍属于 C-02 待办。

`observed_at` 表示建议性读取完成时间，不是原子 broker 快照时间。预览不预留版本或操作位置。草稿/基准/Schema 变化会使预览失效；最终 apply 仍需使用相同原始前置条件，并在正常写锁内重新校验安全。预览通过不证明后续授权、审计可用性、配额/容量或写入成功，broker 和依赖可在预览后变化。本轮观测读取沿用现有 Stream 范围枚举；这里限制的是请求时限，不代表已具备索引或大规模查询能力。

验证：后端测试使用只读 SDK/元数据包装，未实现的写入/写锁方法一旦调用会立即失败；覆盖创建/noop/阻断、旧版本、创建冲突、目标缺失、失败/取消、DLQ 缺失/合法/环和声明并发变化。HTTP 测试覆盖 operator/auditor/禁用授权、请求大小、错误/状态，以及零审计/apply/delete 调用。另有测试验证预览后其他写入发生，原条件 apply 仍失败。

相关 API/后端测试和 `go vet ./...` 通过。首轮 `go test -count=1 ./...` 其他包通过，但 `tools/baremetal-run/TestSupervisedLifecycle` 因 Windows 拒绝绑定临时端口失败；测试未干扰已有监听。重跑证据见执行记录。本轮未执行真实服务预览、浏览器接入、发布部署或远程操作。
