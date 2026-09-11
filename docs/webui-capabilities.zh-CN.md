# 控制台能力与部署期望

[English](webui-capabilities.md) | [简体中文](webui-capabilities.zh-CN.md)

状态：源码/本机候选实现，不代表冻结发布或生产资格。C-02 已包含版本化的[全字段明确类型 Queue 编写 Schema](webui-queue-schema.zh-CN.md)，完整 Schema 驱动表单仍待完成。

`GET /api/v1/console/capabilities` 要求 operator 或 auditor bearer，即使 local-demo 允许匿名读取资源也一样。凭据缺失/无效返回 401，角色不足返回 403，未启用认证返回 404，不支持的查询参数返回 400。成功响应使用 `Cache-Control: no-store`，schema 版本为 `rjs.console-capabilities.v1`。不读取 NATS/monitor，不修改数据；元数据读取成功不代表后端就绪。

## 显式配置

`RJS_DEPLOYMENT_PROFILE` 接受 `unknown`（默认）、`standalone` 或 `cluster`。非法值在 telemetry 和后端初始化之前导致启动失败。设置模式仅声明操作者的部署期望：不会创建节点、修改副本、配置存储、验证容量或建立验收资格。官方 standalone 与 cluster Compose 清单会声明对应模式。Helm Chart 根据经过 Schema 校验的 `nats.replicaCount` 配置产生声明：1 为 standalone，3 或 5 为 cluster。自定义部署未显式配置时保持 unknown。

`RJS_RELEASE_MANIFEST` 可选指定本机 `rabbit-jetstream.io/release-bundle/v1alpha1` 清单。配置后，进程必须是 revision 绑定的干净发布构建，且清单中的版本、server revision 与分帧内嵌 WebUI 身份必须和可执行程序完全一致，否则在 telemetry 与后端初始化前启动失败。加载器只读，文件上限 1 MiB，拒绝未知字段、不完整结构和尾随 JSON；API 不暴露本机路径或制品清单。裸机验收 supervisor 会自动传入经过校验和验证的冻结 `runtime-manifest.json`。开发及普通 Compose 运行保持未配置。

已知模式报告来源 `configuration`，未知模式报告 `unspecified`。不根据可达节点或元数据副本配置推断模式。配置在管理进程重启后生效，候选 Settings 页面只读展示。

## 响应语义

| 字段 | 含义 | 不证明 |
| --- | --- | --- |
| `deployment.profile/source` | 显式部署期望或未知 | 实际拓扑、容量或健康 |
| `queue.apiVersion/kind` | 支持的 Queue 文档身份 | 完整 JSON schema |
| `queue.supportedReplicas` / `supportedStorage` | 解析器支持值（1/3/5、file/memory） | 已通过生产验收的配置 |
| `queue.minimumPriority/maximumPriority` | 解析器范围，当前为 0–255 | 已验收优先级范围 |
| `queue.requiresExplicitReplicas` | 此契约始终为 true | 自动选择 R1/R3 |
| `queue.defaults` | 从标准 `Queue.Default()` 获取的省略 storage/delivery 默认值 | 完整有效 Queue 文档或部署建议 |
| `features` | 已实现 API 契约标识 | 权限、当前后端可用性或完整界面实现 |
| `qualification.status` | `unreported` | 既不代表发布验收批准，也不代表验收失败 |

当前默认值为 storage `file`、ACK wait `30s`、最大投递次数 `5`。不报告副本数默认值，因为解析器要求明确指定受支持副本数。此展示不会向编辑草稿复制默认值。没有匹配清单时，资格状态为 `unreported`；匹配后，API 以 `reported` 报告清单中的原始资格声明与文件 SHA-256。这只记录已绑定声明，绝不会转换为 `qualified` 结论。发布审批、签名、原生 soak 与 Canary 证据仍是独立门槛。

Settings 独立于已验证会话读取能力。刷新失败或契约不兼容时移除旧能力值，报告未知/不可用，不以节点数、权限或前端常量替代。刷新为手动只读操作。现有引导创建仍要求显式副本/存储选择，并通过服务端预览验证。候选写操作流程已接入下述读取保护；完整 schema 驱动表单和自动刷新偏好仍待后续接入。

