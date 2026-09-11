# Admin UI

[English](README.md) | [简体中文](README.zh-CN.md)

## 内嵌 React 控制台

已批准的 React/Vite 实现在 `src/`，与合成数据设计原型独立。依次执行 `npm ci --ignore-scripts`、`npm test`、`npm run build`，生成 `build-candidate/` 静态候选制品，资源 URL 以 `/admin/` 开头。PowerShell 若限制脚本执行，使用 `npm.cmd`。已包含依赖锁文件，本地离线干净安装通过。最终嵌入部署不需要运行时前端服务或 CDN。

候选版本已接入 Bearer 身份、Queue/Stream/Consumer 读取、节点证据、审计查询、经审阅的 Queue 变更、总览刷新偏好及兼容性元数据。Queue 配置页包含[只读 DLQ 诊断](../docs/webui-dlq-diagnostics.zh-CN.md)，进程级计数不代表逐 Queue 转移证明。凭据和草稿仍仅在内存。这不是已完成的管理控制台，完整需求范围和证据见开发记录。集成时须与管理 API 同源，单独运行 Vite preview 没有后端。`npm run dev` 绑定回环端口 18225；`npm run preview` 使用 18226。两者均不会启动或修改管理服务。

已验收候选制品会原样复制到 `dist/`，嵌入 `rjs-management` 并通过 `/admin/` 提供服务。内嵌控制台只访问版本化管理 HTTP API，不直接连接 NATS。测试会解析构建入口，并要求其中引用的每个带哈希脚本和样式都存在且可由 Go Handler 返回。不得手工修改生成的 `dist/` 资源；应重新构建并验收 `build-candidate/`，再晋升完全相同的文件集。

2026-09-11 的晋升已通过本机构建的真实 Go 二进制在 Chromium 中执行，覆盖 127 项真实服务检查和 23 个 axe 快照。其余生产页面、待批准架构项和发布资格仍记录在[开发记录](../docs/webui-development.zh-CN.md)中；完成内嵌晋升不等于控制台全部完成或版本获准发布。默认资源读取认证由[访问策略](../docs/webui-access.zh-CN.md)定义。

`make test-admin-ui` 是当前 React standalone Compose 浏览器门禁，会在 Chromium 和 Firefox 中执行认证、经审阅的 Queue 创建/修改/删除、会话、移动端失败状态和 axe 检查。执行 `pwsh -NoProfile -File tests/admin-ui/run.ps1 -DeploymentProfile cluster` 可对官方三节点 cluster 清单运行同一门禁；两种模式都会断言服务端声明的对应模式。本机隔离测试可通过 `RJS_TEST_EMBEDDED_UI=1` 和显式指定的最新管理二进制验证 Go 实际提供的资源。
