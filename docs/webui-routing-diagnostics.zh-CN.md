# WebUI 路由诊断

[English](webui-routing-diagnostics.md) | [简体中文](webui-routing-diagnostics.zh-CN.md)

## 状态与边界

WEB-020 正在开发。已实现纯后端计算 `topology.ProbeRouting`、已保存声明的读取 API，以及候选 Queue 路由标签页中的探测。这不代表发布就绪；尚未替换嵌入式发布资源。

计算通过 `BuildPlan` 验证单个 Queue 声明，复用 `QueuePublishSubject` 翻译交换机目标。不发布消息、不读取 broker、不修改资源、不解析其他 Queue，也不证明投递成功。计算前复制规划器会排序的切片，避免诊断读取改变调用方草稿的顺序。

## 输入与证据

- 输入只能选择字面 NATS Subject 或交换机名称／类型／路由键。混合输入、带通配符的探测 Subject、非法声明、不支持的翻译均失败，不能伪装成成功但无匹配。
- 合法但不匹配的 Subject 返回空匹配数组，与验证失败分开表达。
- 结果包含 Queue、声明修订、生成的 Stream 名称、解析后的 Subject、全部生成的 Stream Subject 及匹配项，以及各绑定的键、生成 Subject 和匹配项。这些是声明层面的事实，不是实时 Stream 观测。
- `*` 匹配恰好一个 token；NATS `>` 匹配一个或更多 token。支持的 RabbitMQ 末尾 `#` 由现有规划器展开为前缀和前缀加 `>`，保留零层匹配。中间位置的 `#` 仍不支持。
- 普通 Queue 包含生成的 ingress Subject。优先级 Queue 改为捕获优先级 Subject，因此绑定匹配可以与 **Stream 不匹配** 同时成立。支持字面优先级 Subject 探测；明确拒绝交换机到优先级的解析，因为探测没有实现 SDK 的优先级选择。
- 不收集消息正文、凭据、订阅或客户端身份元数据。本项工作不解决另行待批的订阅可见性决策。

## 剩余集成

1. 已实现：`GET /api/v1/queues/{queue}/routing-probe?subject=orders.created`，或 `?exchange=events&type=topic&routingKey=orders`。使用现有资源读取权限，包括显式本机演示例外。拒绝重复／未知参数、混合模式和非法编码；原始查询 ≤4096 字节，单个解码值 ≤1024 字节，Queue 名称 ≤256 字节。在三秒期限内读取一个已保存声明，验证身份／修订与完整还原，不执行修改。声明不存在返回 404；不可还原／不一致的证据返回 409 `routing_declaration_unavailable`；不支持的探测语义返回 400 `invalid_query`。响应上限 1 MiB，超限返回 503 `routing_probe_limit` 而非截断。成功响应包含 `no-store` 和精确加引号的 KV 修订 ETag。单元／API 测试覆盖这些边界、取消和 GET／HEAD 鉴权。OpenAPI 描述返回字段。
2. 候选版本已实现：双语显式探测操作、字面／交换机模式、分开的生成 Stream 与逐绑定证据表、声明修订和本机完成时间。探测前仍展示原声明映射。不提供发布／重放操作，不自动探测。修改输入、切换模式、取消和卸载均清除证据，迟到响应不能恢复旧结果。内容修订或 KV ETag 变化时要求刷新页面，不混合版本。错误清除结果，并将拒绝／未启用／缺失／变化／无效／超限／不可用与合法无匹配区分。前端测试覆盖这些状态及字段校验，没有添加 JavaScript 路由翻译器。
3. Chromium 和 Firefox 现已覆盖真实 direct/topic/fanout 绑定结果、实时声明变化、取消／重试、中文移动端结果、键盘访问结果最后一列及畸形／未知错误处理；当前候选在两种浏览器中均通过完整套件 107 项检查。移动端表格使用最小宽度及内部横向滚动，已通过断言和截图检查。剩余：审查全部状态与原始声明映射表的完整无障碍能力（仅局部对比度扫描不能证明）。更大规模和发布资格仍是独立门槛。精确候选证据参见开发记录。
