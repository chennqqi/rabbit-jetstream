# OpenAPI 模型生成

[English](openapi-codegen.md)

提交到仓库的 OpenAPI 模型由[发布 API 契约](../api/openapi.yaml)和权威的 [Queue 编写 Schema](../internal/topology/queue-schema.json)生成。公开 OpenAPI 文档有意让 `Queue` 组件指向需要认证的运行时 Schema 端点。`tools/openapibundle` 会在仅供生成的临时文档中替换该引用，并重写 Queue 内部的 `$defs` 引用；生成过程不依赖正在运行的服务端。

运行：

```shell
make generate-openapi
make verify-openapi-generated
```

PowerShell 入口为 `scripts/openapi-codegen.ps1`。脚本固定使用 `oapi-codegen` v2.8.0，并使用 `admin-ui/package-lock.json` 中精确版本的 `openapi-typescript`。输出为：

- `management/internal/apigen/models.gen.go`——只生成 Go Schema 模型；Handler 和路由继续手写。
- `admin-ui/src/generated/openapi.d.ts`——只生成 TypeScript 契约类型；现有带认证、租户感知和无损解码的 HTTP 传输层继续手写。

生成文件需要提交。CI 在临时目录中重新生成并比较精确内容。修改 OpenAPI/Queue Schema 或生成器版本后，应执行生成、审阅语义变化，并将对应输出一同提交。

标记为 `format: uint64` 的线整数会生成 Go `uint64` 和 TypeScript `bigint`。这是有意设计：Admin UI 的无损 JSON 解码器不得把 Broker 计数器、CID、序列游标或副本 Lag 缩窄为 JavaScript `number`。不得通过普通 `JSON.parse` 消费生成的响应类型。

OpenAPI 中的通用对象仍会出现在生成结果中，但这不允许移除运行时校验。只有相关响应 Schema 已具体化，并具备等价的失败与精度测试后，才可逐步迁移 Handler 和 UI 解码器。

