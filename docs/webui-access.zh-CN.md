# WebUI 资源读取访问策略

[English](webui-access.md) | [简体中文](webui-access.zh-CN.md)

状态：负责人已批准 D-05，源码已实现，尚未部署。D-04 另已批准使用 React/Vite 构建 Go 嵌入式静态资源，不增加运行时前端服务或 CDN。

管理应用默认要求 `/api/v1/` 下资源 GET/HEAD 提供 operator 或 auditor Bearer 认证，包括 info、cluster、nodes、controller、Queue、Stream、Consumer。缺失/无效凭据返回 401；角色不足返回 403；未配置认证返回 404 `read_api_disabled`。升级调用方前配置已有的 `RJS_ADMIN_TOKEN`/`RJS_ADMIN_TOKENS`、`RJS_AUDIT_TOKENS` 或 OIDC 设置。不得把 Token 放入 URL、提交的文件或浏览器持久存储。

兼容性：匿名 API 调用方及旧版后台的匿名读取默认将不再可用，需要发送 `Authorization: Bearer …`。生产 React 登录/API 接入尚未完成，本次源码变更不代表后台已就绪。已有冻结二进制和远程验收不变。

仅同时设置 `RJS_LOCAL_DEMO=true` 并绑定字面量回环地址，例如 `RJS_HTTP_ADDR=127.0.0.1:8223` 或 `[::1]:8223`，才允许匿名资源读取。演示模式下通配地址、内网/公网 IP、DNS 名称（包括 `localhost`）均在后端初始化前拒绝启动。默认 false，无效布尔值不会启用演示。转发头不会授予例外。反向代理或隧道仍可能暴露回环监听：不得通过这两种方式公开演示监听。容器端口发布不属于该例外的支持方式。

演示模式绝不绕过 session、audit、preview 或写入认证。`/healthz`、`/readyz`、`/metrics`、`/admin/`、`/api/v1/openapi.yaml` 和 `/api/v1/native-sdk-contract.json` 仍公开，监控/静态资源暴露需通过网络控制限制。静态资源公开不代表有权读取受保护资源。Session 默认报告 `resource_read_policy=authenticated`，演示模式报告 `anonymous`。

公开 readiness 探针失败时只返回 `{"status":"not_ready"}`。服务日志只记录稳定错误类别，绝不记录依赖错误原文；连接串、主机或凭据材料既不进入探针响应，也不复制到日志。

受保护的管理读取、预览、导出、路由探测、审计读取及 Queue 写操作的后端失败遵循同一边界。HTTP 状态、稳定错误码及写操作阶段／影响证据继续保留，但后端生成的消息会替换为按错误码分类的安全文本；日志只保留稳定类别与安全操作上下文。客户端校验和前置条件错误由管理服务自身生成，因此继续返回可操作信息。

验证：`go test -count=1 ./management/internal/config ./management/internal/app ./management/internal/api ./api`。覆盖回环绑定边界、GET/HEAD、两类允许角色、凭据拒绝、未配置认证、伪造转发头和公开初始化路径。这些是本地源码测试，不是生产浏览器或发布资格证据。