## 能力绑定预览与发送前检查

候选版本对所有创建/编辑/删除模型启用能力检查。预览前后分别读取认证能力，检查所需契约标识，并将预览绑定到响应的规范化指纹。对象键顺序及 features/replicas/storage 集合内部顺序不视为变化，部署、默认值、支持范围、schema 或其他契约数据变化则使绑定失效。能力缺失、不兼容或缺少必要标识时禁止继续。

PUT 或 DELETE 之前再次读取，必须匹配预览绑定指纹。失败则清空预览与确认，保留草稿/原始 ETag 或删除证据，回到 preview-error，不生成写请求 ID、不发送写入。删除 force 和手输确认重置，必须明确重新预览、确认后才能继续。进入实际写入调用后的错误仍保守地视为未知，不能自动重试。下载证据保留本次审阅绑定的能力。

前端检查是时点性保护，并由下述接收实例前置条件补充，不是预留或集群级原子快照。后端继续独立执行认证、声明前置条件、规划与归属检查。已声明的 Queue Schema 版本及内容 ETag 现已参与绑定；必需的 Schema 读取及其边界见 [Schema 契约](webui-queue-schema.zh-CN.md)。没有引入后台轮询或隐藏写入重试。

## 共享能力观测

共享 API 完成的能力读取（包括 Settings 刷新）会通知同一凭据代次下保留的创建/编辑/删除模型。正文或版本变化、响应不兼容、读取失败时，未提交的审阅及确认会在返回页面前失效。保留草稿原文和原始声明 ETag；删除的 force、手输确认和勾选确认重置。能力未变化时保留审阅。继续操作必须明确重新预览、确认。

这些通知不会重置或解锁正在处理的操作、未知结果、成功回执及归档证据。归档/丢弃的模型取消订阅。使用旧凭据发起的响应不会通知新会话，观察者异常不会改变传输结果。这是基于已观测结果的失效机制，不是持续监控；尚未观测到的变化仍依赖上述预览/发送前检查及接收实例检查。

## 接收实例的契约前置条件

能力 GET 返回不透明强 `ETag`（`"rjs-capabilities-v1:<64 位小写十六进制字符>"`），由不可变能力数据和管理端报告的构建版本生成。部署期望与标准默认值参与计算，进程名称、运行时长、凭据和资源观测不参与。此标识与 Queue 声明 ETag 不同。它不是二进制证明：不同二进制若报告相同版本与能力数据，可能具有相同标识。

`capability-preconditions` 功能标识声明支持 `X-RJS-If-Capabilities-Match`。候选创建/编辑/删除流程要求此功能及有效能力 ETag，同时比较正文指纹与不透明版本，并在 POST 预览、GET 删除预检、PUT 和 DELETE 上发送原始版本。缺少该契约的旧服务端不能启用候选界面写操作。

认证后，接收实例在解析/规划或记录审计意图之前，按自身不可变契约检查该可选请求头。版本不匹配返回 412 `capabilities_changed`；空、弱、格式错误、重复或组合值返回 400 `invalid_capabilities_precondition`。缺少此头仍保留旧 API 客户端兼容性。该条件不替代 Queue `If-Match`/`If-None-Match`、归属检查或删除确认，也不冻结消息、Consumer、外部写入者或集群。

只读预览的 412 可使审阅失效。一旦 PUT/DELETE 进入传输，最终 412 仍不能证明此前透明浏览器重放没有执行；候选状态保持未知，不会对此显示“未发送写入”，也不自动重试。API Token、TLS 和逐请求认证仍有必要，能力标识不是权限或秘密。

验证包括认证下的 nil 后端 handler 测试、标准默认值与解析器接受范围检查、部署模式初始化测试、前端无效/过时响应，以及本机浏览器 Settings 刷新/拒绝访问覆盖。这些检查不建立生产资格或实际部署容量。
