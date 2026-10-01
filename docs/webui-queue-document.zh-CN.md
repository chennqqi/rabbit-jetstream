# 无损可编辑 Queue 文档

[English](webui-queue-document.md) | [简体中文](webui-queue-document.zh-CN.md)

状态：源码已实现，尚未发布；C-02 的部分交付。现有声明存储和 apply 行为不变。

`GET /api/v1/queues/{queue}` 现新增：

- `document`：规范化版本化 Queue 文档；无法证明转换无损时为 `null`。
- `document_error`：不能作为编辑草稿时提供原因。

所有原字段和带引号的原始 KV `ETag` 均保留。HTTP 200 且 `document=null` 表示声明可查看，但不能通过此适配器安全编辑；不是 Queue 缺失或迁移成功。不修改 broker 资源或声明。现有匿名读取策略不变，D-05 仍待决。

后端重建 labels、原始 subjects/bindings、副本/存储、保留、投递、优先级和 DLQ，再运行规范 `BuildPlan`。生成的完整 Plan JSON 必须与原 Plan JSON 相等，包含内容版本、元数据、派生资源和警告。Queue 身份及声明/计划版本也必须一致。因此会拒绝缺少必要来源信息的旧计划、不支持的元数据/配置、名称不一致及内容版本变化。不会猜测缺失 Subjects 或静默填充导致有损的默认值。

默认值会规范化，所以时长/单位的字面格式、默认值显式或省略的表达可能不同，但生成配置不能变化。省略优先级仍省略，显式优先级零仍为零。Go 整数/纳秒值保留精度。浏览器必须使用[无损解码器](../admin-ui/src/api.mjs)，不能先用普通 `JSON.parse` 再转换大整数。

编辑器接入要求：

1. 只以 `document` 为草稿来源，保留支持字段和标签。document 为空时，不回退到旧浏览器 `queueFromPlan` 猜测。
2. 将本次读取的 ETag 作为不透明字符串，贯穿编辑、预览及 apply；文档生成的内容哈希不是修改前置条件。
3. 文档为空时禁止编辑并显示原因，但保留只读 Plan/观测查看。旧声明迁移需要单独显式流程，不能在 GET 时自动写入。
4. 文档/基准/Schema 变化使预览失效。表单和专家 JSON 使用同一草稿及精确序列化器。

执行的验证：

```sh
go test -count=1 ./internal/topology ./management/internal/api ./management/internal/jetstream ./api
node --test admin-ui/tests/api.test.mjs
go vet ./...
```

本地均通过。测试覆盖 labels、Subject 和 direct/topic/fanout bindings、DLQ、优先级省略/0/7/255、int64 上限、纳秒、输入无别名/无修改、拒绝不可逆计划、HTTP 增量兼容、精确大 ETag 和只读调用。不声称已完成真实 NATS 或浏览器接入。完整 Queue/Plan JSON Schema、部署能力和实际表单接入仍待完成；不能据此关闭 C-02、WP-02 或整体开发目标。
