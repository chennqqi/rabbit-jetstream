# 兼容性控制台

[English](webui-compatibility.md) | [简体中文](webui-compatibility.zh-CN.md)

候选 `/admin/compatibility` 页面为已认证 operator 和 auditor 提供 WEB-034 只读工作区，组合独立的管理构建信息、公开原生 SDK 契约和已有服务端能力面板。失败清除受影响的元数据，不将其当作当前信息，也不丢弃成功的其他来源。导航取消阻止迟到响应恢复数据。不提供升级、重启、功能开关或消息操作。

## 构建 API

`GET /api/v1/console/build` 即使在 local-demo 模式也要求 operator/auditor 授权。拒绝查询参数，不调用 Broker。成功响应使用 `Cache-Control: no-store`：

```json
{"schemaVersion":"rjs.build-info.v1","version":"v0.1.0-rc.2","goVersion":"go1.25.0","os":"linux","arch":"amd64","revision":"…40 个小写十六进制字符…","revisionSource":"release-build","modified":false,"uiAssets":{"algorithm":"sha256-framed-files-v1","digest":"…64 个小写十六进制字符…","fileCount":4}}
```

此示例描述管理可执行程序，不代表全部 Broker 节点。`uiAssets` 标识该进程实际嵌入并提供的精确封闭文件集。SHA-256 构造会对排序后的每个相对路径与字节长度进行分帧，因此路径和内容边界没有歧义；它既不是签名，也不是资格证据。发布打包现会注入精确的干净 server revision，即使发布二进制有意使用 `-buildvcs=false`；只有 revision 为有效 40 位且干净、版本绑定流水线设置了 clean marker 时才报告 `revisionSource=release-build`。仅手工注入 revision 会报告 `injected`，不会声明 `modified=false`。普通构建可报告 `go-build-info`。缺失仍表示未报告，不代表干净构建。不返回完整 Go 设置、链接器参数、本机路径及依赖列表。

## SDK 与能力

页面读取已有公开接口 `/api/v1/native-sdk-contract.json`，不改变其缓存策略。展示 Schema 标识、声明可用阶段、投递保证、发布确认、确认策略和必填/可选消息头名称。不推断已安装客户端兼容性或已发布 SDK 版本；特别是 `native-sdk-implemented-unreleased` 保持原样，不改成绿色“已发布”。原生契约不代表 RabbitMQ 线协议兼容。

复用的能力面板独立报告解析器支持值、部署期望、编写 Schema 及发布清单状态。各来源不是原子快照，各节点版本仍在节点详情页。配置 `RJS_RELEASE_MANIFEST` 后，进程启动时会把清单版本、server revision 与 WebUI 身份精确绑定到当前干净发布构建；页面只以 `reported` 展示清单原始声明及文件 SHA-256，绝不会把声明改写成 `qualified`。

本地发布包会在 `release-manifest.json` 的 `webui` 字段记录同一身份，并复用 Go 实现而不是在打包脚本中另写一套哈希算法。裸机派生打包会拒绝缺失／非法身份、原样传递该字段，并自动向 management 子进程提供冻结的 `runtime-manifest.json`。配置的清单缺失、过大、格式错误、结构不完整或身份不匹配时，应用初始化会失败。这是精确绑定，不是签名验证、审批证据校验或对清单声明的独立认证。

## 验证

Go 测试覆盖 Broker 不可用时的 operator/auditor 认证访问、匿名拒绝、非法查询、缓存策略、干净与普通 revision 注入、对路径／内容变化敏感的确定性制品哈希及敏感构建参数排除。契约测试要求本地打包、management Dockerfile 和 GitHub Release 工作流同时注入 revision 与 clean marker。前端测试覆盖有效／非法 revision 来源和资源身份、旧服务端字段缺失、未知 VCS 状态、保留未发布 SDK 阶段、独立失败、只读传输及取消后的迟到响应。嵌入式 Chromium `artifacts/webui-live-nFrxil/report.json` 使用 `-buildvcs=false` 且按发布方式注入 revision/clean marker，通过 131 项检查和 24 份可访问性快照，并独立核对所有实际提供 UI 文件与 API 身份一致。晋升后的 UI 身份为 `1d044d84576088a1f6e7b6811b18c6f52569daa0fafc138823b0ed9a316043c7`。这些测试不证明签名或发布资格。
