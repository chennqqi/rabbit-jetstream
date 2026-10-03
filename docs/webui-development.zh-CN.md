# WebUI 开发执行记录

## 浏览器 SSO（WEB-025）

已按批准架构实现 OIDC Authorization Code + PKCE S256，且不创建服务 Cookie／会话。浏览器只在 `sessionStorage` 临时保留一条 state／verifier，回调时先删除该记录并清除地址栏参数，再交换授权码；验证后的 ID Token Bearer 通过既有会话模型仅保留在页面内存中。管理服务只使用发现得到的固定 token endpoint，验证返回的 ID Token 与角色映射，绝不返回 refresh/access token 或签发方原始错误。静态 Bearer 输入继续保留。部署配置、Helm fail-closed 关系、公开 OpenAPI 引导契约和双语运维说明均已同步。

后端 discovery／JWKS／token 交换测试、API／配置／OpenAPI 测试、382 项可运行前端测试（另有一项预期的 Windows 符号链接跳过）、候选构建、四文件晋升及字节核对均通过。最终内嵌 Chromium `artifacts/webui-live-z6DGZP/report.json` 针对 management SHA-256 `915cf6b0b9edb05b05624a96a93fc7667b2b174bb1648e67a3ea7a241bda107c` 通过 135 项检查。此前 `PsuCHz`、`vVswdk` 和 `dHO1XX` 失败记录保留：Connection detail 断言错误地假定在固定窗口内必然自动成功，而产品契约允许不可用时保留历史并从完成时退避。最终脚本以显式成功读取提供该集成证据，精确定时／退避由确定性模型测试负责，检查标签也不再声称覆盖详情周期刷新。这证明本地实现和内嵌集成，不代表外部生产 IdP、最终发布冻结、Canary 或原生主机资格验收。

## 有界元数据诊断包（WEB-024）

负责人已批准实例内有界内存诊断任务。候选版本现提供仅认证 operator 可创建、仅创建者可查状态／取消／下载的接口，固定 30 秒采集超时、十分钟过期、一个在途采集器、两个保留身份及 8 MiB 归档上限。过期同时由真实定时器及访问时检查执行；取消、关闭及迟到完成均不能重新发布字节。应用关闭会先关闭任务存储，再关闭 JetStream 客户端。

固定进程内采集器包含构建身份、服务端能力、脱敏账户元数据、控制器状态及投影后的节点观测，不接受 URL、路径、命令、请求头或来源选择。原始 monitoring 错误替换为静态可用性说明，URL 凭据会被移除。现有枚举接口尚不能证明硬工作量上限，因此 Queue／Stream 清单、消息、订阅、日志及环境变量继续排除。创建、取消、下载授权及服务端写出完成均记录审计；服务端完成明确不等于浏览器已收到。

双语 `/admin/diagnostics` 工作区按权限显示，刷新页面绝不创建任务；页面展示 partial／终态及清单，创建 Blob URL 前校验 ZIP 媒体类型和长度，并在替换／导航时撤销 URL。后端 API／生命周期测试、全仓 Go 测试及 vet、全部 371 项可运行前端测试（另有一项预期 Windows 符号链接跳过）、候选构建、晋升及 dist 校验均通过。完整内嵌 Chromium 回归 `artifacts/webui-live-hzzJxJ/report.json` 使用精确晋升的四个 UI 文件并通过全部 133 项检查，包括真实显式创建／状态／下载、双语渲染和可访问性检查；下载 ZIP 为 2,987 字节。未使用远程主机或容器。

[English](webui-development.md) | [简体中文](webui-development.zh-CN.md)

目标：完成 WebUI 和后端开发。本记录不缩减[需求](webui-rabbitmq-requirements.zh-CN.md)、[设计规划](webui-design-plan.zh-CN.md)、[API 契约](webui-api-contracts.zh-CN.md)或[选定视觉设计](webui-selected-design.zh-CN.md)的范围。P0 是首个交付门槛；P1 保留为后续待办，P2 按文档需要独立架构批准。完成某个模块或 P0 不代表已具备调研中的全部能力。

## 当前证据和剩余工作

### 有界模板与批量变更

WEB-027 现已具备端到端有界更新路径。包含全部标签的 Queue 导出可生成版本化 `rjs.queue-change.v1` 更新包，携带精确来源 ETag 与无损声明。仅限操作员的 `POST /api/v1/queues/change-plan` 在共享期限内预览 1–100 个更新包，并按输入顺序为每项分别返回就绪、阻塞、无效、冲突、缺失或不可用状态；它从不写入，也不宣称原子性。新的“批量变更”页面会核验全部本地包和完整服务端响应，如实显示部分失败，且只允许就绪目标进入独立审阅。打开保留编辑器前会重新读取 Queue 并匹配原始 ETag；既有的新预览、确认、条件 PUT 和未知写入隔离仍是必需步骤。

前端 365 项测试全部通过，另有 1 项 Windows 符号链接夹具按预期跳过；最终投影修复前全仓 Go 测试和 vet 已通过，修复后定向 API／OpenAPI 测试通过。候选构建、四文件内嵌晋升和核对通过。最终内嵌 Chromium 证据 `artifacts/webui-live-b5ugik/report.json` 通过 132 项检查，覆盖真实 ready／conflict 组合、仅一次独立确认更新、失败目标保持不变、更新包实际下载及精确 int64／标签／ETag 核对和 Blob URL 回收。参见[Queue 模板与批量变更](webui-queue-templates.zh-CN.md)。

### 精确连接身份搜索

WEB-019 现已支持在唯一解析的节点上按客户端名称、授权用户、账户和 MQTT 客户端 ID 精确搜索。只读 POST 契约使敏感值不进入浏览器历史和管理请求 URL；响应仍是既有的纯数值连接投影，绝不回显身份、客户端名称、地址或凭据。原生身份过滤器在上游分页前执行，管理服务再以每批 200 行执行两次 CID 稳定性扫描，计算真实过滤总数。固定核心没有名称过滤器，因此名称搜索只在节点打开连接总数不超过 1,000 时执行：管理服务读取两次完整的私有 CID／名称投影，拒绝变化，只对稳定的第二次观测过滤，并在序列化前丢弃所有名称。原生过滤匹配超过 1,000 条或名称搜索节点超过 1,000 个连接时均返回 422，不展示部分结果。

以下为该段当前最终证据，并取代紧随其后的上一轮账户搜索证据：内嵌 Chromium `artifacts/webui-live-yMAIVH/report.json` 通过 133 项行为检查和 24 份可访问性快照，自动违规为零，包含真实 `rabbit-jetstream` 客户端名称搜索、URL／响应无名称回显、冻结条件刷新和显式清除。晋升后的四文件 UI 身份为 `2470bf1d235099a2b75d5dd88dacd794b8b71570b14d468e99d4336bf88b1b04`。有界真实集成证据 `artifacts/connections-live-W75a3N/report.json` 绑定 management SHA-256 `96d00ed3ded58d0a1ade0bd65ee159f7bc0e308a15c0c8511d74be736367f23a`，验证真实命名客户端过滤及无回显，同时保留 1,000 订阅成本和 1,000／1,001 身份边界检查。

WebUI 仅在已挂载页面模型的内存中保留已提交精确值，以密码输入框呈现，不写入路由或链接；即使用户编辑尚未提交的控件，结果翻页仍使用冻结的已提交条件。Monitoring/API/OpenAPI 测试、vet、全部 361 项前端测试（另有一项预期的 Windows 符号链接夹具跳过）、候选构建和受保护的四文件内嵌晋升均通过。最终内嵌 Chromium 证据 `artifacts/webui-live-8p3vq8/report.json` 通过 133 项行为检查和 24 份可访问性快照，自动违规为零，覆盖真实 `$G` 账户搜索、URL 不回显、冻结条件刷新和显式清除；晋升后的 UI 身份为 `37729e9ba53a696c9544b13e0992b357bae68c9d72a62d2d0f5a520c4a3630b4`。`tools/upstreamcheck` 现在只解析成功 Git 命令的 stdout，同时在失败时保留 stderr 诊断，因此宿主 Git 警告不会再被误判为未跟踪文件；原失败夹具、真实本机子树校验、完整 `go test ./...` 和完整 `go vet ./...` 均已通过，且未弱化 tree／commit／修改检查。正式生产主机资格验收仍是后续工作；未使用远程主机。

本机有界规模证据 `artifacts/connections-live-dBLGtC/report.json` 已通过，运行前后均绑定 NATS SHA-256 `847b0be2b5b1011064ec099436e3e2c85eb47c2b4e6c774b02c9fdd8922eff48` 和 management SHA-256 `15c66bf75109041d017754a4ff3cea9e9ca79e6d001decfed076237c2bcb0ba8`。在 Windows 10.0.26100 x64、22 逻辑 CPU、33.8 GB 内存主机上，50 次完整 1,000 订阅读取的 API P99／最大值为 13.74 ms；读取期间 697 次连续同客户端 PING 的 P99 为 4.20 ms、最大值 6.40 ms；179 次 NATS RSS 采样相对 26.4 MB 基线未观测到峰值增长。结果通过报告中 2,000 ms API、250 ms PING 和 64 MiB RSS 增量门槛。同一次运行还验证 1,000 个账户匹配连接的精确身份搜索首尾页、真实总数和无身份回显，两页合计 140.53 ms；第 1,001 个匹配连接返回 422 且不返回部分行。测试未发布消息，未改变远程或已有进程。这是绑定候选制品和所记录主机的有界证据，不是正式生产容量或长稳资格。

### 有界连接订阅详情

WEB-019 现已具备安全展开订阅所需的内部两阶段 monitoring 原语。它只解析已配置的精确节点与 CID，先读取数值连接详情；订阅数未知或超过 1,000 时，在展开前拒绝。随后只针对同一 CID 请求 `subs=detail`。第二次观测必须保持节点／CID 及精确计数；发生变化、行缺失、SID 重复、UTF-8／控制字符非法、计数为负或标识超限都会使整份观测失败。SID 始终保持精确字符串，重复 Subject 保持独立行，缺失的可选最大值不会伪造为零。投影不包含地址、客户端名、账户、凭据、JWT、证书或原始 connz 对象。

受保护且不接受查询参数的端点和 Connection 详情 UI 现已完成。操作者显式操作前，UI 不会发起订阅读取；界面说明标识可能包含业务敏感信息，只对已接受的完整观测执行本地筛选／分页。瞬时不可用可保留明确标注的历史证据；超限、连接消失、拒绝、身份歧义和数据不兼容都会清除证据。会话／导航销毁会取消请求并清除订阅行。界面绝不推断 Consumer 所属关系、积压或健康。

Monitoring/API/OpenAPI 测试、vet、全部 360 项前端测试（另有一项预期的 Windows 符号链接夹具跳过）、候选构建和受保护的内嵌晋升均通过。最终嵌入式 Chromium `artifacts/webui-live-jsNcJ4/report.json` 通过 132 项行为检查和 24 份可访问性快照，自动违规为零。验证覆盖无隐式请求、真实测试自有连接读取、精确大整数、重复 Subject、本地筛选、超限拒绝／恢复、双语文本及窄屏布局。晋升后的四文件 UI 身份为 `e14d3da934c456fcc4773e9376a6bb7dee752bf5df78dffde80aa48c55735264`。客户端名称／身份搜索及独立的 1,000 行／client-lock 成本资格验收仍属于 WEB-019 待办。未改变远程主机或已有进程。

### 发布运行时精确版本身份

WEB-034 现已让正式发布构建的 management 进程具备精确 server revision 身份，即使发布二进制有意使用 `-buildvcs=false`。本地发布打包、management Dockerfile 和 GitHub Release 工作流都会同时注入 40 位 revision 与独立 clean-build marker。只有两者同时成立时才报告 `revisionSource=release-build` 和 `modified=false`；仅手工注入的合法 revision 会标为 `injected`，不会声明工作树干净，普通 VCS 元数据仍标为 `go-build-info`。已认证的兼容性工作区会严格校验并展示该来源。

可选的 `RJS_RELEASE_MANIFEST` 路径现在会在 telemetry 或后端初始化前加载有大小限制且严格解析的本地发布包清单。只有当前进程属于 revision 绑定的干净发布构建，并且清单版本、server revision 与内嵌 WebUI 身份全部精确一致时才会接受。认证 capabilities API 只暴露中性的 `reported` 状态、清单原始声明及其 SHA-256；不会编造 `qualified` 结论，也不会暴露路径或制品列表。原生裸机 supervisor 会自动传入经过校验和验证的冻结 runtime manifest；未配置的开发进程继续报告 `unreported`。

定向 Go/API/config/app/裸机/契约测试、vet、全部 355 项前端测试（另有一项预期的 Windows 符号链接夹具跳过）、候选构建、受保护的制品晋升和差异检查均通过。最终嵌入式 Chromium 证据 `artifacts/webui-live-zQxPm8/report.json` 使用重新构建的发布式 `-buildvcs=false` 二进制，通过 131 项行为检查和 24 份可访问性快照，自动违规为零。晋升后的四文件 UI 身份为 `1ad06ed2b8d2abe73206b0dc29a01f5aa79f5889d10a3b6e16191014b538d6fe`。一次浏览器尝试缺少新版 Playwright 运行时，一次正确拒绝了未绑定版本的合成能力响应，一次 axe 运行因后端源码随后变化而主动停止；均不作为通过证据。签名验证、已安装客户端兼容性及最终 release-approval/Canary 证据仍是独立工作。未使用远程主机。

### Consumer 副本与配置节点关联

WEB-018 现将一个精确 Consumer 的计数／配置／副本证据，与具有独立时间戳的配置节点监控读取组合展示。报告的成员名称仅匹配 varz 身份有效且 server name 唯一的节点；重复名称表示歧义，无匹配保持未解析，不转化为节点缺失。节点读取失败、过期或证据不完整会撤销当前关联，但不改变独立的 Consumer 观测。界面明确不宣称完整成员、原子快照、节点健康或积压因果关系。

八项诊断模型测试及全部 355 条前端测试通过，另有一条 Windows 符号链接夹具跳过。候选 Chromium `artifacts/webui-live-VCWwR0/report.json` 与晋升后的嵌入式 Chromium `artifacts/webui-live-yycovk/report.json` 各通过 131 项检查和 24 份可访问性快照，覆盖唯一匹配、未解析成员、节点失败／恢复、双语及移动端／键盘路径。晋升后的四文件身份为 `fff7060cf6b9ce5a95681d12eaf8b76ca8a286a695499b6aed15b358bca50511`。历史趋势、权威节点健康契约和真实饱和验收仍是独立工作；未使用远程主机。

### 全局 Consumer 代次与列表

已确认的“Consumer 先列表、再筛选”设计现已端到端实现。operator 显式构建有界的实例内完整代次；日常列表、筛选和分页读取绝不会重新扫描 broker。失败、超时、超限或声明不一致的采集不能发布部分行，只能把旧完整代次保留为 stale。界面区分 unavailable、collecting、stale 与真正空结果，以 Stream 加 Consumer 名称作为规范身份，保留精确零计数，并支持 Queue、Stream、名称、durable、模式和状态在代次绑定分页前筛选；auditor 不能启动采集。

后端/API/契约测试、352 项前端测试（另有 1 项 Windows 符号链接测试按预期跳过）、候选构建和嵌入晋升均通过。最终嵌入式 Chromium 证据 `artifacts/webui-live-PI9GIr/report.json` 通过 129 项检查和 24 个无障碍快照，其中包含独立 Consumer 列表审计。晋升后的 4 文件 UI 身份为 `2b59c95b70711737cda35c556472fcd2ad5887edd7ecb47da66b4dd9504c019f`。文档约定的 10,000 Queue / 100,000 Consumer 延迟与内存目标，仍须在注明硬件上实测后才能认定达标。

### 有界 Queue 列表 Stream 观测

Queue 列表现在会在声明存储、请求副本和 Plan 版本旁展示观测到的 Stream 状态、精确存储消息数及 Consumer 数。后端固定执行一次完整声明枚举和一次完整 Stream 枚举，仅按声明 Plan 的精确 Stream 身份连接，再沿用既有筛选／排序／分页契约；浏览器和后端均不会逐行读取。Stream 枚举失败时保留声明页，但观测标为不可用且不伪造零；缺失与身份冲突保持区分。显式零会经过无损解码保留。

“存在”要求既有拓扑 reconcile 逻辑确认 Stream 配置及管理 ownership 元数据没有变化，并要求 R3/R5 的可用副本证据均为 current。界面刻意标为“Stream 一致；未检查 Consumer 配置”，而不是健康：这次有界列表读取不会逐个检查 Consumer 配置、客户端连接、投递或收敛。真实 NATS 首轮发现未设置限制的零与负一表示在语义上等价；实现现复用 reconcile 的规范化逻辑，没有复制错误比较器。

定向后端／API／拓扑测试与 vet、347 项前端测试（另有 1 项 Windows 符号链接夹具跳过）、候选构建、OpenAPI 检查及差异检查通过。候选 Chromium `artifacts/webui-live-lewfPi/report.json` 与晋升后的 embedded Chromium `artifacts/webui-live-Hl8ySC/report.json` 均通过 127 项检查和 23 个无障碍快照。真实行证明存储消息为零、实际有 3 个优先级 Consumer、Stream 状态匹配且声明为 R1；embedded 运行还保留了精确 UI 制品身份。首轮原始字段比较报告 `artifacts/webui-live-nkh202` 正确失败，不作为通过证据。未使用远程主机。

### 精确内嵌 WebUI 制品身份

需认证的 build 接口现在会报告 management 进程实际嵌入并提供的封闭 `admin-ui/dist` 文件集的确定性 SHA-256 身份。摘要除内容外还对排序后的相对路径和字节长度分帧，因此路径重命名或简单拼接边界变化不会产生歧义。兼容性页会严格校验并显示该身份与文件数，同时仍兼容尚未报告该字段的旧服务端。这是运行时制品身份，不是签名、发布清单绑定或资格结论。

Go/API/契约测试、vet、346 项前端测试（另有 1 项 Windows 符号链接夹具跳过）、候选构建及差异检查通过。Chromium 候选证据 `artifacts/webui-live-ZVCH2T/report.json` 通过 127 项检查和 23 个无障碍快照，随后候选通过受保护流程晋升了 4 个文件。本机重新构建的 management 二进制以 embedded 模式提供晋升文件；`artifacts/webui-live-BfrW9R/report.json` 同样通过 127/23 项，并独立要求 API 摘要和文件数与浏览器实际文件集一致。此前两次尝试因升级后的 Playwright 浏览器缺失而在产品断言前失败；随后一次正确发现并修复测试工具仍期待 `unknown/unspecified` 的过期假设。这些失败不计为通过证据。未使用远程主机。

本地发布包清单现在通过一个调用内嵌实现的小型 Go 报告工具记录同一身份；裸机派生包会验证并保留该身份，不会重新标记未知 UI。工具／包测试、定向 vet、两份发布脚本解析、契约防回退及差异检查通过；较早的 Queue 观测功能晋升时，4 个文件的身份为 `df27482ff8245e72cefb29418df1816187b3f776058cce19ceae303f7f88b48a`，当前晋升身份见上文。完整制品打包未运行，因为发布打包会正确要求干净、冻结的 server/SDK 组合，而当前开发工作树有意保留未提交变更。自动加载资格清单及签名验证仍待完成。

### 官方部署现已声明 WebUI 部署意图

standalone 与 cluster Compose 清单现分别传入 `RJS_DEPLOYMENT_PROFILE=standalone` 和 `cluster`。Helm management 工作负载根据 Chart 中经过校验的操作者配置产生同一声明（`nats.replicaCount` 为 1 时是 standalone，为 3 或 5 时是 cluster），绝不从可达节点或运行时健康推断。契约测试固定了两项 Compose 值及 Helm 映射；使用固定版本 Helm 的渲染验证了默认 cluster 与单节点 standalone 输出。真实 standalone 与 R3 cluster Compose 浏览器门禁均明确断言设置页能力区域报告预期模式、来源为 `configuration`。原生 Linux Kind 生产 smoke 现在还会认证访问实际运行的 management 服务，并要求 capabilities 响应包含 `cluster` 与 `configuration`，从而发现仅靠模板渲染无法发现的运行时传递故障。本地 Windows 校验不能替代 CI/原生 Linux 对该 Kind 门禁的实际执行。这关闭了 WEB-004 在官方部署清单中的接线缺口，但不代表容量或发布资格验收。

本地三节点裸机监督器现在也会声明 `cluster`，隔离的真实服务浏览器与连接测试工具则声明各自创建的拓扑。监督器环境纯函数测试通过，两份 JavaScript 工具均通过语法校验。更宽的监督生命周期回归中，一个子用例通过，但 calibration 子用例约 44 秒才结束，超过既有 40 秒上下文；该次执行保留为失败证据，没有通过放宽超时伪装为成功。本轮没有接触远程验收进程或文件。

Compose 浏览器运行器现在可以选择任一官方部署清单，并把预期模式传给同一套测试，不维护两套分叉用例。cluster 路径会显式选择三副本 Queue，随后读取规范声明并要求 `document.spec.replicas == 3`，再继续完成修改、重新认证和删除。Chromium 与 Firefox 各自四项 cluster 测试均通过。第一次 R3 Chromium 尝试已成功创建 Queue，但错误地把声明封装当作 Queue 文档读取；该失败执行已按 API 的规范 `document` 字段修正，不计入通过证据。全部 cluster 容器、三个数据卷和网络均正常清理。

该门禁现已接入可重复的工程流程。`make test-admin-ui` 保留为向后兼容的 standalone 入口，`make test-admin-ui-cluster` 覆盖三节点路径，`make test-admin-ui-all` 同时执行两者。CI 新增有时限的组合浏览器任务，Full/Release 本地 RC 会把 standalone 和 cluster 记录为两个独立证据步骤。发布/安全契约测试固定了这三处连接，避免只跑模块测试的绿色任务悄悄替代真实浏览器部署覆盖。完整 contract 包和 PowerShell 解析均通过；当前 Windows 工作站没有 `make` 可执行文件，因此 Make 展开仍需由 Linux CI 提供证据，本地不宣称已执行。

### 当前 React Compose 浏览器门禁与 HTTP Request ID 兼容性

依赖旧版 DOM 的 Compose 测试已替换为当前 React 工作流。针对本机构建的 management 与 NATS 镜像，Chromium 和 Firefox 现在分别验证：认证与导航；经审阅的 Queue 创建、修改和删除；两个变更阶段之间清除会话并重新认证；凭据仅驻留内存及拒绝路径；窄视口下 Node 不可用状态；以及 axe 的严重/致命级结果。最终组合执行 8 项全部通过，并正常清理容器、网络和数据卷。

该门禁在 capability 与 Queue Schema 前置校验均通过后发现了真实部署缺陷：删除流程使用 `crypto.randomUUID()`，而浏览器从 Compose 服务名等非安全 HTTP 主机提供本地控制台时不会暴露该方法，因此 DELETE 根本没有发出。Queue 删除现与 Queue 应用使用相同的密码学随机 128 位十六进制 Request ID 策略，基于 `crypto.getRandomValues()`，且不提供 `Math.random()` 降级。新增定向测试覆盖“存在 `getRandomValues()`、不存在 `randomUUID()`”的环境。当前前端测试为 346 项通过，另有 1 项仅因 Windows 无法创建符号链接夹具而跳过；构建并嵌入的制品为 `index-D3M6gqgW.js`，晋升后的字节一致性已核验。该浏览器门禁不替代剩余架构决策或发布资格验收。

### 已验收 React 候选制品已晋升到 Go 二进制

此前已验收的 `index-B-XT-xMC.js`、`index-DxS1U_tt.css`、`react-vendor-IsLpfBRF.js` 及配套 HTML 现已替换 `admin-ui/dist` 中的旧版文件。内嵌 Handler 测试会解析 HTML、读取其中引用的每个带哈希资源，并验证 SPA 回退及缓存／安全响应头；管理 API 测试不再假设已删除的旧 `/admin/app.js`。两份执行退役旧脚本的测试已删除，没有为了测试保留伪兼容资源；当前 343 项前端模块测试全部通过。`go test ./admin-ui ./management/internal/api` 和定向 vet 均通过。

真实服务测试工具新增明确的内嵌模式：`/admin/` 会代理到最新构建的管理进程，而不是读取候选目录。Chromium `artifacts/webui-live-tS47Es/report.json` 和 Firefox `artifacts/webui-live-bE6IjT/report.json` 均记录 `embeddedUI: true`，使用 `bin/rjs-management-embedded.exe` 通过全部 127 项行为检查；六个二进制／UI 输入在结束时保持不变。每次运行均采集 23 个 axe 状态且零项违规。每个浏览器的两个移动端横向滚动表格快照对被裁切单元格产生严重级别的对比度“无法判定”项，而非违规；逐列截图仍作为人工证据，因此不宣称零 incomplete。

候选晋升现在可复现，不再是无文档的复制操作。`npm run promote` 会验证封闭的 HTML／带哈希 JS／CSS 文件集且拒绝符号链接，完成暂存与哈希检查后仅切换同级 `dist` 目录，失败时回滚，并再次核验结果。`npm run verify:dist` 完全只读，遇到新增、删除或字节变化都会失败。三项定向测试覆盖旧文件替换、漂移、入口引用缺失及链接拒绝路径（只有 Windows 禁止创建测试链接时才跳过该夹具）；完整套件为 345 项通过、1 项环境跳过。Make 目标、CI、发布工作流和本地 RC 验证器现已公开或强制该契约。剩余架构决策和发布资格使完整目标继续保持开放。

### Queue 列表声明部署列已验证

Queue 列表现在展示已有的期望存储类型和请求副本数，不再只有 Queue 标识与 Plan revision。两个表头都明确表示声明／请求语义，不声称实时容量、健康或收敛。只有嵌入 Plan 的 Queue、内容 revision 与外层声明一致，storage 属于受支持值，replicas 为正安全整数时才展示；不一致或旧记录缺少证据时显示“未知”，但不丢弃可导航的 Queue 行。Stream 列表保持不变。

全部 351 项前端测试及生产构建通过，制品为 `index-B-XT-xMC.js`，CSS／React vendor 未变。最初两次 Chromium 和一次 Firefox 运行均已通过新增 Queue 断言，随后发现默认冻结管理二进制缺少源码中已经实现的 routing-probe 路由；安全请求时序证明它在 9ms 返回 404，并非五秒性能停滞。因此从当前源码在本机构建 `bin/rjs-management-queue-list.exe`；恢复管理端自身生成、类型明确的稳定 `DLQ dependency cycle` 消息后，Chromium `artifacts/webui-live-R8TE01/report.json` 与 Firefox `artifacts/webui-live-6i5XxP/report.json` 各通过全部 125 项检查，包括真实 `file` 与 R1 Queue 单元格、23 次 routing-probe 请求观测及六个独立核验的候选输入。

随后相同且未变化的候选通过完整 axe 回归：Chromium `artifacts/webui-live-RHx0TK/report.json` 与 Firefox `artifacts/webui-live-EhZRIi/report.json` 各通过 127 项检查和 23 个可访问性快照，违规与需人工复核项均为 0。该结果取代前述未启用 axe 的证据限制。Queue 列表的观测健康、存储消息数和 Consumer 数聚合仍需有界后端契约；未增加逐行 N+1 读取，也未伪造健康／零值。未改动远程主机或嵌入式发布部署；完整目标继续保留。

### 管理工作流的后端依赖错误泄露已统一关闭

readiness 审查暴露了更广泛的不一致：多数受保护的 JetStream handler 把同一份上游原始错误作为 API 消息返回，旧版 info 与 audit 列表路径还有独立的原始响应。读取、预览、删除预览、Queue Consumer、导出、路由探测、审计列表／窗口／请求及 Queue 写操作的后端失败，现在保留 HTTP 状态、稳定错误码和写操作阶段／影响证据，同时返回按错误码分类的安全消息。日志只保留稳定错误类别与安全操作上下文，绝不记录依赖错误原文。管理端自身生成的校验、前置条件及类型化冲突语义继续保持可操作。

handler 回归测试把包含用户名、模拟密码和内部主机的 NATS URL 注入 readiness、info、cluster、Stream 及全部审计读取变体，响应和服务日志均不得出现这些内容。审计 intent 持久化失败另有日志断言，要求保留 `error_kind=audit_unavailable` 且不包含依赖错误原文。`go test ./management/internal/api ./api ./tests/contract`、`go vet ./management/internal/api`、全部 350 项前端测试及差异检查均通过。前端结果证明恢复行为依赖状态／错误码，而非后端英文文本。未改动浏览器制品、安全策略、远程主机或部署；完整目标继续保留。

### 公开 readiness 错误泄露已关闭

审查发现，未认证的 `GET /readyz` 会返回 JetStream readiness 原始错误。因此，包含 NATS URL、主机或认证细节的依赖错误可能越过公开探针边界。readiness 失败现在只返回 `{"status":"not_ready"}`、HTTP 503 和 `Cache-Control: no-store`；日志只保留稳定错误类别。新增回归测试注入包含用户名和模拟密码的连接错误，解码响应并要求状态是唯一字段。

OpenAPI 的 200/503 响应现在描述精确、封闭的 readiness 信封，不再错误地让 503 客户端引用通用 API 错误信封。使用仓库内 Go 构建缓存后，`go test ./api ./management/internal/api ./tests/contract` 与 `go vet ./management/internal/api` 均通过。完整 `go test ./...` 中其余已报告包均通过，但在当前受限 Windows 环境仍非全绿：`tools/baremetal-run` 无法绑定临时 TCP 端口，`tools/upstreamcheck` 无法读取用户级 Git ignore 文件。两个失败均未执行本次修改的 readiness handler。未改动 WebUI 制品、认证策略、远程主机或部署；完整目标继续保留。

### 登录成功焦点转换已验证

身份验证成功后，登录表单会被移除并创建认证控制台。应用现在只检测“认证控制台从不存在变为存在”的状态转换，并在渲染后聚焦当前 `#console-content` 容器。这避免 Verify 按钮消失后焦点退回 document body。普通路由变化不会重新聚焦，不会覆盖跳过导航控件，到期时也不会移动焦点；已验证的反向转换仍只在 identity 真正清除后把焦点送回 Token 输入。

完整前端测试总数保持 350 项通过。本地候选构建通过，制品为 `index-TuD_GGn7.js`/`index-DxS1U_tt.css`，React vendor 未改变。两套完整浏览器回归均显式启用到期夹具：Chromium `artifacts/webui-live-zbzPRJ/report.json` 和 Firefox `artifacts/webui-live-y0ttH8/report.json` 各通过 127 项检查及 23 个可访问性页面快照。真实断言验证首次认证后的内容焦点、跳过导航的独立键盘激活、普通清除后的焦点恢复，以及到期后确认清除的焦点恢复。套件完成后独立复算全部六个冻结输入哈希，均一致。未改变认证协议、远程主机或内嵌部署；完整目标继续保留。

### 清除 Session 后的焦点恢复已验证

清除已验证 Session 会移除触发操作的控件，并用登录表单替换控制台。应用现在只检测“原先保留 identity、现在 identity 不存在”的状态转换，并在该次渲染后聚焦 Bearer Token 输入。首次打开页面、验证失败以及到期但仍保留 identity/证据时不会自动聚焦。这避免了普通清除或确认丢弃证据后焦点退回 document body。

完整前端测试总数保持 350 项通过。本地候选构建通过，制品为 `index-Qvyhuhiy.js`/`index-DxS1U_tt.css`，React vendor 未改变。两套完整浏览器回归均显式启用到期夹具：Chromium `artifacts/webui-live-7dn1k8/report.json` 和 Firefox `artifacts/webui-live-qExZbE/report.json` 各通过 127 项检查及 23 个可访问性页面快照。真实断言要求从设置页普通清除后，以及 Session 到期后确认清除未知写入证据后，Token 输入均获得焦点；取消清除仍保留证据和 Session。套件完成后独立复算全部六个冻结输入哈希，均一致。未改变服务端凭据撤销行为或远程/内嵌部署；完整目标继续保留。

### Session 过期导航生命周期已修复并验证

Session 到期后会有意保留已验证身份元数据及内存中的变更证据，供审阅/下载，但会卸载认证路由内容。审查发现，新跳过导航控件只依据保留的 identity 显示，因此到期后仍存在，而其 `#console-content` 目标已经消失。现在由一个经过测试的 `hasAuthenticatedConsole` 边界统一控制浏览器标题的路由模式、跳转控件、路由播报及认证内容。过期 Session 仍保留证据界面，但不再暴露无效跳转目标或过期路由播报。

新增一项生命周期测试后，前端共 350 项测试通过。本地候选构建通过，制品为 `index-5XERjzep.js`/`index-DxS1U_tt.css`，React vendor 未改变。两套完整浏览器回归均显式启用合成到期夹具：Chromium `artifacts/webui-live-TukuLq/report.json` 和 Firefox `artifacts/webui-live-5I5aTM/report.json` 各通过 127 项检查及 23 个可访问性页面快照，其中包括未知写入证据保留/下载、不增加 PUT，以及到期后跳转控件、路由播报和认证内容目标同时不存在。套件完成后独立复算全部六个冻结输入哈希，均一致。未改变服务端凭据撤销行为或远程/内嵌部署；完整目标继续保留。

### 不抢焦点的 SPA 路由播报已验证

认证后的路由与语言变化现在会更新一个视觉隐藏、polite、atomic 的状态区域，其文本复用浏览器标题的安全映射。这使辅助技术能够获得页面内导航播报，同时不会把焦点从导航或内容上移走。播报仅包含路由中可见的资源标识，不暴露凭据、操作者、草稿或响应数据。

完整前端测试总数保持 349 项通过。本地候选构建通过，制品为 `index-DsSseM6v.js`/`index-DxS1U_tt.css`，React vendor 未改变。首次完整运行到达删除流程时，发现旧测试错误假设整页只有一个 status 区域；随后将删除结果定位正确限定到 Queue 删除区域，而不是移除合法的路由状态。修复测试后，Chromium `artifacts/webui-live-Ks6Top/report.json` 和 Firefox `artifacts/webui-live-pUToE7/report.json` 各通过 126 项检查及 23 个可访问性页面快照。真实断言要求初始 Queue 列表、导航到设置页及切换中文后，页面标题与播报文本完全一致。套件完成后独立复算全部六个冻结输入哈希，均一致。此前失败报告不用于候选资格结论。未改变远程或内嵌部署；完整读屏器验收和完整目标继续保留。

### 路由词汇本地化已验证

已保存 Queue 路由表、路由探测输入和路由探测结果表现在通过同一份冻结双语词汇映射展示交换机、类型、路由键以及生成/匹配的 Subjects。此前同一个 Exchange 概念在中文页面多个位置仍固定为英文的问题已消除。绑定类型、Subject 语法、请求字段及匹配行为均未改变。

新增一项映射测试后，前端共 349 项测试通过。本地候选构建通过，制品为 `index-kaqMdb_o.js`/`index-DxS1U_tt.css`，React vendor 未改变。Chromium `artifacts/webui-live-WG8uxD/report.json` 和 Firefox `artifacts/webui-live-7Bqe8X/report.json` 各通过 126 项检查及 23 个可访问性页面快照。真实 direct/topic/fanout 夹具会切换为中文，验证本地化交换机输入、结果表全部五个表头，以及已保存表和探测表中两个独立的“交换机”表头；既有移动端键盘横向滚动检查仍通过。套件完成后独立复算全部六个冻结输入哈希，均一致。未改变后端路由行为或远程/内嵌部署；完整目标继续保留。

### Stream 可见字段本地化已验证

Stream 详情工作区在中文界面中不再将保留/丢弃策略、存储字节、Consumer 数、序列号边界及待投递/待确认表头固定显示为英文。现有冻结 Stream 展示映射现在同时管理这些可见标签和可访问区域名称。Stream、Consumer、Subjects 等领域标识继续沿用既定双语产品术语；机器值和 API 字段均未改变。

组合映射测试继续通过，完整前端测试总数保持 348。本地候选构建通过，制品为 `index-BMwTbX8K.js`/`index-DxS1U_tt.css`，React vendor 未改变。Chromium `artifacts/webui-live-LA1aKC/report.json` 和 Firefox `artifacts/webui-live-MZGlqg/report.json` 各通过 126 项检查及 23 个可访问性页面快照。真实中文 Stream 断言会针对已加载的真实服务数据，验证四个 Consumer 表头和全部新本地化的配置/状态标签。套件完成后独立复算全部六个冻结输入哈希，均一致。敏感连接身份/搜索和订阅扩展仍属于架构策略工作，不因本轮本地化而获得批准。未改变远程或内嵌部署；完整目标继续保留。

### 路由感知的双语页面标题已验证

浏览器标题现在随认证/过期状态、当前 SPA 路由、资源标识和界面语言变化。Queue、Stream、Consumer、节点、连接、变更及运维工作区在应用内导航和浏览器历史切换后均可被识别，不再全部显示固定的 `Rabbit JetStream` 标题。纯映射仅包含路由中已有的资源标识；凭据、操作者身份、草稿和响应数据不会进入标题。不支持的路由使用通用“页面不可用”标题，不回显任意值。

新增一项映射测试后，前端共 348 项测试通过。本地候选构建通过，制品为 `index-BC6MeOJ7.js`/`index-DxS1U_tt.css`，React vendor 未改变。Chromium `artifacts/webui-live-FzEa1S/report.json` 和 Firefox `artifacts/webui-live-HFNATE/report.json` 各通过 126 项检查及 23 个可访问性页面快照。真实浏览器断言覆盖认证后的初始 Queue 列表、导航至设置页以及中文切换。套件完成后独立复算全部六个冻结输入哈希，均一致。这不代表完整辅助技术验收。未改变远程或内嵌部署；完整目标继续保留。

### 键盘跳过导航已验证

已认证用户现在可在重复出现的顶栏和主导航之前使用双语“跳到页面内容”控件。激活后，焦点会移动到当前页面内容容器，不改变 hash 路由，也不发出请求。这里特意使用按钮，因为控制台使用 URL hash 路由；普通片段链接会被解释为页面导航。控件未聚焦时位于视口外，获得焦点后显示在固定导航层之上。

第一版候选发现了一项真实的跨浏览器缺陷：程序化或辅助技术焦点不一定满足 `:focus-visible` 启发式条件，因此已聚焦控件仍可能停留在视口上方。显示规则现响应所有焦点，全局焦点轮廓仍使用 `:focus-visible`。最终本地构建通过，制品为 `index-LwVF0yC6.js`/`index-DxS1U_tt.css`，React vendor 未改变。Chromium `artifacts/webui-live-4c8akw/report.json` 和 Firefox `artifacts/webui-live-vgj0vP/report.json` 各通过 126 项检查及 23 个可访问性页面快照。新增检查验证控件可聚焦、聚焦后进入视口、键盘 Enter 激活、内容获得焦点以及 URL 不变。套件完成后独立复算全部六个冻结输入哈希，均一致。此前失败报告不用于候选资格结论。未改变远程或内嵌部署；完整无障碍验收和完整目标继续保留。

### 删除流程可访问本地化已验证

Queue 删除区域和编辑器转删除交接区域不再在可见内容为中文时保留固定英文可访问名称。新增冻结展示映射提供与界面语言一致的名称，同时不改变删除授权、预检、确认、写入或证据语义。真实服务辅助测试进入保留有编辑器的删除流程，切换为中文后通过中文可访问名称定位两个区域及 Queue 相关内容，再切回英文继续原有破坏性操作安全路径。新增一项单元测试后，前端共 347 项测试通过。

本地候选构建通过，业务制品为 `index-onuemjrE.js`，`index-BKGrxozS.css` 与 React vendor 未改变。Chromium `artifacts/webui-live-woL0lQ/report.json` 和 Firefox `artifacts/webui-live-lXyg8a/report.json` 各通过 125 项检查及 23 个可访问性页面快照，并正常退出。套件完成后独立复算全部六个冻结输入的磁盘哈希，均一致。这是限定范围的浏览器语义证据，不代表完整读屏器/WCAG 或生产破坏性操作验收。未改变远程主机或内嵌部署。架构待批工作和完整目标继续保留。

### 总览与兼容性区域可访问本地化已验证

总览、监控摘要、管理服务与账户、已声明 Queue、兼容性、管理进程构建、原生 SDK 契约及服务端能力区域，现通过同一份冻结展示映射提供与界面语言一致的可访问名称。真实服务辅助测试会在页面数据加载后切换为中文，通过中文名称定位这些区域及代表性的真实值；兼容性故障恢复和总览刷新恢复后也会再次验证，随后切回英文继续其余流程。机器契约、刷新行为和授权语义均未改变。新增一项单元测试后，前端共 346 项测试通过。

本地候选构建通过，业务制品为 `index-B3hm9LbT.js`，`index-BKGrxozS.css` 与 React vendor 未改变。Chromium `artifacts/webui-live-zqsFCX/report.json` 和 Firefox `artifacts/webui-live-s15zLE/report.json` 各通过 125 项检查及 23 个可访问性页面快照，并正常退出。套件完成后独立复算全部六个冻结输入的磁盘哈希，均一致。新增证据属于浏览器语义覆盖，不是专用中文截图，也不代表完整读屏器/WCAG 验收。未改变远程主机或内嵌部署。仍需负责人批准的全局 Consumer 索引、诊断任务及完整目标继续保留。

### Stream 与 Queue 摘要可访问本地化已验证

Stream 详情、观测 Consumer 表及分页现提供与语言一致的可访问名称；Queue 摘要中的主 Consumer 指标组和 Consumer 排查入口亦同步处理。冻结展示映射将机器查询／状态契约与文案分离。真实服务辅助测试在真实三行 Stream Consumer 集合上切换中文，通过中文名称定位详情／表格／分页，再切回英文继续验证分页、筛选及历史路径。新增一项组合单元测试，前端共 345 项通过。

候选构建通过，产物为 `index-CXeA_sUd.js`，`index-BKGrxozS.css` 和 React vendor 未变。固定中文夹具 `artifacts/webui-selected-YA8UOf/report.json` 通过，并强制要求中文 Queue 摘要指标及排查入口名称。Chromium `artifacts/webui-live-5YcPh2/report.json` 和 Firefox `artifacts/webui-live-RMjDML/report.json` 各通过 125 项并正常退出。测试核验后，全部六个冻结输入的独立磁盘哈希一致。这证明限定范围的语言切换及现有流程，不代表完整读屏／WCAG 验收或全局 Consumer 索引。未改变远程或内嵌部署，完整目标仍待完成。

### 节点工作区可访问本地化已验证

节点、节点集合及 JetStream 指标现通过不可变展示映射提供与语言一致的可访问名称。真实服务测试将真实集合切换为中文，通过中文集合名称定位节点；进入详情后，通过中文节点区域及 JetStream 指标定位内容，再切回英文。节点身份、监控读取、失败语义和健康边界说明未改变。新增一项单元测试，前端共 344 项通过。

候选构建通过，产物为 `index-03IZW5s6.js`，`index-BKGrxozS.css` 和 React vendor 未变。Chromium `artifacts/webui-live-o0kmdw/report.json` 和 Firefox `artifacts/webui-live-5t0JO7/report.json` 各通过 125 项并正常退出。测试核验后，全部六个冻结输入的独立磁盘哈希一致。已查看 Chromium 移动详情截图：长 Node ID、来源和指标未造成页面溢出。中文名称属于浏览器断言，不是专用截图或完整读屏验收。未改变远程或内嵌部署。其余固定英文区域及需架构批准的工作使完整目标仍保持未完成。

### 审计工作区可访问本地化已验证

审计事件、导出和结果窗口区域现通过冻结的展示映射提供与语言一致的可访问名称。真实服务辅助测试将已有真实筛选结果切换为中文，通过中文名称定位三个区域，再切回英文并确认结果仍保留。查询、授权、导出字节及审计语义未改变。新增一项单元测试，前端共 343 项通过。

候选构建通过，产物为 `index-CVnIQDPq.js`，`index-BKGrxozS.css` 和 React vendor 未变。Chromium `artifacts/webui-live-Ah7K7s/report.json` 和 Firefox `artifacts/webui-live-tRKBjr/report.json` 各通过 125 项并正常退出。测试核验后，全部六个冻结输入的独立磁盘哈希一致。新增断言属于语义浏览器证据；本轮未增加中文审计截图，也不代表完整读屏验收。未改变远程或内嵌部署。其他固定英文区域仍在复查；需架构批准的功能及完整目标仍待完成。

### 连接页可访问本地化已与 CSS 解耦

系统性复查在连接列表、数据表、分页及详情样式中发现相同的英文标签耦合。现增加与语言一致的可访问名称及稳定结构类；CSS 和逐列对比度选择器不再依赖翻译文本。真实服务测试必须使用中文可访问名称重新定位中文列表、数据行、分页和详情，不能继续复用英文定位器。新增一项纯标签测试，前端共 342 项测试通过；源码检查确认 CSS 已无针对 `aria-label` 的属性选择器。

候选构建通过，产物为 `index-DyvzkjER.js`／`index-BKGrxozS.css`，React vendor 未变。Chromium `artifacts/webui-live-dQKIBb/report.json` 和 Firefox `artifacts/webui-live-HYaGs7/report.json` 各通过 125 项并正常退出。测试核验后，全部六个冻结输入的独立磁盘哈希一致。已查看 Chromium 移动截图：表格保持内部滚动，页面无横向溢出；测试会逐个滚动数值列，使其实际接受对比度检查。中文可访问名称有断言，但本轮未新增中文连接截图，因此视觉证据仍是英文移动截图。未改变远程或内嵌部署。这是限定范围的 WEB-016 进展，不代表完整无障碍验收；需架构批准的功能及完整目标仍待完成。

### 副本本地化及语言无关移动布局已验证

共享副本组件现根据语言显示区块／滚动区域的可访问名称及 Leader／Follower 文案，证据模型仍保留机器角色值。使用语言无关的 `.replica-observations` 类替代依赖英文 `aria-label` 的 CSS。新增一项纯标签测试，前端共 341 项测试通过。固定中文夹具 `artifacts/webui-selected-gMKU5E/report.json` 针对 `index-CqfsHcjR.js`／`index-DO-tqaBV.css` 通过，移动截图已复核。

首个语义修复候选（`index-D80__2tR.js`）虽在两浏览器各通过 125 项，但视觉复核发现：翻译可访问名称后，依赖英文标签的 CSS 失效，六列被强行压缩。相关报告（`webui-live-UMMvGP`、`webui-live-Oom0dr`）保留，但不作为最终移动布局资格证据。固定夹具 `webui-selected-bThxN8` 的首次断言也因在桌面视口要求必须溢出而失败；现将断言移至移动视口，没有削弱要求。

修复 CSS 并增加移动端内部溢出和角色单元格不压窄断言后，Chromium `artifacts/webui-live-H6Vvpo/report.json` 和 Firefox `artifacts/webui-live-RehSGp/report.json` 各通过 125 项并正常退出。测试核验后，全部六个冻结输入的独立磁盘哈希一致。中文完整面板截图显示身份／角色列可读；Firefox 右侧列截图显示通过原生键盘滚动后，最后活动值完整可见。这是限定范围证据，不代表全部 WCAG 路径完成。未提升内嵌版本或操作远程。诊断任务／全局 Consumer 索引仍待架构选择，完整目标尚未完成。

### 诊断任务设计及不覆盖竞态修复

新增[诊断任务提案](webui-diagnostic-jobs.zh-CN.md)，依据现有身份、审计持久化、处理器超时和应用退出机制，区分建议的实例内存／operator／仅创建者策略与尚未实现的 API、预算、生命周期、下载审计和验证门槛。引入任务存储／访问模型前须负责人批准，CLI 可用不等于浏览器接口已获批准。

检查另复现 CLI 发布竞态：新增测试在采集期间创建输出，最终 Rename 覆盖它，测试明确失败。现改为同目录硬链接发布，目标存在即失败，清理临时文件且无覆盖式降级。输出文件系统须支持硬链接。该回归及全部 `tools/rjsctl`／`internal/redact` 测试通过，定向 vet 和差异检查通过。此次 Windows 测试不是原生 Linux 验收。未改变冻结二进制、候选 UI 或远程主机，完整 WebUI／后端工作仍待完成。

### 诊断 JSON 安全基础

检查 WEB-024 的复用基础时发现，CLI JSON 脱敏使用浮点数解码，并在输入畸形时原样返回。现提取可复用的 `internal/redact.JSON`：保留数字原始精度，递归脱敏敏感字段及绝对 URL 用户信息，非法输入不返回正文。CLI 对已知 JSON 来源即使 Content-Type 错误也执行验证；畸形来源不归档，仅在清单保留 HTTP 状态及静态错误，其余来源继续采集。新增测试检查实际 ZIP 中的省略行为、秘密夹具值不出现、uint64／大修订号精确保留；共享测试覆盖 int64 边界、小数、指数、畸形／尾随输入及输入不变。

因本地 Go 缓存／Git 配置权限失败获准重试后，完整 `go test ./...`、定向 `go vet ./internal/redact ./tools/rjsctl`、格式及差异检查通过。最初受权限限制的全量运行不计为通过。更新[诊断文档](diagnostics.zh-CN.md)并补齐对应中文版，明确仅采集首个 200 项页面及脱敏局限。未修改前端候选、冻结二进制或远程任务。这是共享安全前置能力，不是浏览器下载接口；有界任务／存储生命周期、过期、访问策略和审计仍未实现。全局 Consumer 架构批准及完整目标仍待完成。

### Queue 模板浏览器证据已完成

Chromium `artifacts/webui-live-Bp1Utf/report.json` 和 Firefox `artifacts/webui-live-PIRiG8/report.json` 各通过 125 项检查并成功退出，保持 `bin/rjs-management-consumer-diagnosis.exe`、`index-DI4f3uuo.js`、vendor／CSS 和 HTML 不变。测试核验后，全部六个输入的独立磁盘哈希一致。两张中文模板截图已查看，控件、说明及换行精确 JSON 可读，页面无横向溢出。新增检查证明本地审阅、切换字段隔离、明确预览／确认、单次仅创建请求、保存的优先级配置、不隐式创建 DLQ 目标及模板重置。不证明优先级投递顺序、DLQ 转移、新表单完整键盘覆盖或任意批量更新。参见[模板范围](webui-queue-templates.zh-CN.md)。本轮未重建候选、部署内嵌版本或操作远程。完整目标仍待完成，包括全局 Consumer 架构批准及节点／历史整合。

### 本地声明模板已实现，浏览器验收进行中

在已有仅创建流程中增加[三种可审阅 Queue 模板](webui-queue-templates.zh-CN.md)。仍要求明确部署输入、精确整数、Schema 就绪、服务端预览和独立确认。切换模板不会把未选中的优先级／DLQ 字段带入文档，也不会隐式创建 DLQ 目标。340 项前端测试、辅助测试语法及差异检查通过。候选构建在 Vite 临时目录权限获准重试后通过：`index-DI4f3uuo.js`，vendor／CSS 未变。两套真实服务浏览器测试已启动，后端保持 `bin/rjs-management-consumer-diagnosis.exe`，最终证据待完成。未改变远程／内嵌部署。全局 Consumer 索引仍待负责人批准架构，完整目标尚未完成。

### Consumer 副本键盘验证已完成

Chromium `artifacts/webui-live-20P3Jl/report.json` 和 Firefox `artifacts/webui-live-VmzuCP/report.json` 各通过 124 项检查，两次进程退出码均为 0。测试验证原生 Tab 进入、ArrowRight 滚动到最右侧活动指标、单元格精确文本与可见性，以及 Shift+Tab 退出。两张专用中文截图均已查看：最右侧活动值完整可见，前面的列可能在横向滚动表格内部被裁切。这不代表所有列同时容纳，也不代表所有无障碍路径均已验收。

除测试自身的运行前后核验外，完成后独立核对全部六个冻结输入的磁盘哈希，均一致。输入保持为 `bin/rjs-management-consumer-diagnosis.exe`、NATS、`index-BUgw-oru.js`、vendor、CSS 及 HTML。未重建候选、提升内嵌版本或操作远程。副本条件仍为合成数据，不是真实集群故障验收。节点／历史整合、全局 Consumer 索引及完整目标仍待完成；索引架构仍需负责人决定。

### Consumer 副本键盘验证已启动

Consumer 副本真实服务辅助测试新增原生键盘进入、ArrowRight 横向滚动、最后单元格精确文本／可视区域检查及 Shift+Tab 退出，单独保存右侧列中文截图。语法和差异检查通过。两套完整浏览器测试已启动，保持 `index-BUgw-oru.js`／vendor／CSS 及 `bin/rjs-management-consumer-diagnosis.exe` 不变；最终报告及视觉复核待完成。未重建应用或候选，未操作远程或提升内嵌发行文件。此前 123 项证据不包含此新增键盘路径，完整目标仍待完成。

### Consumer 副本浏览器证据

Chromium `artifacts/webui-live-KoeEPr/report.json` 和 Firefox `artifacts/webui-live-XArN1f/report.json` 各通过 123 项检查，使用 `bin/rjs-management-consumer-diagnosis.exe`、`index-BUgw-oru.js` 及未变的 React vendor／CSS。两次进程均成功退出，完整候选运行前后核验及独立文件哈希一致。新增合成检查覆盖离线／未同步／大整数落后提示、重复身份拒绝且不展示部分拓扑／提示，以及已有的过期／读取恢复路径。

已查看两套中文移动端诊断截图。计数、指引和可见的左侧副本身份可读，页面无横向溢出。副本表内部横向滚动，现有截图未展示最右侧指标列；显式键盘／右侧列验证仍待完成。这是合成观测处理，不是真实集群故障、法定人数或节点健康测试。本轮未改变应用代码、候选资源、远程任务或内嵌发行文件。其余 WebUI／后端需求及全局 Consumer 索引的负责人决定仍待完成。

### Consumer 副本验收进行中

两套真实服务浏览器测试已启动，使用未变的 `bin/rjs-management-consumer-diagnosis.exe`、`index-BUgw-oru.js`、React vendor 和 CSS。现覆盖明确标记的合成离线／未同步／落后提示、重复拓扑抑制及过期／读取恢复。最终报告及移动端截图复核待完成，不宣称真实集群故障验收。

新增后端映射测试，证明 Consumer Leader／成员身份、离线／同步标志、最大 uint64 落后数及 int64 活动时间在转换中保留，不引用可变 broker 输入；缺失拓扑仍保持缺失。JetStream 测试及差异检查通过。此新增仅涉及测试，不改变正在浏览器验收的可执行文件。未改变远程或内嵌部署。完整目标与全局索引架构决定仍未完成。

### Consumer 副本证据整合

Consumer 积压诊断现复用共享副本解析／表格，并增加关联具体成员的建议性提示：未报告 Leader、Follower 离线／未同步及正数落后量。配置数量与成员覆盖保持未知，不推断 Stream 或节点健康。缺少拓扑不是健康；畸形／重复身份抑制副本提示，旧读取清除当前副本区域。审查还补齐共享落后数／活动时间格式化器缺少的 uint64／int64 上界，同样保护已有 Stream 证据展示。

全部 335 项前端测试通过，覆盖副本精确边界、身份失败、缺少拓扑与过期清除。候选构建通过（`index-BUgw-oru.js`，vendor／CSS 不变），无体积警告；辅助脚本语法／差异检查通过。真实服务辅助测试现包含明确标记的合成副本计数和重复拓扑恢复，但尚未重跑。此前 122 项报告不验收此版本。未改变后端可执行文件、远程任务或内嵌发行文件。节点健康／历史整合及更大范围目标仍未完成，全局索引架构仍待负责人批准。

### Consumer 诊断浏览器证据

Chromium `artifacts/webui-live-nVA7Ed/report.json` 和 Firefox `artifacts/webui-live-fEnK5l/report.json` 各通过 122 项检查，使用本机构建的 `bin/rjs-management-consumer-diagnosis.exe`、`index-CZjh3G2i.js`、`react-vendor-IsLpfBRF.js` 及不变的 CSS。两个测试进程均成功退出。完整候选文件清单／内容核验及独立磁盘哈希一致。已查看两套中文移动端诊断截图：精确大整数、有限上限解释及非根因指引可读，没有横向溢出。

新增检查验证真实 broker 默认 ACK 上限，再测试合成阈值／Pull／重投计数、清除过期诊断但保留原观测时间、畸形上限处理及仅 GET 的真实读取恢复。截图及饱和计数仍明确为合成，不证明真实负载饱和或吞吐。本轮未改变产品代码、候选输入、远程任务或内嵌发行文件。WEB-018 仍需整合节点／副本及历史证据；完整无障碍、规模与总体目标尚未完成。全局 Consumer 索引架构仍待负责人确认。

### Consumer 诊断真实服务测试框架

已在本机构建 `bin/rjs-management-consumer-diagnosis.exe`，完成时带非致命模块缓存写入警告。经批准访问本机缓存后，完整 `go test ./...` 通过，辅助脚本语法／差异检查通过。候选保持 `index-CZjh3G2i.js`，React vendor／CSS 不变。两套完整浏览器测试已启动，最终报告和截图复核待完成。

新增辅助测试先读取自有 broker 的真实 Consumer，核对固定 broker 的默认 `max_ack_pending=1000`。随后用合成响应计数检查精确 uint64／int64 展示、ACK 阈值／Pull／重投提示、中文移动端布局、503 清除当前提示／事实但保留历史观测时间、畸形上限处理及真实读取恢复，要求 Consumer 请求全部为 GET。饱和计数明确为合成，不是实际负载或消息投递结果。真实服务测试文档已改用新可执行文件。未改变远程或内嵌部署；完整 WEB-018 和总体目标仍未完成。

### Consumer 积压提示与观测 ACK 上限

Consumer 响应及 OpenAPI 增加只读 broker 配置 `max_ack_pending`，详情页复用已有精确读取提供中英文提示。正数积压、达到正数 ACK 上限、有待投递但无等待 Pull、报告重投等产生有边界的排查指引，不判定健康／根因或客户端断开。过期／失败／在途／未来观测不生成当前提示。旧服务缺少字段及畸形／已舍入整数不当作零值。见[范围与待办](webui-consumer-diagnosis.zh-CN.md)。

全部 332 项前端测试、针对性 JetStream／API／契约测试及 Go vet 通过。验证发现并修正了 Schema 必填不一致和 JSX 闭合标签错误。最终本机候选构建通过（`index-CZjh3G2i.js`，React vendor／CSS 不变），无包体积警告。本次尚未构建新后端可执行文件或完成浏览器验收，之前的分包候选报告不覆盖新增功能。真实 broker 上限证据、界面刷新／失败／移动端检查及结合节点／历史的完整 WEB-018 仍待完成。未改变远程或内嵌部署；全局索引架构决定仍待确认。

### 分包候选真实服务验收

Chromium `artifacts/webui-live-nifjny/report.json` 和 Firefox `artifacts/webui-live-TqdbEH/report.json` 各通过 121 项检查，使用 `bin/rjs-management-dlq-errors.exe`、`index-KBlTTNsa.js`、`react-vendor-IsLpfBRF.js` 及不变的 CSS。两次进程均成功退出。报告完成候选文件全集及运行前后指纹检查，独立磁盘哈希与全部六项输入一致（两个可执行文件、HTML、CSS、两个 JS chunk）。这证明所测流程能在分包候选上加载和运行，不代表性能或发布资格验收。

模拟捕获 `artifacts/webui-selected-TQuqJ1/report.json` 也通过，包含新增初始 HTML 一致性断言及捕获后的完整文件校验，其 HTML 和所有记录资源的独立哈希均与磁盘一致。这完成下方此前待重跑项。运行中未改变候选文件，未改变远程或内嵌部署。全局 Consumer 索引仍等待负责人架构决定。其余功能、完整无障碍、规模／性能实测和发布替换尚未完成。

### 框架分包与完整候选指纹

候选校验现对目录内所有普通文件计算指纹，包括 HTML 未直接引用的 chunk，并在真实／模拟捕获后比较文件清单与内容；拒绝符号链接。新增两项测试覆盖未引用／嵌套资源、同长度内容变化、文件增删、缺少首页及目录链接。全部 328 项前端测试通过。这是前后完整性检查，不是文件系统原子快照，也不能防止运行中修改后恢复。

React／react-dom／scheduler 现组成 `react-vendor-IsLpfBRF.js`（193.81 kB），与业务 `index-KBlTTNsa.js`（316.44 kB）分离，CSS 不变。本机构建通过，原先超过 500 kB 的单包警告消失，未提高警告阈值。这分离框架与业务更新的缓存，不代表已测得首屏延迟或总传输量降低。模拟捕获 `artifacts/webui-selected-WNIz4I/report.json` 通过且完成全部资源校验；随后补充的首页一致性断言仍需重跑捕获。完整 Chromium／Firefox 真实服务回归已启动，后端保持 `bin/rjs-management-dlq-errors.exe` 不变，最终报告待核验。未改变远程或内嵌发行文件。全局 Consumer 采集模型仍未批准，与本轮工作独立。

### 双语诊断浏览器验收与下一项架构确认

Chromium `artifacts/webui-live-BwDLaS/report.json` 和 Firefox `artifacts/webui-live-KUm6WW/report.json` 各通过 121 项检查，使用 `bin/rjs-management-dlq-errors.exe`、`index-DpCZS5za.js` 和 `index-Bcxqs_G2.css`。两份报告均记录运行前后输入核验，独立磁盘哈希与两份报告一致。Chromium 在固定新上下文语言后通过。已查看两套中文移动端循环截图：草稿／ETag 保留、修正指引及无提交按钮状态可读，没有横向溢出。此前上下文／语言失败记录保留；通过不代表完整无障碍、未知写入判定或发布就绪。

已对照当前 handler 和枚举实现核实下一项未实现需求 WEB-021：现有集合仅限单 Queue／Stream，不是全局检索。[全局 Consumer 提案](webui-global-consumers.zh-CN.md) 比较实例内完整代次内存索引与按次枚举，保留 10,000 Queue／100,000 Consumer 目标，定义缺失／外部身份、部分失败和代次安全分页规则。已请求负责人确认采集模型；依 feature-dev 架构检查点，在确认前暂停索引实现，不宣称已实现 API／导航。其他剩余工作及完整目标仍未完成。未改变远程或内嵌发行文件。

### 循环诊断 Firefox 证据与 Chromium 语言复测

Firefox `artifacts/webui-live-KUm6WW/report.json` 通过 121 项检查，使用 `bin/rjs-management-dlq-errors.exe`、`index-DpCZS5za.js` 及不变的 CSS。运行前后指纹与独立磁盘哈希一致。已查看中文移动端循环诊断截图：保留的 JSON、原始 ETag、修正指引及不可提交状态可读，页面无横向溢出。这验证所测预览流程，不代表未知写入权威判定或完整无障碍。

Chromium `artifacts/webui-live-UlaZ2T/report.json` 在新上下文等待英文 Bearer token 输入框时失败。检查发现该上下文漏设主测试显式使用的 `en-US`，而应用启动语言取自浏览器。现已固定相同语言及桌面视口；这修正了明确的测试环境不一致，但未捕获超时时的精确页面状态。保留失败记录，已在候选输入不变的情况下仅重启 Chromium。语法／差异检查通过；Chromium 验收仍待完成。未改变远程或内嵌部署。

### 浏览器测试上下文修正

首次诊断回归（Chromium：`artifacts/webui-live-lGVD5S/report.json`；Firefox：`artifacts/webui-live-iyb3nu/report.json`）均在记录 116 项检查后报错 `Please use browser.newContext()`。新增辅助测试尝试在 Playwright 单页面快捷 API 所有的上下文中再开一页，错误发生于循环诊断页面加载前；两次均不算完整通过。现改为创建并关闭独立上下文，不改变候选资源或应用行为。语法／差异检查通过，两次重试已启动，结果仍待核验。

已查看两次失败运行保存的中文移动端批次历史截图：接受状态翻译、请求标识可读，无横向溢出。这仅是有限视觉证据，不证明尚未执行的循环界面检查通过。完整 `go vet ./...` 通过，未改变远程或内嵌发行文件。

### 可操作的循环预览诊断

Queue 编辑器现通过有界预览诊断展示中英文 DLQ 循环说明：检查引用声明、修正依赖链后重新预览，不把循环误当网络故障。明确循环可能存在于下游，不一定返回当前 Queue。展示条件要求 preview-error、无已发送请求标识、HTTP 400，且错误和正文的 `dlq_dependency_cycle` 一致。不为循环错误猜测字段路径；未知写入不会获得仅适用于预览的恢复提示。

全部 326 项前端测试、候选构建（`index-DpCZS5za.js`，CSS 不变）、辅助脚本语法及差异检查通过。包体积警告仍存在。真实服务辅助测试现核对英文归档标签且保留原始归档状态；新增独立浏览器页验证真实循环预览、中英文诊断、可编辑草稿保留、无提交按钮、一次 POST／零 PUT 及声明／ETag 不变，并保存中文移动端截图。两套完整浏览器运行已启动，最终结果及截图复核待完成。未改变后端可执行文件、远程环境或内嵌发行文件。

### 循环错误契约已验收；翻译候选已构建

Chromium `artifacts/webui-live-zSosDI/report.json` 和 Firefox `artifacts/webui-live-foWY6B/report.json` 各通过 120 项真实服务检查，使用 `bin/rjs-management-dlq-errors.exe`、`index-C-lPqgXc.js` 和 `index-Bcxqs_G2.css`。两份报告记录运行前后输入核验，重建前独立磁盘哈希一致，两次测试进程均成功退出。循环样例现验证预览与条件 PUT 返回 HTTP 400 `dlq_dependency_cycle`，原声明／ETag 不变。此前中断目录仍属于不完整证据。

两次运行结束后，已构建翻译候选 `index-DSp3oWxg.js`，CSS 不变。经批准访问 Vite 临时目录后构建通过；超过 500 kB 的包体积警告仍未解决。补上此前漏掉的 `uneditable` 标签后，针对性状态测试通过。本次浏览器报告验收的是上一版前端资源与新后端，不包含新构建翻译。批次历史翻译的视觉验证、完整无障碍及更大范围 WebUI／后端目标仍待完成。未改变远程或内嵌部署。

### 批次状态翻译与中断回归恢复

执行页和归档历史现共用中英文状态标签，包含读取中／读取失败等状态。“已接受”仅指提交已接受，不代表资源健康。未来未知状态明确显示未知；归档 JSON 的原始机器状态码不变。全部 325 项前端测试通过，包括标签覆盖、未知键及证据保留检查。尚未重建候选资源，标签的浏览器视觉验证仍待完成。

此前 `dlq-errors` 浏览器运行 `artifacts/webui-live-YSn16K` 和 `artifacts/webui-live-2fvGXf` 在中断后没有最终报告。原句柄已缺失，经批准查询进程确认对应测试 Node、管理服务及 NATS 均未存活。保留这些目录作为不完整证据，不计通过。新的隔离运行使用相同 `bin/rjs-management-dlq-errors.exe` 及未改动候选资源。上一轮完整 Go 回归已通过；新浏览器运行仍须核对最终报告和哈希。未改变远程环境或内嵌发行文件。

### DLQ 循环错误分类

已观测到的 DLQ 循环现通过可包装后端错误标识映射为 HTTP 400 `dlq_dependency_cycle`，不再误报 `503 jetstream_unavailable`。其他依赖错误保留原分类。预览与写入证据测试覆盖映射，链路测试要求非循环错误不得误分类。新增编辑器回归验证循环预览后保留草稿／ETag、禁止提交／冲突合并；已发送写入的拒绝仍保持未知，禁止重发。

全部 323 项前端测试、JetStream／API／契约测试及针对性 Go vet 通过（Go 检查首次权限失败，经批准访问缓存后通过）。未重建可执行文件或候选资源，未改变远程环境。真实服务辅助测试现要求新状态码／错误码；此前 120 项报告覆盖旧 503 契约，不验收本次修改。下一次浏览器运行前须重建后端。完整 WebUI／后端目标仍待完成。

### 传递 DLQ 真实服务回归

Chromium `artifacts/webui-live-I9mNs3/report.json` 和 Firefox `artifacts/webui-live-okN15H/report.json` 各通过 120 项检查，使用本机构建的 `bin/rjs-management-dlq-chain.exe`，前端保持 `index-C-lPqgXc.js` 和 `index-Bcxqs_G2.css` 不变。运行前后指纹与独立磁盘哈希一致。新增样例尝试闭合三 Queue DLQ 循环：预览和条件 PUT 均拒绝，原声明与 ETag 保持不变。未知写入检查现要求声明／Consumer／审计各来源明确返回 `available`，避免子串匹配误将 `unavailable` 视为通过。

完整 `go test ./...` 经批准重试后通过。首次因 `upstreamcheck` 无权读取 Git 全局忽略配置而失败，未修改子树绕过该失败。本机构建完成，伴随非致命模块缓存写入警告；辅助脚本语法检查通过。未改变远程任务或内嵌发行文件。这验收了所测循环与未知结果约束路径，不代表跨 Queue 并发原子性、未知写入权威判定、完整无障碍、原生发布资格或整个 WebUI／后端目标已完成。

### 有界传递 DLQ 循环检查

预览／提交的依赖验证现遍历已保存目标链，拒绝返回拟提交来源项、重复目标、无效／缺失声明及超过 100 项的链路。遍历、直接目标 Stream 观测及全链 KV 修订复查共享五秒预算。不修复、不发布消息、不读取第 101 个目标，不声称下游健康或跨 Queue 原子性。观测之后的并发变化仍不在快照保证范围内。

新增三项测试，覆盖有效链／传递循环／既存循环、缺失／无法还原声明、取消、精确 100／101 边界和传递节点修订变化。JetStream／API／拓扑／契约测试通过，静态检查经批准访问缓存后通过，差异检查通过。未改变可执行文件、前端候选、远程主机或内嵌发行文件。现有 119 项浏览器证据早于本次源码修改；原生／浏览器验收及完整目标的其余工作仍待完成。

### 批次未知写入约束浏览器证据

Chromium `artifacts/webui-live-SSLPOb/report.json` 和 Firefox `artifacts/webui-live-UpHOyw/report.json` 各通过 119 项真实服务检查，使用未变的 `bin/rjs-management-external-review.exe`、`index-C-lPqgXc.js` 和 `index-Bcxqs_G2.css`。运行前后指纹及独立磁盘哈希一致，测试专属服务正常退出。辅助脚本语法与差异检查通过。本轮未改变应用代码、候选资源、远程主机或内嵌发行文件。

新增测试实际转发一次成功创建，以畸形 JSON 替换响应，并确认根项已存在而依赖方仍缺失。只读检查和站内导航后，浏览器仍保留未知状态及原请求标识，不重放、不创建依赖项、不归档。这验证未知结果约束，不是权威判定：资源存在／当前审计观测仍不能将成功归因于原始尝试，明确结果判定仍未提供。恢复流程完成、其他故障模式、完整无障碍及更大范围目标仍待推进。

### 外部依赖真实服务浏览器证据

Chromium `artifacts/webui-live-N1XyPV/report.json` 和 Firefox `artifacts/webui-live-judcYp/report.json` 各通过 118 项检查，使用本机构建的 `bin/rjs-management-external-review.exe`、`index-C-lPqgXc.js` 和 `index-Bcxqs_G2.css`。两份报告均核对运行前后候选输入，独立哈希亦一致。后端构建出现非致命模块缓存写入警告但成功完成。相关拓扑／API／JetStream／契约测试及差异检查通过，测试专属服务正常退出。未改变远程或内嵌部署。

新增真实服务路径对以缺失外部 Queue 为根的依赖链保持内部顺序为空、预览顺序前置项优先。目标缺失时预览失败且无 PUT。测试显式准备仅补齐该目标；重新预览和独立确认后，按前置顺序恰好产生两次条件创建，保持已保存精确 int64。传递依赖方须等待前置请求接受。它证明所测缺失到恢复流程，不代表外部漂移穷尽验证、未知写入恢复、消息投递、完整无障碍或发布就绪。完整 WebUI／后端目标仍待完成。

### 外部前置依赖前端预览接入

前端现独立校验和展示 `review_order`，保留原有问题／就绪状态。校验拒绝重复／越界索引、依赖倒序、遗漏可预览项和结构性错误。旧响应缺少该字段时仍仅允许内部顺序项，包括再次投影到执行控制器后。批次执行可准备外部根依赖项进行当前服务端预览；批次内前置项仍须已有接受请求。继续复用仅创建／独立确认／未知写入保护，任何外部观测状态都不授权写入。

新增三项测试覆盖当前预览／确认、四种外部状态预览失败且无 PUT、畸形预览顺序及旧版回退。全部 322 项前端测试和候选构建通过（`index-C-lPqgXc.js`、`index-Bcxqs_G2.css`），构建使用已批准的本机 Vite 临时目录访问。块体积警告仍待处理。未改变后端可执行文件或远程环境。此前 116 项报告不覆盖本次新增功能；真实浏览器外部依赖验收前须重建包含 `review_order` 的后端。完整 WebUI／后端工作仍未完成。

### 外部依赖预览排序契约

纯规划器／API 新增独立于 `order` 和 `ready` 的 `review_order`。允许外部前置依赖未验证的项及其批次内依赖方进入后续显式目标端预览，不移除任何问题码，也不声称外部依赖就绪。无效／重复／循环项及其传递依赖方仍被排除。两类顺序共用确定的依赖优先计算，纯规划器不增加读写。存在／缺失观测不改变预览排序。

新增两项拓扑测试，覆盖混合结构问题、输入排列、问题保留、100 项外部根依赖链和空数组输出；扩展原有内部排序及 API 证据测试。拓扑、API、JetStream 和契约测试经批准访问 Go 缓存后通过，静态／差异检查通过。OpenAPI 和中英文迁移文档已反映新增字段。前端消费、外部目标最新预览／提交及浏览器证据仍待完成，未生成新二进制／候选资源或执行远程操作。

### 批次归档浏览器证据

Chromium `artifacts/webui-live-eWr9Lz/report.json` 和 Firefox `artifacts/webui-live-naa6FB/report.json` 各通过 116 项真实服务检查，使用 `bin/rjs-management-batch-execution.exe`、`index-Bn1EzD3i.js` 和 `index-Bcxqs_G2.css`。报告核对运行前后输入，独立哈希亦与当前文件一致。两份中文移动端历史截图已检查，页面无横向溢出；`accepted` 等原始阶段标签仍可改善本地化。测试专属服务正常退出。全部 319 项前端测试、辅助脚本语法及差异检查通过，未改变生产／远程／内嵌部署。

新增覆盖取消后确认归档且无 Queue API 调用、解析归档精确整数／请求标识、导航后恢复历史、启动第二批次、不新增 PUT 地拒绝此前保留名称、归档未提交项及逐字比较首份归档。它证明所测归档流程，不代表会话过期展示、未知写入恢复、外部依赖执行、完整无障碍或发布验收。完整目标仍未完成。

### 归档批次命令撤销

专项复核发现旧批次视图原先只撤销预览／提交，归档后的旧引用仍可调用本地编辑、表单读取及 `discard`；后者可能清除共享 API 凭据。现归档后撤销全部转发命令，仅保留快照／订阅观测。全局登记中的原控制器仍保留正常会话清理和删除交接权限。新增回归逐一调用旧视图全部命令，确认无请求、无凭据清除、无草稿改变；已有归档测试也增加编辑无效断言。

全部 319 项前端测试、候选构建（`index-Bn1EzD3i.js`、`index-Bcxqs_G2.css`）及差异检查通过。构建使用已批准的本机临时目录访问，仍有超过 500 kB 的块体积警告。这是源码／模型验证，不是新增归档浏览器证据。未改变后端、远程负载或内嵌发行文件。归档浏览器验证及完整目标的其余范围仍待完成。

### 批次归档与新活动批次（源码已验证）

增加显式确认归档、通过复制保留的文件／规划／结果证据，以及创建页和会话过期页的只读历史。归档预检全部已准备控制器，拒绝进行中／未知请求，禁用旧批次发送并清除当前批准，同时保留历史快照。不写入、不撤销资源、不释放保留名称，原有全局删除交接仍可使用。随后移除当前活动批次，可规划另一批次；历史继续受会话清除／页面离开警告保护。

新增两项测试及进行中／未知结果归档断言通过，全部 318 项前端测试通过。Vite 临时目录经批准重试后候选构建通过（`index-uPm7hfgq.js`、`index-Bcxqs_G2.css`）；超过 500 kB 的块体积警告仍待处理。差异检查通过。未改变后端／远程环境／内嵌发行文件。此前 115 项报告不验收本次归档 UI；浏览器确认、历史／新批次交互、未知写入恢复、外部依赖及完整目标的其余工作仍待推进。

### 逐项执行浏览器证据

Chromium `artifacts/webui-live-alSWRl/report.json` 和 Firefox `artifacts/webui-live-cO0zjO/report.json` 各通过 115 项真实服务检查，使用 `bin/rjs-management-batch-execution.exe`、`index-l49rUMLL.js` 和 `index-Bcxqs_G2.css`。本机构建的后端包含修正后的 DLQ 目标 Stream 校验；构建出现非致命模块缓存写入警告但成功完成。两份报告均核对运行前后输入，独立文件哈希亦一致。两份中文移动端结果截图已检查，页面无横向溢出。测试专属服务正常退出，未改变远程环境或内嵌发行文件。

新增执行路径验证独立批次确认、前置依赖门禁、仅预览不创建、逐项独立批准、按目标／来源顺序恰好两次仅创建 PUT、已保存精确 int64、切换项使预览失效、站内导航恢复及返回保留批次。全部 316 项前端测试、相关 Go 测试及差异检查也通过。这证明所测成功路径，不代表未知写入浏览器恢复、外部依赖执行、归档／替换、完整无障碍、规模或发布就绪。这些要求和完整 WebUI／后端目标仍待完成。

### 修正仅检查声明存在的 DLQ 依赖校验

执行复核发现：预览与提交共用的 `checkDeadLetterDependency` 此前仅检查已保存声明、直接双 Queue 循环和优先级兼容。此前暗示已经检查实时目标的说明过强。现增加声明身份／还原校验，读取精确目标 Stream，复用规范 Stream 归属／配置比较，并再次读取目标 KV 修订。目标在预览后消失时，提交会在修改来源／DLQ 资源前被阻止，不自动修复目标。这仍是快照检查，不代表 Consumer／副本健康、投递证明或跨 Queue 事务。

新增两项测试（十二场景只读矩阵、预览到提交间目标消失），修正原有合法 DLQ 用例，使其具备真实目标 Stream。`go test ./management/internal/jetstream ./management/internal/api ./internal/topology ./api` 通过；静态检查经批准重试本机缓存访问后通过，差异检查通过。本轮优先修复发现的前置缺口，不宣称执行浏览器覆盖。未改变可执行文件／候选资源或远程负载；下一次真实服务套件前须重建后端。逐项执行浏览器验收及其余 WebUI 范围仍待完成。

### 批次逐项执行：源码接入

实现会话级批次协调和一次审阅一项的界面，复用现有创建请求登记与 Queue 编辑器。启动批次需独立确认文件／问题，不执行写入。准备的项继续使用仅创建前置条件、最新服务端预览与独立提交确认。批次内前置项须已有接受的请求，其实时有效性仍由依赖项预览检查。切换项清除预览批准；请求进行中／结果未知时锁定切换；旧选中模型引用不能预览／提交。修改依赖和归档前置项会阻止发送。无效／外部依赖规划项仍阻塞，已排序的独立项可以推进。不引入自动重试或回滚。

批次在站内导航时保留，并纳入会话清除／页面离开保护；当前每会话支持一个保留批次。新增六项协调测试，全部 316 项前端测试通过。Vite 临时目录经批准重试后候选构建通过：`index-l49rUMLL.js`／`index-Bcxqs_G2.css`。Vite 提示 JavaScript 块超过 500 kB，构建体积优化仍待完成。差异检查通过。未改变后端二进制、远程负载或内嵌发行文件。此前 113 项报告仅覆盖规划，不验收本次执行界面。浏览器执行覆盖、外部依赖执行、批次归档／替换及更完整的结果处理仍待完成。

### 批次规划真实服务浏览器证据

Chromium `artifacts/webui-live-kKI38t/report.json` 和 Firefox `artifacts/webui-live-lXdhh9/report.json` 各通过 113 项检查，使用本机构建的 `bin/rjs-management-batch.exe`、`index-D85z3-7D.js` 和 `index-Bcxqs_G2.css`。两份报告均核对运行前后候选指纹，并独立与当前文件核对一致。后端构建出现非致命 Go 模块缓存写入警告，但成功完成。相关 API／拓扑／契约测试、静态检查及差异检查通过，测试专属服务正常退出。未改变远程主机或内嵌发行文件。

新增辅助测试覆盖实际多文件选择、无效文件阻止请求、精确 int64 传输、依赖优先顺序、两个待规划 Queue 均未创建、重复／无效声明、存在／缺失外部观测仍保持阻塞、畸形响应清除、重试及显式清除。两份中文移动端截图已检查：长名称换行且页面无横向溢出，但重复长文件名仍较占空间。这证明所测规划生命周期，不代表逐项执行、完整无障碍、规模或发布验收。目标端预览／提交及逐项结果仍未完成。

### 批次规划 UI：源码接入与模型验证

创建 Queue 页面现可在本地读取多个 UTF-8 JSON 文件，仅在显式规划后发送声明。文件名与输入顺序保留在本地展示。读取失败阻止整个请求；声明语义无效时服务端仍保留索引。界面分别展示逐项问题、依赖优先顺序及外部声明观测，并明确本步不创建 Queue、不授权写入。能力变化、更换文件及取消会清除原有规划证据。

按 feature-dev 流程新增七项模型测试，全部 310 项前端测试通过。Vite 临时目录权限问题经批准重试后候选构建通过（`index-D85z3-7D.js`、`index-Bcxqs_G2.css`）。响应校验拒绝身份／索引不符、未阻塞项名称重复、错误顺序和不完整外部证据。长文件名可换行。本批次 UI 尚未完成浏览器验证，未生成新后端可执行文件或远程部署。先前 111 项浏览器报告仅覆盖此前单文件候选版。逐项目标端预览／提交及结果仍待实现。

### 批次导入规划 API 与外部声明观测

实现仅 operator 可用的 `POST /api/v1/queues/import-plan`，复用预览鉴权／能力检查和纯拓扑规划器。严格有界 JSON 在类型化验证失败时仍保留项索引与安全名称。外部引用去重后形成声明层观测及精确 ETag，不授权写入，也不静默将外部依赖标为满足。查找阶段共享五秒期限，取消后不返回部分响应。

新增三项 API 测试通过。修正测试辅助函数返回类型，并经批准使用本机 Go 缓存重跑扩大验证后，`go test ./management/internal/api ./internal/topology ./api`、`go vet ./management/internal/api` 和差异检查通过。已更新 OpenAPI 和双语声明迁移文档。未修改 UI、候选可执行文件，未运行浏览器或改变远程工作负载；批次 UI 和逐项预览／提交仍待完成。

### 多 Queue 依赖规划内核（API／UI 待接入）

按 feature-development 工作流实现面向 1–100 个声明的纯计算 `topology.PlanImport`，复用规范计划校验。保留逐项索引／问题，区分真正循环成员和被阻塞的依赖方，将外部依赖标记为未验证，仅为未阻塞项计算确定的依赖优先顺序。共享规划器输入切片复制，保证导入和路由探测都不修改调用方数据。内部就绪明确不代表目标端就绪或写入授权。

新增五项测试及现有拓扑测试通过。扩大验证首次遇到本机 Go 缓存权限问题，经批准运行 `go test ./internal/topology ./management/internal/api ./api`、`go vet ./internal/topology` 和差异检查通过。未接入 API／UI、构建候选二进制、运行浏览器或执行远程操作。目标端依赖检查、逐项预览／提交和结果仍待完成，参见[声明迁移契约](webui-declaration-portability.zh-CN.md)。

### 单文件导入浏览器流程与原生文件控件修正

最终修正候选证据：Chromium `artifacts/webui-live-XdHnBC/report.json` 和 Firefox `artifacts/webui-live-hmhxY8/report.json` 各通过 111 项检查，使用 `index-CbKaVW9R.js`／`index-B8d9kddo.css` 及未变化的 `bin/rjs-management-export.exe`。输入指纹在运行前后验证，另行对照当前文件核验一致。修正后的中文移动端截图已检查，差异检查通过，测试所属服务正常停止。这证明已覆盖的单文件创建流程，不证明多文件依赖、完整无障碍或发布就绪。

新增真实浏览器文件选择、畸形文件恢复、拒绝替换表单、显式导入草稿审阅、焦点交接、未知字段服务端预览拒绝、精确值修正及独立确认的仅创建提交。辅助测试断言选择／准备不触发 Queue API I/O、预览后资源仍不存在、恰好一次条件 PUT，以及保留名称拒绝。首轮 Firefox `webui-live-Tyb3sa` 通过 111 项检查；首轮 Chromium `webui-live-Bu5meT` 在后续导入断言前发现原生文件输入框导致移动端溢出，保留此失败证据。

已将文件输入限制在面板宽度内，并明确显示读取成功的文件名／字节数，因为原生选择器会重置以允许重新选择同一文件。全部 303 项前端测试通过；重建 `index-CbKaVW9R.js`／`index-B8d9kddo.css`。最终重跑证据另行记录，不能由此前 Firefox 通过推断。未修改后端或远程工作负载。

### 单文件导入受保护的创建草稿

按 feature-development 工作流，在创建 Queue 页面新增本地有界 UTF-8／无损 JSON 读取和审阅面板。导入显式准备现有仅创建编辑器，没有引入独立写路径。当前创建 Schema 可用性和副本／存储选项控制能否准备；替换已有表单需要同意，保留的请求身份不能覆盖，仍必须服务端预览。未知 spec 字段保留供服务端拒绝，不宣称已有完整浏览器端 Schema 验证。

新增五项测试，覆盖文件／文档封装／上限／取消、精确值、无 I/O 准备、当前 Schema 门槛和仅创建冲突保护。全部 303 项前端测试和候选构建通过（`index-BoFKWhiO.js`／`index-LdfbKOyh.css`）。未重跑浏览器集成，之前下载报告只证明其旧输入。多文件依赖和逐项结果仍待完成。未修改后端、嵌入式发布资源或远程工作负载。

### 单 Queue 导出真实浏览器下载验收

按 feature-development 验证流程新增真实服务辅助测试，下载浏览器实际文件，无损核验 int64 最大值／优先级零值和标签脱敏／包含，观测策略切换／卸载时原生 Blob 撤销，并覆盖真实声明变化、畸形响应及取消／重试，保存中文移动端展示。测试值为合成数据，文件不包含真实凭据。已在本机构建支持导出的后端 `bin/rjs-management-export.exe`。

候选 `index-Adk8ZOtU.js`／`index-LdfbKOyh.css` 在 Chromium（`artifacts/webui-live-5KnJd9/report.json`）和 Firefox（`artifacts/webui-live-Ug1UsF/report.json`）中各通过完整套件 109 项检查。报告验证运行前后输入，独立指纹核验与当前文件一致。中文移动端导出截图已检查，差异检查通过，所属服务已停止。本轮未发现需要修正的应用缺陷。导入／依赖流程、完整无障碍和发布资格验收仍待完成，未修改嵌入式资源或远程系统。

### 候选 Queue 导出下载界面

按 feature-development 工作流实现双语配置页导出面板和独立手动读取模型。在准备有界本地下载前，核验精确来源 ETag、所选标签策略下完整文档相等、范围、省略标签数和附件文件名。标签策略变化／取消会清除证据，迟到响应被隔离。替换／清理时释放本地 Blob URL，不生成带凭据的链接。缺失／不可还原的来源文档无法通过面板导出。

全部 298 项前端测试通过，其中新增四项导出模型回归。经批准本机构建候选成功：`index-Adk8ZOtU.js`／`index-LdfbKOyh.css`。未重建后端可执行文件或重跑浏览器；此前浏览器报告只证明旧候选。真实下载内容／Blob 生命周期、布局及完整 WEB-023 导入／依赖流程仍待完成。参见[声明迁移契约](webui-declaration-portability.zh-CN.md)。未修改嵌入式资源或远程系统。

### 单 Queue 声明导出 API（WEB-023，UI／导入待完成）

按 feature-development 工作流复用完整 Queue 还原及规范 Queue Schema，新增有界、经过鉴权的 JSON 附件端点。默认导出移除全部自由文本标签；显式包含标签时可往返生成相同修订。来源 ETag、省略标签数量和单 Queue 范围是响应来源信息，不是目标写入前置条件。不含 Plan／运行／消息数据，也不递归包含依赖。参见[声明迁移契约](webui-declaration-portability.zh-CN.md)。

新增三项 API 测试，覆盖精确值、标签策略、无修改、鉴权、查询／身份／旧声明错误、输出上限与取消。本机 Go 缓存权限失败后，经批准运行 `go test ./management/internal/api ./internal/topology ./api` 和 `go vet ./management/internal/api` 通过。未构建候选二进制／UI、运行浏览器或执行远程操作。下载 UI、导入／依赖预览／逐项结果及完整 WEB-023 仍待完成。

### 绑定结果键盘可访问性与错误契约浏览器证据

新增有名称、可获得键盘焦点的绑定结果区域，复用现有可见焦点样式。浏览器测试从探测按钮用 Tab 进入区域，以 ArrowRight 滚到最后一列，确认焦点保留，保存中文移动端截图，再用 Shift+Tab 退出。取消请求的测试等待现有有界进入断言及无条件释放。新增真实浏览器未知／畸形 404／409、畸形 400／403 用例，要求清除证据、正确显示无效／拒绝并通过真实读取恢复。这也完成了上一项仅源码错误修正的浏览器验证。

全部 294 项前端测试通过。重建候选 `index-C0VjHKI1.js`／`index-LdfbKOyh.css`，搭配未变化的 `bin/rjs-management-routing.exe`，在 Chromium（`artifacts/webui-live-5vyz2i/report.json`）和 Firefox（`artifacts/webui-live-GflMmX/report.json`）中各通过 107 项检查。两份报告验证运行前后输入，独立磁盘指纹核验一致。键盘截图已检查，差异检查通过，测试所属进程已停止。全部状态的完整无障碍、声明映射表键盘访问，以及规模／发布资格验收仍未完成。未修改嵌入式发布资源或远程系统。

### 路由错误契约修正

检查发现此前任意 404 都被显示为 Queue 缺失，任意 409 都被显示为声明证据变化，包括代理错误和畸形正文。源码模型现要求明确的 `not_found`／`read_api_disabled`／`routing_declaration_unavailable` 错误码才能得出对应结论。解码无效优先于资源结论；即使正文无效，401／403 仍优先表示权限拒绝。新增经过实际 `createAPI` 解码器的回归，覆盖未知错误码、HTML／截断错误、明确错误、证据清除、后续不可用及恢复。全部 294 项前端测试和差异检查通过。

本次仅修正源码／测试。未重建此前通过浏览器验证的候选资源，新错误处理尚未经过浏览器验收。此前 105 项检查报告仅证明其记录的旧输入指纹，不证明本次源码改动。未修改后端或远程工作负载。

### 真实绑定模式集成与移动端表格修正

最终修正候选证据：Chromium `artifacts/webui-live-6sjmJ6/report.json` 和 Firefox `artifacts/webui-live-sAEiuQ/report.json` 各通过 105 项检查。两份报告验证运行前后输入，另行对照当前文件核验指纹一致。修正后的中文移动端截图已检查，测试所属服务正常停止。完整无障碍、更大规模路由展示和更广泛发布资格验收仍未完成，不能据此宣称完整发布就绪。

隔离浏览器套件新增独立的真实 direct/topic/fanout Queue，验证逐绑定行匹配隔离、topic 末尾 `#` 的零层匹配、中文展示、真实声明修订／ETag 变化后的拒绝展示、刷新页面恢复及取消／重试。首次 Chromium `webui-live-8fg7J8` 和 Firefox `webui-live-DEeTes` 通过，但检查中文移动端截图发现列被挤成难以阅读的竖排文字，尽管页面无溢出断言通过。

已修正候选 CSS，为路由表设置最小宽度，在现有容器内横向滚动，并增加移动端绑定表宽度／滚动断言。重建候选 `index-DqBiZCDn.js`／`index-LdfbKOyh.css`；全部 293 项前端测试通过。这遵循 feature-development 的渲染／验证步骤，也说明仅功能检查不足。此前通过的报告不能证明修正后的候选合格，最终重跑证据另行记录。未修改远程系统或嵌入式发布资源。

### 候选路由探测界面与验证

最终结果：Chromium 运行 `artifacts/webui-live-A5qIwU/report.json` 的全部 103 项检查通过，输入指纹已在运行前后验证，测试所属进程已清理。这只证明下述候选在已覆盖场景中的结果，不代表 WEB-020 全部完成或可提升为发布版。未宣称对本候选重跑 Firefox。

在 Queue 路由标签页新增双语手动探测，使用后端翻译器，没有另写 JavaScript 路由语义。结果区分绑定与生成的 Stream 匹配，携带修订／完成时间证据，不宣称投递成功。修改输入、切换模式、取消／卸载及失败均清除此前证据；验证响应身份／字段、修订和精确 ETag。新增五项前端测试，覆盖查询上限、错误／无匹配区分、版本变化、字段校验和迟到响应隔离；全部 293 项前端测试通过。Vite 临时文件权限失败后，经批准重试，候选构建通过。

首次真实 Chromium 运行（`artifacts/webui-live-3lVlsN`）通过字面匹配／无匹配后，因无法精确定位模式选择框标签而失败。已为两个选择框添加明确可访问名称。重建候选为 `index-Co7QZQPX.js`／`index-lGfTAhZ9.css`，搭配独立构建的 `bin/rjs-management-routing.exe` 测试。随后运行（`artifacts/webui-live-A5qIwU`）已生成路由桌面／移动端截图，完成新增优先级匹配／无匹配、不支持的交换机输入及恢复检查；完整套件结果需另行核实。375px 截图已检查，无溢出断言通过。局部桌面／移动端**仅文字对比度**扫描为零违规／零未完成，不代表完整无障碍能力。未替换嵌入式资源或修改远程工作负载。绑定模式浏览器结果、实时修订／取消／双语覆盖及更广泛资格验收仍未完成。

### 已保存声明的路由 API（WEB-020，页面待接入）

新增 `GET /api/v1/queues/{queue}/routing-probe`，沿用现有资源读取鉴权。处理器严格限制／解析输入，在三秒期限内读取一个声明，检查身份／修订与完整还原，再调用纯拓扑探测。响应携带精确 KV ETag 和 no-store；证据超限时失败而非截断。新增 OpenAPI 字段契约及四项 API 测试，覆盖读取前鉴权、精确修订、验证、缺失／不一致／旧声明、取消、无修改和响应上限。

本机 Go 缓存访问失败后，经批准运行 `go test ./internal/topology ./management/internal/api ./api` 和 `go vet ./management/internal/api` 通过；差异检查通过。[路由文档](webui-routing-diagnostics.zh-CN.md)记录输入上限和错误语义。未集成前端、构建候选、执行真实 broker／浏览器验证或远程修改。WEB-020 与完整开发／发布范围继续保持未完成。

### 路由诊断计算内核（WEB-020，待集成）

按 feature-development 工作流复用现有拓扑规划器和交换机翻译，没有在前端另写路由语义。新增单 Queue 纯计算探测，包含声明修订、独立的绑定／Stream 匹配、字面 Subject 验证、显式拒绝优先级交换机解析，以及输入切片隔离。测试覆盖 direct/topic/fanout、末尾 `#` 的零层展开、NATS 通配符边界、交换机隔离、ingress／优先级行为、不支持的输入、确定性输出和输入不变性。

首次测试样例漏填必需的副本数，已修正。随后遇到 Go 缓存权限失败，经批准在本机重跑，`go test ./internal/topology` 和 `go vet ./internal/topology` 通过。参见[路由诊断边界与剩余集成](webui-routing-diagnostics.zh-CN.md)。未宣称已集成 API／UI、构建候选、重跑浏览器或执行远程操作。订阅可见性仍待确认；WEB-020 和完整目标继续保持未完成。

### 精确连接在途取消与期限证据

新增两项后端回归，覆盖此前未测试的详情读取边界。确定性传输检查身份解析和精确详情使用相同期限，保留更早的调用方期限，否则采用共享五秒上限。真实本机 HTTP 测试在进入 `/connz` 后取消，分别覆盖尚未返回响应头和已刷出部分正文；要求上游 context 取消、调用方及时以不可用结束，不返回部分详情或误报缺失。新增测试连续运行十次通过。

首次遇到本机 Go 缓存权限错误后，经审批运行 `go test ./management/... ./api/...`、`go vet ./management/internal/monitoring` 及差异检查均通过。这些场景未证明存在实现缺陷，本轮未改变应用行为、二进制、UI 资源或远程负载。订阅元数据可见性仍待负责人确认，本次工作不授予该权限，也不代表更广功能/发布范围完成。

### 连接详情调度与解码回归

[订阅方案](webui-subscription-design.zh-CN.md)中的元数据可见性仍待负责人答复，自动目标续跑不视为批准。未新增订阅端点、明文元数据暴露或角色变更。本轮继续推进已实现数值连接详情中不依赖该决策的工作。

新增三项回归：基于完成时间的 20/40/60 秒封顶退避及恢复重置；隐藏/手动模式和读取中偏好变化不产生并发请求；真实无损 API 解码器处理畸形/空/尾随/不安全数字响应。无效证据清除时间和值，后续不可用响应不能使旧值复活，恢复后精确值重现，畸形拒绝响应仍优先按权限拒绝处理。前端全部 288 项测试及差异检查通过。本轮仅改变测试/文档，既有候选资源和先前浏览器证据未改变，不宣称新浏览器回归或远程操作。整体目标保持未完成，包括订阅权限确认与剩余功能。

### 精确连接页面浏览器集成及移动端修复

真实服务浏览器套件现从列表 CID 进入详情，等待真实周期读取，验证不可用历史保留与节点/CID 缺失、拒绝、歧义和非法 JSON 清空，再通过真实读取恢复。检查每页 25 行的返回上下文、浏览器前进/后退、双语及桌面/移动端可访问性。首次 Chromium（`artifacts/webui-live-5vgBp1/report.json`）在详情功能检查后发现移动端溢出：长节点 ID 段落缺少断行规则。已增加详情范围内的段落断行；移动端截图现先于溢出断言保存，失败也能保留视觉证据。

前端全部 285 项测试及重构建候选通过。最终 `index-BnWAuTyS.js` / `index-DhuVBk5C.css` 配合 `bin/rjs-management-connection-detail.exe`，在 Chromium（`artifacts/webui-live-cwDRUN/report.json`）和 Firefox（`artifacts/webui-live-P1OqXt/report.json`）各通过 102 项检查。两套输入指纹一致，运行前后已核验并独立与磁盘重核。已检查移动端详情截图。两者详情桌面/移动端扫描均无违规或未确定项，不代表全状态可访问性验收。差异检查通过，测试自有服务正常停止。未修改后端或远程负载，未提升内嵌发布资源。搜索、客户端元数据策略、订阅详情、规模/集群故障及完整发布验收仍未完成。

### 候选精确连接详情页

连接列表 CID 链接现进入由节点 ID 与精确 uint64 CID 标识的独立详情路由。路由保留来源列表 offset/limit 供返回链接使用，但详情 API 仅收到节点/CID。页面复用刷新偏好、隐藏暂停、单请求/退避及卸载隔离，保留原来源时间戳和精确数值计数。连接缺失与节点缺失分别提示；不可用观测保留明确标记的历史证据，确认缺失/拒绝/禁用/歧义/无效时清空。不虚构消失原因、健康结论、客户端身份或订阅内容。feature-dev 指导复用列表投影校验及既有导航/刷新边界。

新增五项测试覆盖路由/上下文/uint64 边界、精确请求及纯数值投影、错误保留/清空/恢复、畸形身份/时间/计数和取消后的迟到响应。前端全部 285 项测试及候选 `index-Cm1L1MTm.js` 构建通过，差异检查通过。本次新页面/资源尚未完成浏览器、真实入口/历史或响应式/可访问性验证；此前 100 项浏览器报告不验证它。后续真实测试需使用单独构建的支持详情 API 的管理可执行文件。本轮未改变后端、远程服务或内嵌发布资源。完整连接搜索/订阅详情和更广要求仍未完成。

### 真实连接详情验收及上游总数语义更正

真实 NATS 验收暴露了此前模拟夹具未覆盖的契约错误：精确 CID 的 `connz` 保留节点连接总数。先前读取源码时混淆了局部 `totalClients` 与响应 `c.Total`，导致多客户端节点上的有效详情返回 503。已移除错误的总数等于匹配数校验；依据零或一条有效返回行判断 CID 存在性，保留原身份、元数据和计数校验。新增节点总数 200 的有效详情、其他客户端存在时目标 CID 缺失，以及总数为零却返回行的非法响应回归。已更正 OpenAPI、中英文文档及此前执行记录中的错误断言，无需修改 NATS 子树。

扩展 `tests/integration/connections-live.mjs`，逐一查询 201 个真实客户端，验证纯数值无总数投影、GET/HEAD 拒绝、非法 CID/查询拒绝、节点缺失与最大 uint64 CID 缺失区分，以及关闭 CID 后详情消失而其他连接仍可读取。首次 `connections-live-4AqJsZ` 失败记录保留。最终 `artifacts/connections-live-hRyWHx/report.json` 通过 12 项检查：共 202 个连接、五页、201 次精确详情读取，运行前后二进制指纹一致。仅在本机构建独立 `bin/rjs-management-connection-detail.exe`（SHA-256 `27aa24caa89e83d0d175b61434c0d5bbf0f61814719539f78ff5e846a7ed5dd7`），保留旧候选二进制。自有服务/套接字正常停止。管理/API 测试、监控静态检查、脚本语法及差异检查通过。未改变远程负载或 UI 资源。前端详情、搜索/订阅详情及更广验收仍未完成。

### 受保护精确连接 API 与共享节点解析

新增 `GET /api/v1/nodes/{node}/connections/{cid}`，精确解析无符号 uint64，拒绝查询参数，沿用资源读取授权、no-store 响应及类型化脱敏错误。节点缺失与连接缺失分别为 `not_found` 和 `connection_not_found`。feature-dev 指导抽取已有解析器供列表及详情共用：仍要求完整身份覆盖、恰好一个匹配端点，以及解析加读取共用五秒期限。未引入较弱回退或当前页扫描。OpenAPI 描述不含歧义总数的纯数值详情。

新增四项 API 测试覆盖 GET/HEAD 授权、精确 CID、I/O 前拒绝非法查询、类型化错误、读取禁用配置及真实 HTTP 监控传输。既有身份测试现同时覆盖列表与详情的节点缺失、重复、不完整及重启观测。OpenAPI 测试检查两种投影及精确数值边界。首次遇到 Go 缓存权限失败后，经审批执行 `go test ./management/... ./api/...` 及 `go vet ./management/internal/monitoring ./management/internal/api` 均通过。差异检查通过。未构建二进制或前端资源，未改动远程服务。下一步是真实 NATS 详情验收及前端详情实现；完整搜索/订阅详情和更广要求仍未完成。

### 精确连接传输基础

新增内部 `monitoring.Client.ConnectionDetail`，使用固定 NATS 的精确 CID 能力，不扫描当前页或枚举集合。feature-dev 指导复用已有有界传输和纯数值投影。校验精确服务器身份及完整 uint64 CID，只有有效零匹配观测表示连接不存在。后续真实 NATS 验收更正：CID 过滤响应保留节点总数，并非匹配数。原总数相等校验有误，现已移除；详情仍不提供该无关总数字段。未引入元数据/订阅扩展、上游修改或数据路径操作。见[详情边界](webui-connections.zh-CN.md#剩余集成)。

新增三项测试覆盖最大 uint64 精确查询/投影、敏感字段排除、有效缺失与畸形/节点变化/不匹配/不一致证据区分、非法输入在 I/O 前拒绝、禁止重定向及发送前取消。监控定向测试通过。扩展测试首次遇到本机 Go 缓存权限错误，经审批重试 `go test ./management/... ./api/...` 及 `go vet ./management/internal/monitoring` 均通过。差异检查通过。未构建新二进制或 UI 资源，未改动远程服务。节点 ID 解析集成、受保护 API/OpenAPI、精确详情 UI、真实节点验收、搜索和订阅详情仍属于整体目标中的未完成项。

### DLQ 受控延迟及校验浏览器验收

候选 `index-G6oAw5xw.js` 在 Chromium（`artifacts/webui-live-yGw3j6/report.json`）和 Firefox（`artifacts/webui-live-zl4fC3/report.json`）各通过 100 项检查。两者候选资源/二进制一致，运行前后已核验，完成后又独立与磁盘核对。脚本现同时检查页面和下载证据中的非法控制器 `lastRun`、`lastSuccess` 及畸形 JSON：不包含控制器值，独立可用的目标/Stream 观测保持有效，后续真实读取可恢复。

每种浏览器两次受控控制器响应验证等待期间按钮禁用及释放后成功恢复。第三次请求主动挂起至生产十秒期限，要求证据转为不可用、刷新恢复、导出不含控制器值且新读取成功。Chromium 记录取消发生于 10,013 毫秒。十五秒断言仅适用于该主动故障，正常导出就绪仍保留五秒断言。这些结果验证受控延迟/超时下的状态收敛，不证明此前自发就绪失败的根因。原失败作为尚未定位的历史证据保留，不据此宣称 UI 永久死锁，也不从验收中静默删除。

脚本语法和差异检查通过。应用源码未变，上一轮 280 项单元测试结果仍适用，本轮无需重构建。测试自有服务正常停止。未改变后端、远程负载或内嵌发布资源。更广功能完成度、完整可访问性、连接搜索/订阅详情、规模/集群故障及发布验收仍未完成；本次 100 项回归不代表整体目标完成。

### DLQ 控制器时间及解码校验修复

新增两项回归在旧实现上失败：不可能的控制器日期（`2026-02-30`）被接受为可用并可导出；畸形 JSON 被标为不可用而非不兼容。两项现均已修复。控制器 `lastRun`/`lastSuccess` 复用严格观测时间校验，保留合法 RFC3339 偏移及纳秒原文，仅将精确 UTC Go 零时间（可带全零小数）视为未报告。共享零时间前缀但小数非零的值不再被静默丢弃。控制器时间无效时清除该来源，不丢弃独立有效的目标/Stream 证据。三个 DLQ 读取的解码失败均分类为不兼容，权限拒绝仍优先。

新增测试覆盖两个时间字段、日历/格式失败、导出排除、精确零值处理和保留精度的恢复，以及目标、Stream、控制器经真实 API 解码器的畸形 JSON、拒绝和恢复。前端全部 280 项测试通过。普通构建遇到 Vite 临时文件权限错误，经审批本机构建重试成功生成 `index-G6oAw5xw.js`。差异检查通过。尚未对该新资源重跑浏览器；此前 98 项报告对应 `index-c960u0Ao.js`。本次修复与未定位的 Chromium 就绪超时相互独立。延迟/重复浏览器读取及新候选回归仍待完成，连接搜索/详情、更广 WebUI/后端和发布工作亦保留。未改变后端、远程服务或内嵌发布资源。

### DLQ 就绪调查与 Chromium 重跑

在应用资源及二进制均未改变的情况下，本次未复现此前 Chromium DLQ 就绪失败。`artifacts/webui-live-trKfQo/report.json` 全部 98 项检查通过，运行前后输入指纹核验通过，记录的 14 次 DLQ 相关读取均在 2–17 毫秒完成。此前失败仍未定位：单次重跑通过不证明根因或修复。未修改应用代码，也未放宽断言或超时。

新增限定范围的浏览器诊断：允许列表内的 DLQ 接口名、响应状态、请求耗时/完成标记，以及导出就绪失败时的来源状态标签与区域截图。该诊断不记录请求头、令牌、响应正文或原始网络错误。新增三项模型测试证明：目标不存在不会提前结束独立控制器读取；真实传输超时能结束停滞控制器读取并允许恢复；过时/已清空读取不能恢复旧证据。前端全部 278 项测试、脚本语法及差异检查通过。测试自有服务正常停止；未改变远程服务、后端代码或内嵌发布资源。

下一步调查：结合这些诊断执行重复及主动延迟的 DLQ 浏览器读取，区分瞬时传输延迟与 UI 生命周期缺陷后再判断是否解决。连接文档已按实际完成的列表/API 工作同步，未删减搜索/详情、集群故障、规模或完整可访问性要求。更广的 WebUI/后端/发布目标仍未完成。

### 连接时间浏览器回归与可见单元格对比度

新增真实浏览器断言，分别向连接两个时间字段注入不可能的日历日期，要求全部连接行和时间清空，再经真实读取恢复。移动端连接首行的六个数字单元格逐一完整滚入视野，对精确目标执行对比度检查，要求实际检测节点数大于零、无违规且无未确定结果。保留原始裁剪扫描及逐单元格截图/报告，已检查 Firefox 最右侧单元格截图。这仅关闭此渲染状态下此前被裁剪的连接单元格对比度复测，不代表完整可访问性验收。

未改变的 `index-c960u0Ao.js` 候选配合专用连接管理二进制，通过 Firefox 全部 98 项检查（`artifacts/webui-live-QMYei4/report.json`），包含运行前后输入指纹核验。Chromium 通过新增连接检查及六个可见单元格对比度检查，但整套在记录 92 项检查后失败（`artifacts/webui-live-HbWKvE/report.json`）：DLQ 证据导出就绪断言等待五秒后，刷新按钮仍禁用。原因尚未确认，不得将 Chromium 计为完整通过，也不通过放宽断言隐藏失败。运行结束后已独立将两份报告的输入哈希与磁盘重新核对。前端测试重跑 275 项通过；脚本语法和差异检查通过。

当前账户最初缺少浏览器可执行文件，经网络审批将锁定版本下载至已忽略的 `artifacts/playwright-browsers`；Firefox 在受限环境外经审批重试后才能建立页面。这些初始环境失败不是页面验证证据。测试自有服务已由脚本停止。未修改远程服务、内嵌发布资源或后端代码。下一步：定位并复现 Chromium DLQ 读取/导出失败，再重跑完整 Chromium 验收。完整连接搜索/订阅详情及更广 WebUI/后端/发布工作仍未完成。

### 严格连接观测时间校验

连接来源时间校验不再依赖宽松的 `Date.parse`：拒绝不可能的日历日期、24 点、无效时区偏移、缺少时区及超过纳秒精度的小数。复用已有精确 RFC3339 解析器，不改变审计行为；合法来源字符串保留原偏移和精度。来源或管理时间无效时清除连接页及本机读取时间，后续合法响应恢复新证据。

前端 275 项测试及候选 `index-c960u0Ao.js` 构建通过。两项新增测试覆盖日历/格式边界及连接两个时间字段的失败/恢复。差异检查通过。本次仅模型强化未重跑浏览器，下方 96 项检查报告对应较早候选，不验证本次新资源哈希。屏外对比度复测、完整连接搜索/订阅详情及更广 WebUI/后端/发布要求仍未完成。未改变后端、远程主机或内嵌发布资源。

### 连接页面浏览器集成与展示修复

真实服务浏览器测试发现，仅支持毫秒的读取时间格式化器将 Go/NATS 纳秒来源时间显示为未知。连接页现直接展示原来源时间，不损失精度。测试还发现页大小标签包含选项文本，已用显式标签/控件关联修复可访问名称。截图复核发现移动端列过窄，尽管自动可访问性未发现违规，仍难以阅读；表格现横向滚动，保留完整数字，整理控件，并显示实际行范围而非裸偏移。

前端 273 项测试及最终构建通过。最终 JS `index-B3a7DJu5.js` / CSS `index-CHiy7tmV.css` 配合专用连接 API 管理二进制，在 Chromium（`artifacts/webui-live-R21RBH/report.json`）和 Firefox（`artifacts/webui-live-BsXGUF/report.json`）各通过 96 项检查，运行前后核验且两者输入一致。覆盖节点入口、真实总数/周期读取、失败保留与清空、查询历史、返回导航及中英文响应式渲染。已检查最终移动端截图。桌面连接扫描无违规/未确定结果；移动端无违规，但屏外单元格对比度仍未确定，需要可见目标复测。此前时间/标签失败及布局改进前的通过报告均不是最终候选证据。差异检查通过，未改变远程服务或内嵌发布资源。完整连接搜索、订阅详情及完整可访问性/功能/发布验收仍未完成。

### 候选节点连接页面

新增节点详情入口、节点范围连接路由、中英文表格/分页、独立来源时间戳及共享刷新调度。查询改变销毁旧读取器；不可用错误保留明确标记的历史页，缺失/歧义/拒绝/禁用/无效结果清空。无损校验精确 uint64 CID 和计数，不向视图传递未知元数据。不提供尚未支持的搜索或订阅详情。feature-dev 流程指导复用已有路由和刷新边界。见[候选页面](webui-connections.zh-CN.md#候选页面)。

前端 272 项测试通过，包含五项新增路由/模型测试，覆盖查询身份、精确数值投影、失败保留/清除、无效响应和迟到完成隔离。最终候选 `index-v-SH5PCa.js` 构建通过。浏览器/真实集成、移动端/可访问性检查及完整连接诊断范围仍未验证；此前浏览器报告不构成本候选验收。未改变远程服务或内嵌发布资源。下方独立真实 API 证据与 UI 验收保持区分。

### 真实 NATS 连接分页验收

新增隔离黑盒测试 `tests/integration/connections-live.mjs`。仅启动本次拥有的回环地址进程，核验随机服务名称和 server ID，建立 201 个真实 NATS 客户端但不发布消息，遍历全部连接页，检查有序唯一 CID 及仅数值投影，关闭一个客户端并验证总数/成员变化。检查匿名拒绝、拒绝未支持搜索、缺失节点错误及越界页原偏移。结束后清理拥有的进程/socket，不复用现有服务。见[测试说明](../tests/README.zh-CN.md#本机连接-api-验收)。

最终 `artifacts/connections-live-XT6FbE/report.json` 通过八项检查，基线一个管理连接加 201 个客户端（总计 202，五页）。运行前后 NATS 及独立 `rjs-management-connections.exe` 二进制指纹一致。较早的 `connections-live-ET551F` 早于测试服务身份强化，不作为最终证据。差异检查通过。未改变原候选浏览器二进制/资源、远程主机或长稳负载。这验证真实单节点连接分页/变化，不验证 WebUI、重启/集群故障验收、搜索、订阅详情或性能目标；这些仍属于整体待办。

### 节点连接读取 API

已将 `GET /api/v1/nodes/{node}/connections` 接入内部解析器，严格校验 offset/limit，映射脱敏错误，采用 5 秒 context 和 no-store 响应。继承已有资源读取授权及显式演示例外；未授权 GET/HEAD 不触发监控。OpenAPI 描述精确数值投影，不把当前页过滤作为完整搜索。feature-dev 流程指导复用已有授权及监控边界。见[连接 API](webui-connections.zh-CN.md#节点连接-api)。

五项 API 测试覆盖授权/默认值、认证禁用、监控缺失、查询拒绝、错误映射及处理器到监控 HTTP 集成的敏感字段排除。另有契约测试检查响应字段及精确 uint64/int64 上限，已有 OpenAPI 安全测试覆盖新操作。命令中断后确认无 Go 进程运行，最终重新执行 `go test ./management/... ./api/...`、`go vet ./management/internal/api ./management/internal/monitoring` 和差异检查均通过。未改变冻结二进制、内嵌 UI 或远程服务。真实 NATS 连接/变化验收、WebUI 集成、完整范围搜索和订阅详情仍未完成，不新增浏览器验收声明。

### 连接节点身份解析

新增内部 `NodeConnections`，按精确节点 ID 解析配置监控端点，`/varz` 和 `/connz` 复用有界/禁止重定向传输。解析最多接受 32 个端点，身份读取并发为 4，共享 5 秒期限。必须完整有效覆盖且仅一个匹配；缺失、歧义、不可用、非法查询和端点超限错误保持可区分。发布连接数据前再次核验 server ID，拒绝重启/重新分配后的响应。无效上游 ID 不能构成完整覆盖。feature-dev 流程指导独立身份校验与明确的覆盖不足行为。见[节点解析契约](webui-connections.zh-CN.md#精确节点解析)。

五项新增测试覆盖唯一/缺失/重复/覆盖不足/重启身份、畸形或超大身份响应、请求/端点限制、并发上限 4 及取消/期限。管理/API 测试通过；最后强化校验后，监控测试重复运行 10 次、静态检查及差异检查通过。未改变 API 路由、UI、冻结制品或远程主机。受保护公开 API/OpenAPI、完整范围客户端/身份搜索、订阅详情及真实节点/规模/UI 验收仍未完成；此内部解析器不代表 WEB-019 完成。

### 连接监控传输基础能力

开始 WEB-019 后端开发，按固定版本 NATS 的 `connz` 实现新增 `monitoring.Client.ConnectionPage`。从配置端点读取有界页，核验预期服务器身份，保留精确 CID/计数及原分页元数据，拒绝无效/超大/截断响应和重定向，不投影凭据、地址、自由名称或订阅 Subject。feature-dev 流程指导传输边界，并与最终公开搜索契约明确区分。见[连接实现和集成待办](webui-connections.zh-CN.md)。

五项监控测试覆盖投影/查询、无效响应、分页/请求限制、截断、重定向及取消。`go test ./management/... ./api/...`、`go vet ./management/internal/monitoring` 及差异检查在本机通过。未改变公开路由、UI、内嵌资源、冻结二进制或远程负载。这不是已完成的连接诊断：节点解析/认证、完整范围搜索、客户端元数据策略、有界订阅详情、API/OpenAPI/UI 及真实节点/规模验收仍未完成。下方浏览器证据早于本次仅源码后端增量，不新增浏览器验收声明。

### Consumer JSON 解码失败边界

下方记录的畸形 JSON 待修项现已修复，覆盖独立 Consumer 详情、Queue 摘要主 Consumer 和 Queue Consumer 集合。解码错误归为无效并清除该来源的保留观测/页和读取时间。后续不可用响应不能恢复已经清除的数据；成功恢复后展示新证据。错误体畸形时仍优先保留 HTTP 拒绝访问分类。摘要 Stream 证据和 Queue 声明/编辑器版本保持独立。这是读取器错误分类修复，不改变后端或写入契约，见[刷新契约](webui-refresh.zh-CN.md#consumer-集合刷新)。

前端 267 项测试及候选构建通过，包含使用实际 API 解码器验证截断/尾随/空/无效数值 JSON、拒绝访问、恢复及详情/摘要/集合清空后不恢复旧值。最终 `index-C7MMz4ir.js` 在 Chromium（`artifacts/webui-live-mzMerT/report.json`）和 Firefox（`artifacts/webui-live-HVES8b/report.json`）各通过 94 项检查，输入一致且运行前后均核验。浏览器覆盖畸形 JSON 清空对应视图/时间、不影响其他声明/Stream 证据及恢复。差异检查通过。未改变远程服务、内嵌发布制品或后端行为。完整视觉/可访问性验收、更广 P0/P1 功能及发布资格验收仍未完成，这些检查不证明其完成。

### 独立 Stream 配置/状态刷新

Stream 详情现按共享偏好使用精确 Stream 独立调度器，与 Consumer 查询调度分开。手动刷新只读取自身来源；改变 Consumer 查询不重启 Stream 读取，也不改变其保留时间。隐藏/手动/请求不重叠/退避/销毁边界与其他已支持页面一致。不可用故障保留明确标记历史的配置、计数、副本证据和完整响应及原时间；缺失/拒绝/禁用/无效响应清除该来源，不清空 Consumer 集合。复核发现 JSON 解码错误此前归为不可用；Stream 读取器现归为无效，包括摘要及 Stream Consumer 集合调用方。feature-dev 流程指导显式启用旧值保留，并维持独立资源/查询生命周期。见 [Stream 语义](webui-refresh.zh-CN.md#独立-stream-配置状态刷新)。

前端 261 项测试、候选构建及 `go test ./management/... ./api/...` 通过。最终 `index-Cy3se1gG.js` 在 Chromium（`artifacts/webui-live-EmyRBB/report.json`）和 Firefox（`artifacts/webui-live-37L0Xg/report.json`）各通过 91 项检查，运行前后核验制品且两者输入一致。覆盖真实周期读取、手动来源隔离、查询改变保持时间、网络故障保留及缺失/禁用/拒绝/不可解析 JSON 清空。已检查移动端 Stream 截图，差异检查通过。较早的 `webui-live-2cSGVT` / `webui-live-l1Fpiy` 运行早于解码修复，不是最终候选证据。未改变远程服务、后端行为或内嵌发布制品。

仍需跟进：使用实际 API 解码器和模型复现确认，Consumer 详情仍将畸形 JSON 归为不可用并保留带标记历史值（`failure: unavailable`、`retained: true`）；需与无效响应清空契约对齐，包括摘要调用方。Queue Consumer 集合存在相同分类分支，需补覆盖。本轮不关闭这些读取边界、完整视觉/可访问性验收（包括被裁剪的 DLQ 标签对比度检查）、更广 P0/P1 功能或发布资格验收。下方旧里程碑保留各自历史范围。

### Queue 摘要批次刷新

Queue 摘要现按共享手动/10/30/60 秒偏好，以一个不重叠的只读批次调度声明指定的 Stream 和精确主 Consumer。手动按钮仅选择自身来源，任一读取未结束时均不能插入新请求；自动批次不沿用该选择，仍读取两侧。独立时间不代表原子快照。不可用故障保留明确标记历史的值和原时间；缺失/拒绝/禁用/无效响应仅清除对应来源。不轮询声明、Plan、编辑器 ETag 和审计窗口。feature-dev 流程指导复用已有调度器，并显式启用读取器旧值保留。见[摘要语义](webui-refresh.zh-CN.md#queue-摘要刷新)。

前端 256 项测试通过。已构建候选（`index-Bn_5u9UF.js`）在 Chromium（`artifacts/webui-live-gorHGt/report.json`）和 Firefox（`artifacts/webui-live-bioQVk/report.json`）各通过 89 项检查，运行前后核验制品且两者输入一致。覆盖真实周期观测时间变化且声明证据不变、故障保留与缺失清空、手动模式不轮询及单来源手动 GET。已检查移动端摘要截图，差异检查通过。原有被裁剪的 DLQ 标签对比度检查仍未完成，不代表完整可访问性或视觉验收。未改变后端行为、远程主机或内嵌发布制品。独立 Stream 配置/状态自动刷新及更广功能/发布工作仍未完成。下方旧条目记录各自候选里程碑，不代表当前摘要行为。

### Queue/Stream Consumer 集合刷新

两个 Consumer 集合现按资源/已提交查询使用共享刷新偏好及完成后调度器。同查询网络故障保留精确行、总数、Queue 集合版本及原时间，并显示历史/旧数据提示；冲突（Queue）、缺失/拒绝/禁用/查询拒绝/无效结果清除旧数据。查询改变取消旧读取器/定时器，不复用其他页。周期刷新保留未提交搜索输入及未截断请求偏移。集合刷新不轮询 Queue 声明或 Stream 配置，Queue 集合/页头版本差异仍明确显示；Queue Consumer 链接身份在渲染前校验。feature-dev 流程指导显式启用旧值保留并复用已有有界 API，不触及草稿、前置条件或消息操作。见[集合语义](webui-refresh.zh-CN.md#consumer-集合刷新)。

前端 250 项测试及候选构建通过。Chromium（`artifacts/webui-live-w9lgI0/report.json`）和 Firefox（`artifacts/webui-live-iaK4KG/report.json`）以相同候选输入各通过 87 项检查。新增覆盖真实周期读取、未提交查询/页头证据不变、模拟冲突/版本不一致、失败保留、共享手动模式及显式刷新仅 GET。实际 Broker 故障现断言历史行/时间不变且必须显示旧数据/非当前提示，而不是清空；确认冲突/缺失/权限失败仍清除。已检查移动端集合截图，差异检查通过。未改变后端行为、远程服务或内嵌发布制品。Queue 摘要/Stream 状态刷新、完整视觉/可访问性验收及更广功能/发布工作仍未完成。

### 独立 Consumer 详情刷新

精确 Consumer 页面现按 Stream/name 限定范围，使用共享刷新偏好和单请求调度器。同身份网络故障保留原观测/时间，明确显示历史/新鲜度提示；缺失、拒绝、禁用或无效响应清除观测。同名 Consumer 切换 Stream 时清理旧读取器/定时器。Queue 摘要保留原有手动/不保留旧值行为。复查还发现 `durable`/`ack_policy` 显示字段缺少校验，现拒绝非字符串，避免进入 React 导致渲染错误。feature-dev 流程指导复用精确查询及显式启用旧值保留，而不改变集合或写入行为。[Consumer 文档](webui-consumer-detail.zh-CN.md)现区分当前认证候选行为与早期实施历史。

前端 243 项测试及最终候选构建通过。最终 Chromium（`artifacts/webui-live-4dWkbp/report.json`）和 Firefox（`artifacts/webui-live-xaeuWG/report.json`）以相同输入各通过 84 项检查：真实周期 GET、失败保留/时间、缺失/禁用清除和恢复、共享手动模式，以及显示字段响应错误时不发生渲染崩溃。已检查 Consumer 详情截图。显示字段加固前的较早测试不作为最终候选证据。差异检查通过。未修改后端行为、远程主机或内嵌发布制品。Consumer 集合/Queue 摘要周期刷新、完整设计/可访问性验收及更广功能/发布待办仍未完成。

### 明确禁用读取时清除旧观测

修复总览和节点读取器把 `404 read_api_disabled` 当成普通故障、保留此前受保护观测的问题。两者现清除受影响来源的数据/读取时间，并显示独立的读取禁用提示。普通 404 和状态/错误码不匹配仍保持正常不可用语义，成功的其他来源不受影响；不丢弃草稿、路由输入或写入证据。两个回归测试在修复前复现缺陷。新增后端授权契约测试确认 info/Queue/Stream/节点/控制器读取即使携带旧会话 Bearer Token，也会返回准确的禁用响应。

前端 238 项测试、候选构建及后端授权专项测试通过。Chromium（`artifacts/webui-live-ppSiQN/report.json`）和 Firefox（`artifacts/webui-live-lXifkL/report.json`）以相同候选输入各通过 81 项检查。浏览器注入验证禁用提示、旧数据/时间清除、其他来源独立及成功恢复。这是响应契约覆盖，不是实际滚动认证配置变更验收。差异检查通过；未改变后端行为、远程服务或内嵌发布制品。更广功能开发、可访问性缺口及发布验收仍未完成。

### DLQ 已忽略尝试观测

Controller 状态现将 `dlqIgnored` 与其他计数及运行状态一并累加，包括部分失败批次。Prometheus 新增仅按实例标记的已处理/已忽略计数，并修正已有 moved 指标描述。WebUI 和本地证据显示精确忽略计数，区分旧服务端缺字段与零值，说明忽略事件不证明消息丢失或确认已完成。feature-dev 流程指导沿既有结果/状态/指标/UI 契约增量扩展，未改变消息处理或增加台账。OpenAPI 及双语 [DLQ 文档](webui-dlq-diagnostics.zh-CN.md)说明字段、响应示例及兼容边界。

管理后端/API Go 测试、控制器/API 静态检查、236 项前端测试及前后端候选构建通过。控制器回归覆盖成功、部分失败、空失败及恢复批次的非零忽略计数累加；接口和指标测试覆盖 JSON/指标输出字段。Chromium（`artifacts/webui-live-UWLkM7/report.json`）和 Firefox（`artifacts/webui-live-NBed8l/report.json`）以相同候选输入各通过 79 项检查，包含真实零值、模拟旧服务端缺字段、原生下载保留缺失及恢复。已检查移动端 DLQ 截图。这些样例不证明实际事件转移；逐 Queue 历史、原因观测、已有可访问性缺口及更广发布验收仍未完成。未修改远程主机或内嵌发布制品。

### 节点集合/详情自动刷新

节点页面现共享已有刷新偏好和调度器，按路由/ID 清理读取器，支持单请求读取、隐藏暂停，以及 HTTP 失败和监控部分失败/不可达的有上限退避。网络失败保留旧快照原读取时间并明确标记历史/旧数据；权限拒绝清除数据。新有效快照替换旧身份/指标，因此 Node ID 缺失或匹配不唯一时不能沿用旧详情。feature-dev 流程指导复用监控模型和调度器，未新增 API 或节点操作。见[节点刷新语义](webui-refresh.zh-CN.md#节点集合与详情刷新)。

前端 234 项测试及候选构建通过。Chromium（`artifacts/webui-live-b5eOuu/report.json`）和 Firefox（`artifacts/webui-live-O416if/report.json`）以相同候选输入各通过 78 项真实服务检查。新增浏览器覆盖节点手动模式、真实详情周期刷新、模拟网络错误保留，以及 ID 缺失时详情清除/恢复；原有实际 Broker 停机测试仍通过。已检查移动端节点截图。现有可访问性检查点无确认违规，仍保留裁剪标签未确定项；节点专项可访问性验收及更广功能/发布待办仍未完成。未修改后端 API、远程主机或内嵌发布制品。

### Queue/Stream 列表自动刷新

Queue 与 Stream 列表现共享总览的手动/10/30/60 秒偏好、完成后单请求调度、隐藏暂停、失败退避上限及 30 秒新鲜度标识。每个 URL 查询拥有独立读取器/调度器生命周期，导航取消旧读取；刷新保留已提交查询（包括未截断请求偏移）及未提交搜索输入。设置页明确标注扩展范围，保留已有偏好存储键。feature-dev 流程指导复用调度器和列表旧数据保留模型，未新增详情、预览、写入或审计轮询。见[刷新语义](webui-refresh.zh-CN.md)。

前端 230 项测试及候选构建通过。Chromium（`artifacts/webui-live-vJdLgo/report.json`）和 Firefox（`artifacts/webui-live-S7b0U7/report.json`）分别以相同候选输入通过 76 项真实服务检查。新增真实浏览器检查覆盖 Queue/Stream 周期读取时 URL/未提交搜索不变、共享手动模式无周期请求，以及显式刷新仅发送 GET。原有查询/历史导航、失败保留及写入安全回归通过。已检查移动端设置页截图；完整视觉/可访问性验收仍未完成，包括已记录的 DLQ 裁剪标签。差异检查通过。未修改远程服务、后端 API 或内嵌发布制品；其他读取页面刷新覆盖及更广功能待办仍未完成。

### 页面中的 DLQ 尝试计数解释

双语 DLQ 页面及下载证据现明确区分处理尝试与去重消息转移计数：事件可能重新投递，`moved` 可包含源消息已不存在的情况，成功运行不证明每次转移均成功。导出回归断言这些限制，浏览器测试检查可见提示。前端 225 项测试、前后端候选构建、控制器测试/静态检查及 Chromium 74 项真实服务检查通过（`artifacts/webui-live-iKKMzf/report.json`）。已检查移动端截图。本次候选包含下述部分批次计数修复；尚未针对本次候选重跑 Firefox，完整可访问性及转移历史验收仍未完成。未使用已有或远程服务。

### 保留 DLQ 部分批次计数

修复 DLQ 处理以确认、批次读取或上下文错误结束时，控制器丢弃已返回计数的问题。计数与运行状态现于同一锁内更新。失败运行保留错误及此前成功运行时间；空失败批次不增加计数，恢复后仅累加新返回的结果。表驱动回归测试在修复前复现计数丢失，修复后通过。`go test ./management/... ./api/...` 及控制器 `go vet` 通过。首次竞态测试因 CGO 未启用而无法运行，不算竞态检查通过。未修改远程服务或内嵌发布制品。

### 本地 DLQ 诊断证据导出

新增仅从已读取状态生成的白名单 `rjs.dlq-diagnostic-evidence.v1` 下载。源 Queue/Plan 版本、原声明 ETag/读取时间、目标映射及各来源观测保持独立。不可用状态不导出残留旧值，精确进程级计数保留范围警告。快照变化时替换并释放 Blob URL。这不是后端诊断任务或逐 Queue 转移台账。feature-dev 流程指导复用既有本地下载隐私/生命周期模式。见[下载语义](webui-dlq-diagnostics.zh-CN.md)。

前端 225 项测试、候选构建及 Chromium 真实服务回归通过（`artifacts/webui-live-UR8QxX/report.json`）。原生下载覆盖目标缺失/Stream 未观测及目标恢复状态，核对源 ETag 不变、源时间匹配、无凭据及下载期间零 API 请求。Firefox 以相同候选指纹通过同一套 74 项检查（`artifacts/webui-live-EbAE3k/report.json`），差异检查通过。配置页裁剪标签的已有对比度未确定项仍保留，不悄悄标为解决。未修改后端、远程服务或内嵌发布制品，仅使用隔离样例资源。更广后端观测能力及发布工作仍未完成。

### Queue 只读 DLQ 诊断

Queue 配置页现区分未配置 DLQ、目标声明/Stream 观测、读取拒绝/失败及进程级控制器证据。最多跟随一层目标，不发布、确认或重放消息。控制器实例、标志、上次运行/成功时间、是否存在错误及精确计数均明确标注为本进程的所有 Queue，不作为逐 Queue 转移证明，不复制原始控制器错误。feature-dev 流程指导了复用既有认证 API 及证据边界。见[DLQ 契约](webui-dlq-diagnostics.zh-CN.md)。

前端 223 项测试、本机候选构建和 Chromium 74 项检查通过（`artifacts/webui-live-WRLE2T/report.json`）。两个新建空 Queue 样例验证真实目标读取、模拟目标 404、恢复及诊断零 PUT。已检查移动端截图，新页面 axe 无确认违规；该页面状态的两个横向裁剪导航标签仍属对比度未确定项。Firefox 也以相同候选指纹通过 74 项检查（`artifacts/webui-live-QuIXiv/report.json`），DLQ 可访问性结果一致。差异检查通过。未修改已有服务、远程主机、后端行为或内嵌发布制品；仅使用测试自建进程/资源。逐 Queue 转移历史、完整转移健康观测及更广发布门槛仍未完成。

### 可见目标对比度复测

此前五个移动端对比度未确定目标现有可复现的后续检查：将实际标签/表格元素滚入可见区域，保留原样式，要求 axe 对至少一个实际检测节点明确判定对比度通过。原未确定报告保留，不重写。Chromium 通过全部 73 项功能/复测检查及十次可访问性扫描（`artifacts/webui-live-sRM9k7/report.json`）；五次目标扫描均检测一个节点，零违规、零未确定项。已检查标签和表格复测截图。证据支持原目标因裁剪未能检测，不是修正了颜色。Firefox 通过相同的 73 项检查及十次扫描（`artifacts/webui-live-4GlODT/report.json`），包括五次单节点对比度明确通过。完整候选指纹一致，差异检查通过。

未修改产品、后端或发布资源。更广的对比度状态、屏幕阅读器流程及其他可访问性要求仍未完成；本结果仅解决该样例中的指定目标。

### 自动可访问性检查与语义修正

接入固定版本、仅开发使用的 axe-core 4.10.3，覆盖五个渲染检查点，报告仅保存规则/选择器。不禁用规则。首轮发现已登录页面缺少 h1 和辅助地标嵌套错误；复核同时修正带名称却无相应角色的指标容器。控制台现有视觉隐藏的 h1，声明配置使用有名称的 section，指标使用有名称的 group。保留视觉布局及数据/写入契约。见[运行方式与范围](webui-live-testing.zh-CN.md)。

前端 216 项测试及候选构建通过。Chromium 完整 72 项功能回归和五个 axe 检查点完成，确认违规为零（`artifacts/webui-live-aOTk7V/report.json`）。Firefox 通过相同检查与检查点（`artifacts/webui-live-doUt7Q/report.json`），完整候选指纹一致。两个引擎的移动端总览均保留一组含五个目标的对比度未确定项，位于横向标签/表格内容；属于人工复核待办，不是通过或被屏蔽规则。差异检查通过。未修改后端、远程服务或内嵌发布制品；仅清理测试自建空资源。其他页面、展开对话框、全部语言/缩放组合及屏幕阅读器流程尚未全面审计，完整可访问性门槛仍未关闭。

### 双浏览器真实服务回归

隔离测试现通过 `RJS_TEST_BROWSER` 选择 Chromium 或 Firefox，在候选制品指纹之外记录引擎及实际浏览器版本。两个引擎执行同一套完整断言。Firefox 仅安装在本机 Playwright 缓存，未修改默认浏览器或远程主机设置。Firefox 153.0 通过全部 72 项检查（`artifacts/webui-live-WY7id9/report.json`），包含写入不确定性、原生证据下载、键盘导航和真实 Broker 故障。已检查桌面兼容性和移动端 Queue 总览截图。见[复现命令](webui-live-testing.zh-CN.md)。

Chromium 151.0.7922.34 同样通过 72 项检查（`artifacts/webui-live-5UPe9S/report.json`）。两个报告的完整候选输入指纹精确一致，期间未重建；差异检查通过。这补齐当前 smoke 范围缺少 Firefox 运行的证据，不等于整个视觉/可访问性或发布门槛完成。完整可访问性复核、更广功能覆盖、内嵌资源晋级及原生资格仍未完成。测试清理仅影响自建进程和空删除样例。

### 兼容性工作区与独立构建元数据

新增 `/admin/compatibility`，展示管理运行时/构建、公开 SDK 契约，并复用能力/资格面板。需认证的 `/api/v1/console/build` 不依赖 Broker，只暴露白名单元数据。缺少 VCS 信息保持未知，SDK 未发布状态明确保留。页面不提供升级/重启/开关操作。见[API、隐私及解读边界](webui-compatibility.zh-CN.md)。

前端 216 项测试、管理服务/API Go 测试、API 静态检查、差异检查及本机构建通过。隔离浏览器回归通过（`artifacts/webui-live-LtSBkS/report.json`）：覆盖认证读取、独立来源失败/恢复，以及实际测试 Broker 停止后元数据仍可读取。已检查桌面布局。原审计员六项导航断言已更新为包含兼容性的七项，键盘回归仍执行。feature-dev 流程指导了独立元数据边界及审慎的资格表述。未修改远程服务或内嵌发布制品；测试删除仅限自建空资源。WEB-034 的基础只读工作区已实现；正式资格接入、精确浏览器资源身份、已安装客户端兼容性验证及更广发布门槛仍未完成。

### 请求偏移与观测页偏移分离

修复 Queue/Stream 及 Queue Consumer 列表模型用被截断的响应偏移悄悄替换请求偏移的问题。集合缩小/重新增长后，刷新仍重复原查询；显式分页及筛选重置仍选择新查询。Stream 详情的 Consumer 页面已在刷新时传递路由查询，无需此修改。见[分页契约](webui-refresh.zh-CN.md)。

前端 213 项测试、候选构建、差异检查及现有隔离浏览器回归通过（`artifacts/webui-live-U6Kscs/report.json`）。新增确定性测试核对集合缩小/重新增长时的精确请求偏移，覆盖两类资源集合、Queue Consumers 及显式导航/筛选重置。浏览器验证既有集成流程，不代表现场集合波动复现。未修改服务端契约、内嵌发布制品或远程部署；测试仅删除自建的空删除资源。更广功能及发布工作仍未完成。

### 同查询列表刷新保留

Queue/Stream 手动刷新现对未改变的查询保留此前完整页及时间戳，刷新或失败时明确标记历史行、总数及分页。查询变化及访问/查询拒绝清除旧结果，迟到响应仍被阻止。Stream 计数保持整数精度。见[列表刷新边界](webui-refresh.zh-CN.md)。

前端 211 项测试、本机候选构建、差异检查及隔离浏览器回归通过（`artifacts/webui-live-GZTXwN/report.json`）。真实 Queue 和 Stream 列表各注入一次 503：旧链接及时间戳保留并明确标记，显式刷新成功后清除提示。原写入回归通过。未修改后端、远程服务或内嵌发布制品；仅删除测试自建的空删除资源。列表周期刷新及更广需求仍未完成。

### 总览刷新偏好

设置页现提供明确仅作用于总览的手动/10/30/60 秒偏好，默认 10 秒。仅将白名单标量与语言一起持久化，存储失败时保留内存选择并提示未保存。手动模式允许首次和显式读取，切换间隔不会重叠或取消进行中的批次。下拉框使用显式可访问标签。见[行为与限制](webui-refresh.zh-CN.md)。本节替代下方历史记录中仅保存语言的描述；Token/草稿/证据仍仅在内存。

前端 208 项测试、本机候选构建、差异检查及 69 项隔离浏览器检查通过（`artifacts/webui-live-n24cWn/report.json`）。浏览器验证手动模式下 11 秒内无周期信息请求，显式刷新仍读取，导航保留选择，存储拒绝提示未保存，持久化仅包含白名单语言/刷新值。最终构建还修复了将新数据与上一时钟 tick 比较导致的短暂旧数据误标。此前浏览器失败暴露下拉框标签不明确及测试状态定位过宽，均已修复后完成最终验证。未修改后端、远程服务或内嵌发布制品；测试删除仅限自建空资源。其他只读页面刷新接入及更广的后端、资格与发布工作仍未完成。

### 总览刷新与旧观测保留

仪表盘现每次仅执行一个读取批次，完成后 10 秒刷新，失败退避最长 60 秒，隐藏标签页暂停调度。刷新/失败保留上次成功数值及原时间戳，并明确标记旧观测；权限拒绝清除受保护数据。其他成功面板保持可见。本功能仅影响只读总览，不修改编辑草稿或重试写入。见[刷新契约](webui-refresh.zh-CN.md)。

前端 204 项测试、候选构建、差异检查及隔离真实服务浏览器回归通过（`artifacts/webui-live-FA1eHk/report.json`）。浏览器验证自动刷新、失败来源保留、旧数据提示和手动恢复，并运行原读取/写入场景；调度、退避、可见性及取消有确定性单元测试。未修改远程服务、后端代码及内嵌发布制品；测试仅删除自建的空删除样例资源。可配置刷新偏好及其他只读页面的一致刷新仍未完成，更广需求及发布门槛继续保留。

### 接收端单次写入尝试证据

PUT/DELETE 的审计意图、后端及结果审计失败现提供可选的版本化阶段/影响元数据。候选 UI 展示并下载白名单证据，不解锁结果未知的写入。它描述接收端的一次尝试，不代表此前重放或原始请求的持久完成结果。见[契约及边界](webui-mutation-evidence.zh-CN.md)。

验证通过：前端 199 项测试；管理服务、拓扑及 API Go 测试；API 静态检查；本机候选构建及差异检查。隔离浏览器回归通过 69 项（`artifacts/webui-live-1YczSf/report.json`），包括真实 PUT/DELETE 成功后模拟 `none` 证据、继续锁定、不重复派发及下载证据。已查看移动端证据截图。真实服务端阶段映射由独立单元测试验证，不从模拟浏览器响应推断。仅删除了测试自建的本地空资源，未修改远程服务或内嵌发布制品。持久结果查询及更广的资格/发布门槛仍未完成。

### Schema 控制的路由与标签集合

v1 适配器现核对集合元素类型、标签映射/值规则、路由互斥分支、重复项要求及各类型 key 数量。不修改草稿、不在浏览器执行 Schema 正则，直接生成规则摘要/详情。不支持的集合契约关闭结构化控件并保留专家 JSON。仍使用专用控件和确认语义，不宣称可生成任意未来 Schema 的布局。详见[集合契约](webui-queue-schema.zh-CN.md)。

195 个前端测试、候选构建、差异检查和隔离浏览器回归通过（`artifacts/webui-live-YDYJmu/report.json`，67 项）。仅改变标签值类型的响应使控件关闭，但草稿/写入计数不变，明确重读后恢复。真实 topic/fanout 规则说明及原有路由/标签编辑通过，已检查移动端路由截图。未修改后端代码、已有/远程服务或发布制品；冒烟仅删除其新建且可重建的空资源。持久操作结果、资格接入及其他完整需求/发布门槛仍待完成。

### Schema 支持的创建初始表单

创建初始表单现从认证 Schema 获取副本/存储选项及精确消息数范围，准备模型也检查规则就绪状态和输入值。规则失败/变化时禁用准备但不清空输入，明确重读只进行读取。创建另一个资源时先移除旧规则授权再重新读取。明确部署选择和正数初始配额仍为已说明的创建策略，不插入默认值。详见[创建契约](webui-queue-schema.zh-CN.md)。

193 个前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-rj0CjP/report.json`，66 项）。浏览器验证 Schema 失败/恢复期间保留副本/存储选择及精确输入 9007199254740993，未发送 PUT；原有创建/编辑/删除流程仍通过。已检查移动端 `creation-schema-recovered.png`。未修改后端代码、已有/远程服务或发布制品；冒烟只删除其新建且可重建的空资源。通用 Schema 生成的路由/标签结构、持久操作结果及其他完整目标需求仍待完成。

### Schema 驱动表单控件——候选回归通过

已实现九个标量字段和路由类型选项的 v1 表单适配器。控件类型、枚举顺序、精确范围、必填性及默认值注解来自已核对 Schema，展示标签/布局仍在本地。Schema 缺失/变化时移除结构化控件，保留原始及合并草稿；明确重读只进行读取。成功预览可恢复匹配规则；成功后再次编辑会重读规则。预览/提交、归档或丢弃后抑制迟到表单读取。范围及剩余表单工作见 [Schema 说明](webui-queue-schema.zh-CN.md)。

191 个前端测试、候选构建、差异检查及最终隔离浏览器回归通过（`artifacts/webui-live-Cu2mr4/report.json`，65 项）。较早候选也通过 64 项（`artifacts/webui-live-t0lfik/report.json`），但最终报告覆盖再次编辑/读取观测修正及真实的再次编辑 Schema 失败/恢复场景。新增测试证明 Schema 失败时保留最新声明 ETag 和成功回执，恢复不增加 PUT。已检查移动端标量表单截图。未修改已有/远程服务或后端代码；冒烟仅删除测试新建且可重建的空资源。创建初始设置及通用 Schema 驱动路由/标签结构仍待完成，不代表整个 WebUI 或发布资格完成。

### 服务端诊断到字段的定位

当前服务端预览诊断新增限定在编辑器内、支持键盘的精确标量/路由控件定位。定位会展开已关闭的结构化区；组合/未知/不存在/禁用目标回退原始 JSON，编辑后移除旧诊断。不修改字段、不请求 API、不解锁未知写入。本轮按功能开发流程复核了生命周期和失败路径，详见[定位语义](webui-preview.zh-CN.md)。

186 个前端测试、候选构建、差异检查及隔离真实服务浏览器回归通过（`artifacts/webui-live-yUD09K/report.json`，63 项）。浏览器证明原稿索引 1 通过 Enter 定位 Subject 2、展开折叠区、保持草稿原文/写入计数、保留组合约束定位 JSON，并在编辑后清除定位按钮。已检查移动端焦点截图。未修改后端代码或远程服务；现有冒烟仅删除其新建且可重建的空资源。Schema 驱动表单及其他完整目标需求仍待完成。

### 版本化全字段 Queue Schema

通过认证且不读取后端的接口发布共享拓扑 Schema。Capabilities 绑定内容版本，OpenAPI 引用同一来源。Settings 核对并展示 Schema，已声明的 Schema 读取参与预览/发送前检查和共享审阅失效通知。未知或不兼容 Schema 不允许写入，不自动填充默认值。详见 [Schema 契约、解析器边界和验证](webui-queue-schema.zh-CN.md)。

Go 全量测试/静态检查、本机构建、184 个前端测试、独立元 Schema 及 14 组结构/解析器用例、差异检查和隔离浏览器回归通过（`artifacts/webui-live-EnnJPW/report.json`，62 项）。已检查移动端 Settings 截图。浏览器证据包括认证 Schema 身份/ETag、精确 int64 文本、不兼容刷新恢复和提交前 Schema 不可用时零 PUT。只删除测试新建的空资源（可重建），未修改已有/远程服务或冻结发布制品。完整 Schema 驱动表单、持久操作结果、资格接入及其他剩余需求仍未完成。

### 结构化 Queue 校验诊断

Queue 语义校验新增 `issues_version: rjs.queue-validation.v1` 和 `issues`，每项包含 JSON Pointer `path`、稳定规则 `code` 及可读 `message`。保留原有 `invalid_queue` 和汇总文字。预览及 PUT 在后端/审计操作前拒绝非法文档。语法/类型解析错误不猜测路径。先填充标准默认值再校验，仅在校验成功后排序，因此数组路径指向用户提交顺序。组合约束可指向 `/spec` 或 `/spec/retention`；必填值缺失时路径可指向尚不存在的字段。

编辑器仅在当前只读预览返回 400 时展示已识别且长度受限的诊断数组。未知/非法版本回退到原有受限文本；不解析错误散文、不注入 HTML、不自动修改字段，也不据此推断写操作结果。此项是结构化诊断，不是完整 Schema 驱动表单。

Go 全量测试/静态检查、字段路径/API 定向测试、179 个前端测试、本机构建及隔离真实服务浏览器回归通过（`artifacts/webui-live-eUOUul/report.json`）。已检查移动端 `queue-preview-validation.png`。浏览器验证真实负数限制错误展示服务端路径且未发送 PUT；测试覆盖未排序数组索引、标量/组合规则、解析错误回退及非法诊断数据。只删除了测试新建的空资源（可重建），未修改已有/远程服务或冻结发布制品。

### 共享能力读取通知

Settings 及其他共享能力读取发现契约变化或不可用时，会使保留的未提交创建/编辑/删除审阅失效。草稿与原始 ETag 保留，确认重置。正在处理/结果未知的操作及成功/归档证据不会被解锁。旧凭据响应不能通知新会话。按功能开发流程复核了生命周期，不引入后台轮询或自动重试。

178 个前端测试及候选构建通过。隔离真实服务浏览器回归通过（`artifacts/webui-live-gWvCzY/report.json`），覆盖 Settings 返回编辑/删除页面及失效期间零写入。只删除了测试新建的空资源，可重新创建；未修改远程服务或冻结发布制品。完整 Schema、资格接入、持久操作结果及剩余视觉/无障碍/发布工作仍未完成。下方为历史里程碑，其待办列表记录当时状态。

### 接收实例能力前置条件

能力接口现在返回由不可变契约/配置及报告构建版本生成的不透明 ETag。新增 `X-RJS-If-Capabilities-Match` 条件，在 apply/delete 及两种预览的认证之后、解析/规划、后端访问或审计意图之前执行。版本不匹配返回 412，格式错误/重复返回 400；缺少请求头仍兼容旧客户端。候选写操作要求已声明功能与版本，同时绑定正文和版本并传递原始标识。这是接收实例契约保护，不是二进制证明、资源状态 CAS 或集群级原子性。详见[契约语义](webui-capabilities.zh-CN.md)。

Go 全量测试/vet、本机构建、175 个前端测试、差异检查和隔离真实服务回归通过（`artifacts/webui-live-UrFw9w/report.json`）。真实错误标识的预览/PUT/DELETE 返回 412，声明 ETag 未变化，正常受保护流程通过。单元测试覆盖构建/部署模式身份、非法条件、拒绝时不写审计/资源、原始请求头传递、缺失版本及发送后 412。后者保持未知，不能显示仅适用于前端拦截的“未发送写入”，也不解锁重试。回归仅删除新建的自有空测试资源，可重新创建；已有/远程服务及冻结发布制品未改动。全局主动通知、完整 schema、资格接入和持久化结果判定仍待完成。

### 绑定能力契约的写入审阅

候选创建/编辑/删除流程现在在预览前后读取能力、绑定规范化指纹，并在 PUT/DELETE 前复查。契约变化、缺少必要标识或读取失败会使审阅/确认失效，不发送写入、不生成写请求 ID，原始草稿/ETag 保留；删除手输确认和 force 重置。进入写入后的失败仍保持未知且不重试。下载证据包含能力绑定。指纹忽略对象键和已声明集合的顺序。详见[时点保证与边界](webui-capabilities.zh-CN.md)。

173 个前端测试、候选构建、差异检查及隔离真实服务浏览器回归通过（`artifacts/webui-live-9N4bMZ/report.json`）。在 apply 和 delete 前注入部署契约变化，均验证对应写请求发送数为零，且必须重新预览/确认；既有未知响应测试仍通过。移除了最初冲突的通用错误提示，修正后的移动端截图明确说明未发送写入。未修改后端或远程环境；破坏性回归仅使用其新建的自有空测试资源。Settings 到编辑器的主动通知、服务端原子契约前置条件及完整 schema 驱动表单仍待完成。前端读取检查不能消除最后读取到写入之间的竞争，也不能证明请求均到达相同版本的服务端。

### 认证保护的能力接口与显式部署期望

新增 `GET /api/v1/console/capabilities`，即使 local-demo 也要求 operator/auditor 认证，不读取后端/monitor。`RJS_DEPLOYMENT_PROFILE` 显式声明 unknown/standalone/cluster，非法值在初始化前拒绝，不按节点数推断。契约区分解析器支持的副本/存储/优先级、标准 storage/delivery 默认值、已实现契约标识及未报告的生产资格，不编造副本默认值。Settings 独立于会话权限读取能力，刷新失败/不兼容时移除旧值，不向草稿复制默认值。详见[契约与配置](webui-capabilities.zh-CN.md)。

全量 `go test ./...`、`go vet ./...`、168 个前端测试、本机前后端构建、差异检查及隔离真实服务浏览器回归通过（`artifacts/webui-live-YtPc9k/report.json`、`console-capabilities.json`）。真实验证覆盖 auditor 读取、匿名拒绝、实际 unknown/unspecified 期望、默认值、未报告资格及不兼容响应后的恢复。已修正并检查窄屏标签/值布局。原有破坏性回归仍仅针对两个新建的自有空 Queue，未修改远程环境、部署制品或嵌入式发布 UI。完整 schema 驱动表单、跨编辑器能力/schema 变化失效联动、资格清单接入和自动刷新偏好仍待完成，不代表 C-02 或整体项目完成。

### 单 Queue 编辑到删除的交接

新增明确确认后归档同一 Queue 编辑/创建模型的流程，不清除凭据或其他草稿。原始/原文/合并草稿快照、精确版本及成功回执成为可下载只读记录。旧模型不能恢复或提交，创建流程中的引用同步解除。进行中的读取/写入及未知结果会阻止整个交接，按钮禁用且模型再次检查。删除必须重新读取声明并预检。归档保留于 SPA 导航及过期会话视图；清除会话明确提示保留证据将丢失。详见[交接契约](webui-delete-preview.zh-CN.md)。

165 个前端测试、候选构建、差异检查及隔离真实服务无头回归通过（`artifacts/webui-live-lirIDG/report.json`）。浏览器覆盖取消交接、成功编辑回执归档、非法原文草稿归档、原生下载、归档期间 ETag 不变、登录与另一 Queue 未保存草稿保留，以及未知编辑器的交接/预检禁用。已检查移动端归档/删除截图。破坏性回归仍仅删除新建的两个测试自有空 Queue，未修改已有服务、远程主机或嵌入式发布制品。持久化写操作结果判定、完整视觉/无障碍/跨浏览器及发布验收仍待完成；整体 WebUI/后端目标尚未完成。

### 删除审计翻页与证据下载

新增更早窗口的手动读取和精确游标校验，读取失败保留原窗口/错误，刷新观测时归档此前检查。没有匹配记录的窗口仍保留继续读取入口。翻页与刷新均不改变操作已确认/未知状态。新增明确字段白名单的 `rjs.queue-delete-evidence.v1` JSON 下载，含当前/此前检查和错误分类，排除会话/传输凭据及原始错误文本。凭据到期后的保留证据使用同一个本地下载组件。详见[证据契约](webui-delete-preview.zh-CN.md)。

161 个前端测试、候选构建、差异检查及隔离真实服务无头回归通过（`artifacts/webui-live-RxW8zk/report.json`）。首次浏览器断言发现测试审计历史不足；修正后的测试在每次测试资源删除后增加 130 次无关请求的审计化无变更 apply，验证真实首窗无匹配记录、更早窗口找到记录、游标耗尽、刷新保留历史、不含 bearer 的原生下载及未知结果持续锁定。仅使用测试自有资源，未修改远程环境或嵌入式发布版本。已检查移动端结果截图。单 Queue 编辑/删除交接、持久化结果判定及完整体验/发布验收仍待完成。

### 候选删除界面与未知结果保护

实现独立 operator 删除路由、会话持有的审阅、严格响应校验、精确名称及破坏性影响确认、force 默认关闭、确认失效机制、原始 ETag 条件 DELETE 和单次提交保护。错误保持未知且不自动重试，声明/Stream/请求审计回读仅作证据。SPA 导航及凭据到期展示保留请求证据。同一 Queue 的编辑/创建与删除互斥；当前保留编辑器的交接保守地要求先保存证据再清除会话。详见[流程与剩余限制](webui-delete-preview.zh-CN.md)。

158 个前端测试、候选构建、差异检查及真实服务无头回归通过（`artifacts/webui-live-FrdA9A/report.json`）。测试仅创建并删除其自有的两个空 Queue，验证精确确认、刷新/force 重置、请求头、成功确认、真实删除后的不可读响应、未知状态持续锁定、只读证据及导航保留。已截图检查修正后的移动端确认布局。未修改已有或远程服务、嵌入式发布 UI。完整审计游标遍历、专用删除证据下载、单 Queue 编辑器交接、持久化结果判定及完整视觉/无障碍/跨浏览器验收仍待完成。

### 只读删除预检

新增仅 operator 可用的 `GET /api/v1/queues/{queue}/delete-preview`，要求原始声明 `If-Match`，检查身份并在返回前复查版本，返回 Stream 归属、精确消息/Consumer 计数及默认删除阻止原因。Stream 缺失时省略计数。不获取锁、不修改数据、不写审计。计数不提供原子空队列/未使用保护，force 不能绕过归属限制；不检查 DLQ 依赖方。详见[契约与边界](webui-delete-preview.zh-CN.md)。

全量 `go test ./...`、`go vet ./...`、本机构建管理端候选版本及真实服务无头浏览器回归通过（`artifacts/webui-live-NZLfA7/report.json`、`delete-preview.json`）。只读单元测试替身拒绝任何写入、锁或 ensure 路径。真实预检验证了原始 ETag、归属/计数、auditor 拒绝、旧版本冲突、缺失条件拒绝及声明未变化，未发送 DELETE。前端源码、远程环境和冻结发布制品未改变。删除界面、精确名称确认及未知结果恢复仍待完成。

### 提交值与规范化值审阅

新增独立只读展开项，对照当前提交草稿与服务端提供的规范化目标，列出规范化后补入、省略及表示不同的值，使用转义后的 JSON Pointer 路径与精确 JSON 值。缺失值与 null、空字符串、零保持区分。浏览器不包含独立默认值常量，也不向草稿回填规范化值。数组整体展示；单位/顺序/默认值省略方式等表示变化明确不称为额外语义配置变更。

除整体数组值外忽略对象键顺序。最多显示 256 个不同字段，超限明确提示，完整草稿/规范化文档仍可查看；这不是完整 schema/规则解释。150 项前端测试、候选构建、差异检查和真实服务浏览器回归通过（`artifacts/webui-live-vCJGKH/report.json`）。真实创建预览验证了省略的 ACK 等待/投递次数显示为服务端补入值（`30s`、`5`），草稿未改变、查看未产生 PUT。单测覆盖精确整数、存在性区分、路径转义、顺序及截断。已检查移动端截图。本轮未修改后端/远程环境或提升发布制品。

### 阻止隐式资源接管

协调器之前将冲突的 Queue 归属元数据当作普通安全元数据变更。现在已有 Stream/Consumer 必须具有匹配且非空的 Queue 标记，否则对应操作及整体结果会 blocked，并明确说明不支持接管。提交已有 blocked 结果在资源更新/声明持久化前返回的保护，因此共用检查同时保护预览和执行，包括优先级 Consumer。缺失标记的旧资源有意不再自动接管。匹配标记不等于认证，也不提供外部写者的原子并发保护。

新增单元/后端检查覆盖 Stream、主 Consumer、优先级 Consumer 的缺失/冲突标记，资源快照及声明版本均保持不变。扩展独立真实夹具，在四个 NATS 资源注入其他 Queue 标记，验证预览 blocked、提交 409 且不覆盖标记/声明（`artifacts/webui-live-L07vay/report.json`）。全仓 Go 测试、vet、本机构建、差异检查及启用该选项的真实服务/浏览器回归通过。全仓检查还发现之前的测试工具绕过共用 NATS 连接选项，已接入统一凭据/TLS 策略，未放宽契约测试。拒绝提交期间仍可能有正常审计及队列锁活动。未修改远程服务或生产发布。

### 真实 Broker 元数据漂移验收

通过隔离真实服务脚本的可选 `RJS_TEST_METADATA_DRIFT=1` 和本机构建、记录指纹的专用测试工具，补齐之前仅有假 SDK 的证据。在专用夹具 Stream、主 Consumer 和两个优先级 Consumer 上，通过真实 NATS 配置调用注入过期空值受管标签、移除目标空值标签并添加外部元数据。真实管理预览逐资源报告两项精确的存在性变化，而声明差异仍为空；预览不改变原始 ETag。条件提交修复全部四个资源，保留外部元数据，随后真实预览为 noop。

工具端点/模式限制测试、vet/构建、脚本语法/差异检查及启用该选项的完整真实服务/浏览器套件通过（`artifacts/webui-live-A4s8D0/report.json`；`metadata-drift-preview.json`、`metadata-inject.log`、`metadata-verify.log`）。最初脚本语法错误发生在任何服务启动前，已修正。本轮未修改常规前后端实现，仅使用脚本拥有的本机进程/数据；未修改已有监听服务、远程主机、容器或发布制品。外部元数据并发写者及原生平台资格仍为独立门禁。

### 一致的受管元数据协调

发现资源预览仅比较目标中存在的元数据键，并将缺失键等同空值，而非 noop 提交会整体替换元数据映射。新增共用的有效元数据计算：RJS 命名空间（`rabbit-jetstream.io/`）遵循目标计划，其他观测键保留，除非计划明确提供同名键。预览比较有效映射与观测；Stream、主 Consumer 和优先级 Consumer 提交使用同一合并规则。现可报告/移除过期受管标签、修复缺失的空值标签，其他更新不再清掉无关元数据。

元数据变化字符串现为带引号的值（`""` 表示显式空值）或不带引号的独立标记 `(absent)`，属于有意的输出修正。测试覆盖类似标记的字面值、不修改输入映射、仅有外部元数据时 noop，以及 SDK 假实现中各受管资源的读取/预览/提交收敛。全仓 `go test ./...`、`go vet ./...`、本机管理服务构建、差异检查及既有真实服务浏览器回归通过（`artifacts/webui-live-YNtcA2/report.json`）。完整元数据漂移场景在 SDK 假实现中验证，未向真实 Broker 注入。保留基于观测，不是对外部并发元数据写入的原子合并。未修改远程主机或生产发布。

### 状态机层的预览审阅保护

将审阅数据接收校验放入运行时编辑器的预览转换，修复“无效数据只隐藏 React 控件，底层编辑器仍可能进入 review”的缺口。操作列表缺失/无效/身份重复、单项阻止标志与总阻止标志矛盾，以及声明审阅身份/状态无效，现均进入带 `invalid_preview` 的 `preview-error`，保留草稿及原始 ETag，但不保留可提交预览。该状态下直接调用底层 apply 也会被拒绝。恢复必须重新取得有效预览，并由会话控制器重新确认。旧服务端仍可省略新增的可选声明审阅字段，但不能省略既有必需的操作列表。

已为简化测试夹具补齐必需操作集合，没有放宽校验以迁就测试。新增底层编辑器与会话控制器回归，覆盖之前确认后重新预览收到无效数据、拒绝时无 PUT 及明确恢复。147 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-8FCRlU/report.json`）。已有无效操作/错误声明响应注入现验证状态机拒绝，而非仅 UI 隐藏。这强化客户端工作流完整性，不替代服务端授权或持久幂等机制。本轮未修改后端及远程环境。

### 浏览器声明审阅接入

Queue 编辑器现于观测资源操作上方展示独立声明审阅面板，包含本次预览的原始 ETag、规范化声明字段差异及只读规范化目标展开项。区分仅创建、可比较（包括空差异）、无法重建及旧服务端未提供支持。不替换草稿、不推进原始 ETag，也不将空声明差异视为资源健康或无需修复。保留服务端规范化字符串、未提供的值及精确大整数。

适配器检查 Queue 身份、创建/编辑模式、原始版本、规范化文档身份、状态一致性及重复/无效字段行。审阅信息矛盾时隐藏 UI 确认/提交控件，直到重新取得有效预览；缺失或明确不可用的信息会如实标注，不伪造差异。这仍是 UI 保护，不是新的服务端授权机制。规范化 JSON 可展示服务端默认值，但尚未提供完整逐字段默认值说明。

145 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-FwiNHj/report.json`）。浏览器验证声明值 `100 → 200` 与资源值分开呈现、查看规范化文档不改变草稿、创建说明，以及 Queue 身份不匹配时禁止批准并可重新预览恢复。迭代中已检查移动端面板布局；最终视觉/无障碍验收仍未完成。本轮未修改后端和远程状态，也未提升候选为发布制品。

### 预览 API 中绑定原始版本的声明审阅

PlanPreview/OpenAPI 新增可选 `declaration_review`，区分 available/create/unavailable，提供规范化目标 Queue 文档及仅用于编辑的语义差异。重建复用现有无损证明。比较前检查基线身份、声明/计划内容版本及精确的原始 KV 版本，之后仍执行最终前置条件检查。不支持的记录不伪造差异，读取失败或并发版本变化使预览失败。不新增写入、锁、Bucket、资源或审计操作。

拓扑、JetStream、管理 API、API 契约测试及定向 vet 通过。新增只读包装器测试覆盖创建/无变化、优先级/标签、不支持的目标/基线、四类身份不一致及中途/最终版本竞争。契约漂移检查曾在 OpenAPI 更新前检测到新属性，目前两者已一致。本机构建的新管理候选通过隔离真实服务浏览器回归（`artifacts/webui-live-gZpJ3f/report.json`），验证原始 ETag、规范化目标及 `100 → 200` 的声明上限差异。新审阅信息的 UI 展示仍待接入，现有观测资源面板不变。未操作远程主机或提升发布制品。

### 声明比较器完整性修复

检查后端声明比较基础时复现三处缺陷：从未比较 `maxPriority`（包括省略与显式零）、逗号拼接标签会混淆不同映射、值参数复制后 `Default()` 仍对共享切片原地排序。先新增失败回归，再完成修复。现将优先级变化报告为 disruptive，明确区分 `omitted` 与数值；标签采用确定性的 JSON 映射字符串；默认化前复制 Subjects、绑定行及路由键切片。nil 与空集合归一为相同语义。

修复后拓扑/CLI/API/JetStream 定向测试、全仓 `go test ./...`、`go vet ./...` 及差异检查通过。比较器目前由 `rjsctl diff` 使用；标签的 `from`/`to` 字符串现包含 JSON 对象，而非有歧义的逗号拼接内容，优先级变更可能增加之前漏报的行。下游消费者需注意这一有意的输出修正；本轮未新增管理端点或接入浏览器声明差异。观测资源协调及其展示不变。未修改远程主机、发布二进制或候选构建。

### 结构化观测资源预览审阅

Queue 预览现逐资源展示操作、服务端动作/影响、阻止状态/原因，以及观测值与目标值的字段表。值保持为精确的服务端字符串；缺失值标记为未提供，与显式空文本或零区分。没有逐字段行的 Create/ensure 操作不会显示成“没有变化”的证明。这是观测资源到生成配置的比较，不是两份声明的语义差异、健康报告或审批。原始结果及完整生成计划仍可展开查看。

展示适配器拒绝格式错误或资源身份重复的操作列表，不展示部分表格。结构化操作数据无效时，UI 隐藏确认和提交控件，直到获得新的有效预览；这不构成新的服务端授权边界。现有结果状态及未知写入规则不变。影响等级（包括未来未知等级）仅供参考，不授予权限。

142 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-3M3QzQ/report.json`）。浏览器验证了 `maxMessages` 观测/目标值 `100 → 200`、生成计划展开，以及注入无效操作响应时隐藏批准控件并可重新预览恢复。首次运行发现测试使用了 snake_case 而实际计划为 camelCase，已修正断言。已检查移动端布局，字段表可横向滚动。完整声明语义审阅/默认值解释、后端契约强化及最终视觉/无障碍验收仍待完成。未修改后端源码、内嵌发行版或远程环境。

### 只读预览的校验诊断

编辑器现显示当前 `400 / invalid_queue` 预览失败的服务端诊断，可附带有长度限制的响应请求标识。传输层与响应体错误码必须同时匹配；其他错误类别、无效响应体及已有写入请求标识的状态均不显示。React 按文本而非 HTML 渲染诊断。显示上限 16,384 个字符，超限明确提示截断。编辑草稿通过现有生命周期使旧诊断失效。未增加重试、写入解锁或自动猜测字段名。

140 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-tl1t1J/report.json`）。浏览器将显示诊断与实际后端响应逐字比较，再检查编辑后诊断移除、未知写入恢复中不出现。初始断言要求完整越界整数，但 YAML 解码器会缩写该值；已修正为验证真实响应，而非补造缺失文本。稳定的结构化字段路径及翻译后的字段级错误仍待完成。未修改后端源码、内嵌发布制品或远程环境。

### 防覆盖的结构化标签编辑

共用草稿表单现列出 Queue 标签，提供多行值编辑及明确的新增/重命名/移除控件。原生键输入框仅在确认后提交，取消不影响草稿。精确匹配、区分大小写的重复键会拒绝新增/重命名，不覆盖任一标签。不擅自删除空值或空键；下游约束仍以服务端预览为准。移除须确认。自身属性检查与数据属性构造可保留 `__proto__`、`constructor` 等特殊键，不修改对象原型。未修改配置及大整数保留原值。

标签操作使用现有单一草稿及预览失效路径，不维护另一份表单文档。新增/重命名后键盘焦点移到对应值；移除后返回“添加标签”。冲突错误保留原始草稿及焦点控件。未知/无法表示的文档和锁定写入状态仍遵守现有结构化编辑限制。原生输入框是当前键录入交互，不代表选定设计的最终验收；详细服务端字段错误及完整语义审阅仍未完成。

138 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-M1VDNd/report.json`）。浏览器覆盖取消新增、重复新增/重命名、区分大小写的成功重命名、多行值保留、真实后端预览、取消/确认移除、预览失效、焦点恢复及无新增 PUT。已检查移动端标签截图。未修改后端源码、内嵌发行版或远程环境；完整 WebUI/后端开发及视觉/无障碍验收仍未完成。

### 结构化路由行及本地删除确认

共用草稿表单现支持明确的 Subjects/Bindings 模式、Subject 和 Binding 重复行、Exchange/类型选择及路由键重复行。保留空行、重复项和原始文本供修正，不裁剪或自行转换通配符；路由语义仍由服务端预览验证。新绑定不猜测 Exchange 类型。移除有内容的行、替换路由模式，以及带已有路由键时转为 fanout，均须确认。取消后 JSON 草稿保持不变。Fanout 转换仅在确认后移除键，并禁止添加键；专家 JSON 中的无效 fanout 键仍可见，供明确移除。

专家 JSON 同时包含两种路由时显示模式冲突，不隐式选择。确认选定模式后保留该模式已有内容，仅移除另一种。未知/无法表示的文档仍禁止结构化转换。所有变化沿用现有草稿控制器，使预览/确认失效、保留原始 ETag，不发送写入。标签仍仅在专家 JSON 中编辑；标签引导行、详细字段错误、部署能力及完整语义审阅仍待完成。

135 项前端测试、候选构建、差异检查和隔离真实服务浏览器回归通过（`artifacts/webui-live-y9INZR/report.json`）。浏览器覆盖混合模式内容保留、模式与 fanout 转换的取消/确认、真实后端预览、JSON 到表单同步及无新增 PUT；回归迭代中检查了移动端路由布局。单测补充行下标边界、未知操作/模式、空行/重复项保留、int64 精度及强制确认。未修改后端源码、内嵌发行版或远程环境。完整功能、无障碍及选定设计验收仍未完成。

### 共用草稿的 Queue 结构化配置字段

创建/编辑草稿页现提供带标签的副本数、存储、保留时间/容量/消息数、ACK 等待、投递次数、可选优先级和死信目标字段。适配器绑定 v1alpha1 源码契约，编辑与专家 JSON 相同的无损序列化会话草稿。每次字段变化使预览和确认失效，不推进原始 ETag。留空移除对应属性，不填入客户端默认值；显式优先级零保持不变。未完成的数字输入以字符串保留在 JSON 草稿中；有效十进制整数采用精确整数编码，由服务端预览验证格式、范围和依赖。不舍入、截断或静默丢弃无效输入。

未知字段/版本、无效 JSON 和无法表示的嵌套结构会禁用结构化转换。已有标签、Subjects、Bindings 及其他未修改字段在编辑中保留。路由和标签仍需专家 JSON；绑定/标签重复行控件、完整字段级校验、部署能力及语义审阅仍未完成。这是创建/编辑表单的部分接入，不代表 S-04/S-05 或 C-02 已完成。

132 项前端测试、候选构建、差异检查和真实服务浏览器回归通过（`artifacts/webui-live-RYA7F8/report.json`）。真实 Go 预览拒绝超 int64 上限，接受精确的 `9007199254740993`，再次修改字段后必须重新审阅；这些编辑/预览期间没有 PUT。移动端截图检查发现并修正了字段/焦点间距过紧的问题。已有接受/未知写入及证据下载流程也通过回归。未修改后端源码、内嵌发行版或远程环境；候选仍未发布。

### Queue 编辑器本地证据下载

编辑器现可由用户明确下载当前内存草稿与请求证据 JSON，包含原始/无效编辑内容、原始基线与 ETag、比较结果、提交计划、接受响应及之前接受的请求，以及已读取的检查窗口和 HTTP 失败元数据。整数保持无损。带版本的封装记录序列化时间，并说明不确定性、部分观测和保管要求。明确的字段投影排除会话/传输状态及任意错误消息/响应体；资源文档和审计响应体有意保留，可能包含敏感配置。这不是通用秘密脱敏工具。

下载使用本地 Blob，不调用 API、不重试、不导入、不判定操作结果或解锁写入。替换/卸载时释放旧 Blob URL；新快照准备期间隐藏旧链接。会话到期后，“保留的草稿”展开区仍提供相同下载。清除会话无法删除已下载文件。每个文件只覆盖一个编辑器，不包含所有暂存/已完成的创建草稿或整个会话。Consumer 仍仅覆盖已读取的检查页，审计历史仅覆盖已读取窗口；序列化时间不是观测时间。

128 项前端测试、候选构建和隔离真实服务无头浏览器回归通过（`artifacts/webui-live-AJSS5f/report.json`，启用 `RJS_TEST_SESSION_EXPIRY=1`）。浏览器下载验证了接受请求历史、未知写入草稿/请求/检查结果、不含登录令牌、写入仍锁定及会话到期后的下载能力。到期场景是浏览器生命周期夹具，不是真实 OIDC 资格验收。未修改后端、内嵌发行版、远程主机或发布资格状态；完整 WebUI 与视觉验收仍未完成。

### 迟到读取失败的生命周期覆盖

检查确认共享最新读取辅助模块会同时抑制过时成功与失败。本次补充明确的模型级覆盖，不修改工作正常的生产逻辑：Queue 声明、Consumer 详情、Stream 详情、Stream Consumer 集合、Nodes 分别验证旧 401/404/503 在新请求加载/完成或清除后的行为（45 种组合）。断言旧 signal 已取消、快照身份不变、订阅者不收到更新，且当前读取仍可成功。总览另验证两个独立来源在替换与清除后的行为。

全部 126 项前端测试及差异空白检查通过。测试故意模拟忽略 abort 的传输，验证的是轮次保护，而非依赖浏览器取消。本次补齐失败路径覆盖缺口，不代表全部导航/无障碍或传输资格验收。本轮未修改生产源码、构建制品、部署或远程环境。

### 保留设计约定的旧入口链接

候选现识别 `/admin/` 和 `/admin/index.html` 上的 `#overview`、`#queues`、`#nodes`，映射到对应的新历史路由。规范化替换当前历史条目并保留其 state，因此打开旧书签不会多出一次无用的回退。启动、popstate 和 hashchange 使用同一规范化逻辑，取消订阅时移除监听。带查询参数的入口和资源详情路径的锚点不被改写，仅识别约定的精确锚点。

120 项前端测试、候选构建及真实服务浏览器回归通过（`artifacts/webui-live-QhsL6v/report.json`）。测试覆盖全部旧入口映射、不匹配/带查询参数/资源 URL、历史 state 保留、监听清理及真实浏览器启动/锚点切换/回退。本次实现设计中的旧链接兼容项，未迁移内嵌 UI，也未启用匿名读取。未修改后端或远程环境；完整功能/设计验收仍未完成。

### 拒绝不完整或拼接的监控响应

监控解码器现要求一个 JSON 文档之后正文结束，仅允许尾随空白。第二个 JSON 值、尾随垃圾或合法 JSON 前缀之后的正文读取错误，都会在发布观测前使整个来源无效。varz 失败不暴露身份/指标，并跳过后续来源；routez/jsz 失败保留其他独立有效来源。读取仍受既有 HTTP 超时限制。本次不代表已拒绝重复键或完成全部恶意输入加固审查。

monitoring/API/app 测试、监控包静态检查及重建候选的真实服务浏览器回归通过（`artifacts/webui-live-xq9jX9/report.json`，含输入哈希）。新增用例覆盖三个来源、额外对象/null/数组/垃圾、允许的尾随空白，以及虽含完整 JSON 前缀但实际短于 Content-Length 的 HTTP 正文。未修改前端、上游、部署或远程服务；完整功能/设计验收仍未完成。

### 恢复保留的未写入创建草稿

没有其他活动编辑器时，保留的创建记录现可明确恢复。仅允许本会话拥有、尚未写入的模型，提交中/未知/已发出请求不能使用此路径。新表单非空时须确认丢弃输入。恢复保留原名称及精确原始 JSON（包括无效文本），使旧预览/批准失效，从保留列表移出模型并将焦点置于编辑器。不创建替代模型，也不发送 API 请求；重新预览仍可能报告原有真实冲突。

118 项前端测试、构建及真实服务浏览器回归通过（`artifacts/webui-live-8DYXyk/report.json`）。测试覆盖取消/确认丢弃表单、精确文本、焦点、新预览、无额外写入、无效 JSON、非本会话模型及活动/未决编辑器保护。本次取代此前保留草稿只能只读查看的限制；已接受/未知操作仍走独立流程。feature-dev 指导保留会话模型与既有确认边界。未修改后端、晋级内嵌 UI 或修改远程环境；完整功能/设计验收仍未完成。

### 访问与设置路由

候选现通过响应式导航向已认证角色提供 `/admin/settings`。展示已验证身份、角色、到期时间、资源读取策略和服务端报告的精确权限字符串，明确区分授权与功能/所有权/健康/资格，以及登录验证与持续授权。语言和清除会话入口复用既有处理逻辑，清除继续遵守提交中/草稿/未知结果保护并保留语言偏好。渲染此会话视图无需额外 API 请求或写入。

116 项前端测试、候选构建及真实服务回归通过（`artifacts/webui-live-Xb3M2G/report.json`）。浏览器将权限展示与真实 auditor 会话响应比较，验证双语控件、手机无溢出、清除后重新登录，以及通过设置页取消丢弃未保存草稿。已检查 `access-settings-mobile.png`。路由拒绝查询参数载荷，不影响合法 Queue 名称。feature-dev 指导复用已验证会话状态，不推断能力。部署模式、生产资格和自动刷新偏好明确未提供，因此 S-12 仍为部分实现。未修改后端、旧版内嵌或远程环境，最终设计验收仍未完成。

### 仅记住语言偏好

React 候选现仅将明确选择的 `en`/`zh` 写入 localStorage 的 `rjs.language`。启动优先采用有效保存值，否则跟随浏览器语言（中文变体映射到简体中文，其他语言使用英语）；无效保存值被忽略。切换立即更新页面语言，存储拒绝/配额错误不阻止切换，并提示偏好未保存。清除会话有意保留这项非敏感偏好，凭据、身份、草稿及请求证据仍仅在内存。不代表跨标签实时同步或完整设置页已完成。

115 项前端测试、构建及真实服务浏览器回归通过（`artifacts/webui-live-KuatjB/report.json`）。浏览器验证中英文跨刷新保留、刷新后 Token 输入为空、已认证流程中存储精确只有一个白名单键，以及模拟存储拒绝时仍可切换且明确提示。单元测试覆盖无效保存值、无存储、getter 拒绝和写入失败。feature-dev 指导了独立偏好辅助模块及隐私边界。未修改后端、旧版内嵌资源或远程服务；完整功能/设计验收仍未完成。

### 跨组件回归与制品身份

本机全仓 `go test ./...` 和 `go vet ./...` 通过，包括 baremetal-run 测试（42.041 秒）；部分未变更包使用 Go 测试缓存。113 项前端测试也通过。这是默认本机包测试，不是竞态检测、带构建标签的集成测试、远程稳定性或全部发布门禁。

真实服务浏览器测试现于启动前记录 NATS/management 二进制及候选 HTML/JS/CSS 的精确指纹，并在成功结束前要求文件/哈希不变。本机已重新构建 management，包含近期后端和旧版内嵌修复。完整已实现流程浏览器回归通过，`artifacts/webui-live-UkPpEX/report.json` 包含 `inputs` 与 `inputsVerifiedAt`。本次将证据绑定到受测制品，但不证明源码来源或可复现构建。测试渲染独立 React 候选，不是旧版内嵌控制台的浏览器审查。完整功能/设计/发布完成仍未得到证明；未部署或修改远程服务。

### 重试编辑器首次规范文档读取

候选编辑器现可在首次读取失败或不可编辑时明确选择“重试读取规范文档”。仅在尚无草稿/请求时提供，页面重新挂载不会自动重试。每次必须取得身份匹配的规范文档及原始 ETag 才允许编辑。不支持的声明仍不可编辑，加载期间同步阻止重复点击，清除会话后迟到结果不能恢复状态。已有草稿及未知写入不能进入此恢复路径。

113 项前端测试、候选构建及真实服务浏览器回归通过（`artifacts/webui-live-rAlvHI/report.json`）。浏览器注入重复 HTTP 503，检查明确重试入口/HTTP 状态，再恢复真实端点并继续预览/提交，重试期间没有写入。已检查 `initial-editor-read-failure.png`。单元测试另覆盖不支持的文档、精确整数、重新挂载不重试、草稿保护及迟到读取抑制。未修改后端、晋级内嵌 UI 或修改远程环境。本次完成首次编辑读取恢复，不代表完整功能或视觉验收通过。

### 内嵌仪表盘仅采用最新一轮刷新

旧版仪表盘现为每轮刷新分配序号，并取消上一批只读请求。只有当前轮次可以更新数据、错误提示、时间戳或刷新按钮；即使传输忽略取消，也不能发布迟到结果。可选 signal 仅用于仪表盘 GET，不会取消、重试或修改管理写入及 Consumer 详情请求。

111 项前端测试与 `go test ./admin-ui` 通过。行为测试执行实际内嵌脚本，证明旧成功不能在新失败后恢复节点，旧失败也不能在最新读取仍进行时解锁刷新按钮。最新成功保持权威，被替代请求的 signal 已取消。本次解决此前重叠刷新问题，不涵盖独立的 Queue/账户旧数据或聚合语义。本轮未执行浏览器审查、部署、React 晋级或远程修改。

### 内嵌旧版监控兼容性

仓库检查未发现其他运行时调用方使用可选 JetStream enabled 字段，旧版仪表盘并不显示它。但仍在内嵌的节点渲染存在相关问题：缺失/不可用指标被补零，读取成功标成 Healthy，节点刷新失败保留旧卡片。旧版节点指标现校验来源与有效数值，保留实测零值，展示监控读取可用性而非集群健康，并在刷新失败时清除节点证据。不安全整数显示 Unknown，避免把舍入结果当证据；这不代表旧版已接入无损 JSON 解码。

109 项前端测试和 `go test ./admin-ui` 通过。新增行为测试在 DOM 桩中执行实际内嵌脚本，覆盖零/缺失/无效/不可用指标，以及先成功后失败的刷新。本轮不是新的浏览器截图审查或发布资格验证。仅修改旧版 `dist/app.js` 与 `dist/index.html` 的读取状态标签，未晋级 React 候选，本轮未重建/部署二进制，也未修改远程环境。旧版 Queue/账户聚合、重叠刷新及外部客户端兼容性仍需独立检查；局部修复不代表旧版已可发布。

### 基于证据的 JetStream 启用状态

`jetstream.enabled` 现为可选布尔字段：缺失表示未知，不是禁用。varz 报告配置对象可证明启用；身份匹配的 jsz 明确报告 `disabled` 布尔值时，可取其反值。仅缺失/null 配置或 disabled 字段不能确定任一状态。配置启用与禁用观测冲突时省略 enabled，将节点降级并报告明确错误，各来源证据仍保留。配置类型错误不能证明启用，不可用/身份不匹配的 jsz 数据不能覆盖有效 varz 证据。

客户端缺失处理语义有所变化：不能把缺失 enabled 默认当作 false。候选界面已有 Unknown 展示，生产旧版/第三方消费者仍需在发布前检查兼容性。OpenAPI 已记录字段可选语义。monitoring/API/app 测试、静态检查、本机构建及真实服务浏览器回归通过（`artifacts/webui-live-KVVxHX/report.json`）。浏览器验证真实 NATS 启用状态，并注入字段缺失/false 检查展示后恢复真实响应；后端测试覆盖 true/false/缺失/null/冲突/类型错误及异节点来源拒绝。未修改前端生产代码、远程服务或内嵌 UI；最终资格验收仍未完成。

### 路由观测完整后才报告对等节点名称

监控后端现先校验 `/routez` 列表存在，且非负 `num_routes` 与原始行数一致，再对名称去重。缺失/null 列表、条数不符、空行/未命名行和仅部分行有名称时，routez 不可用且不返回部分对等节点集合；有效的 varz/jsz 观测保持可用。明确的零条数加空数组仍表示测得的空集合。多个路由连接可能共用对等名称，因此条数校验在去重之前进行。与本节点显示名称相同的对等名称不再被过滤，名称不是稳定服务端身份。

固定版本 NATS 在同一服务端锁内用相同路由遍历器生成条数和列表。monitoring/API/app 测试、静态检查、候选构建及真实服务浏览器回归通过（`artifacts/webui-live-s4FbYY/report.json`）。新增测试覆盖错误/不完整列表、有效零路由、连接池及同名对等节点。OpenAPI 已记录更严格的完整性语义。对等名称仍不证明独立节点 ID、完整成员或健康。未修改前端、上游或远程环境；完整资格验收仍未完成。

### 将监控观测绑定到同一服务端身份

后端现要求 `/varz` 返回非空服务端 ID，且 `/routez`、`/jsz` 的 ID 完全一致，才将其数据附加到节点。固定版本上游的 `Routez` 和 `JSInfo` 均定义了 `server_id`，本次使用既有协议，未修改上游。varz 身份缺失使端点不可用并停止后续读取；其他来源身份缺失/不匹配仅使该来源降级，保留读取完成时间，不附加其对等节点/指标，同时保留其他有效来源。身份一致不证明原子快照、时效性、成员完整性或健康。

受影响的 monitoring/API/app 测试和静态检查通过。本机构建的新 management 候选通过真实 NATS/浏览器回归（`artifacts/webui-live-WVMoy2/report.json`），包含来源指标及不可用状态处理。新增后端测试覆盖缺失/null/空白主身份、routez/jsz 独立不匹配、空白差异及已部分解析的异节点指标不泄漏。OpenAPI 已记录更严格的可用性语义。尝试运行竞态检测，但因本机 CGO 关闭未能执行；未安装编译器或修改远程主机。前端既有未知来源逻辑无需改动。完整功能/视觉资格验收仍未完成。

### 未写入的创建草稿改用其他名称

创建草稿现可选择“保留草稿并选择其他名称”，仅适用于尚未发出请求标识的编辑/审阅/阻止/预览失败/冲突/拒绝状态。原模型、精确原始 JSON（包括无效文本）、预览及错误保留为会话只读记录，新表单全部清空。不会重命名资源、覆盖声明、复制过时预览批准或静默丢弃旧草稿。本会话保留的名称仍禁止复用；这些保留记录暂不支持恢复为编辑器。

提交中/未知/检查中/已接受的请求及更新草稿均不能使用此路径。会话注册表保留旧草稿，因此未保存草稿的清除/离页保护继续生效；明确清除会话仍会移除它们。保留操作和站内导航不发出写入。这取代此前“未写入创建流程尚不能更改名称恢复”的说明；已发出写入的未知结果仍锁定。

107 项前端测试、候选构建及真实服务浏览器回归通过（`artifacts/webui-live-tLRMNy/report.json`）。真实流程产生同名冲突，保留草稿，验证跨导航保留精确 JSON 和 HTTP 409 证据，拒绝复用保留名称，再对其他名称独立预览/确认。只有新资源被创建，原 ETag 不变。已检查 `creation-conflict-recovered.png`。单元测试覆盖全部允许/禁止状态、请求标识门禁及无效文本保留。未修改后端、远程环境或生产内嵌版本；完整功能/设计验收仍未完成。

### 连续执行仅创建 Queue 工作流

创建被接受后，现可选择“创建另一个 Queue”，无需清除会话。已完成模型保留于会话草稿注册表和创建历史展开区，所有输入（包括副本/存储选择）重置为空，焦点移至新名称字段。后续每个资源仍必须独立预览、明确确认并使用仅创建前置条件。重复使用本会话保留的创建名称会明确报错，不再静默重开或替换旧模型，也不覆盖之前的 Queue。

提交中、未知、冲突及编辑状态不能使用此切换，已接受的更新也不能使用。历史保留请求/响应标识、提交文档/Plan 和接受结果，仅支持站内导航保留，不代表健康、收敛或持久化存储。清除会话仍会移除所有保留模型。未成功创建后更改身份属于独立工作流，本实现不丢弃或绕过该状态。

105 项前端测试、候选构建及真实服务浏览器回归通过（`artifacts/webui-live-oFQJuO/report.json`）。浏览器验证空白设置/焦点、同名拒绝且无写入、独立创建第二条、不同请求标识、第一条资源 ETag 不变及导航后历史保留；原有新会话同名冲突测试也通过。已检查 `create-another.png`。未修改后端、远程环境或内嵌生产版本；最终视觉验收与完整功能清单仍未完成。

### 更新接受后继续编辑

更新被接受后，现可明确选择读取最新声明并继续编辑。必须先获得身份匹配的规范文档和有效 ETag 才开启干净的新草稿，绝不递增或沿用旧 ETag/提交响应 ETag。新草稿必须重新预览并单独确认提交。之前已接受请求的标识、提交文档/Plan、结果及响应元数据保留在会话内存展开区及到期保留证据中。这是响应历史，不是当前收敛证明或持久化结果存储。

读取期间锁定重复操作。读取不可用、格式错误或身份不匹配时，保留之前的接受状态和证据，只有手动重试读取成功才允许编辑。清除会话后忽略迟到响应。未知写入不能使用此入口，已接受的创建也不能转换为覆盖；连续创建仍是独立的未完成工作流。

102 项前端测试、候选构建及真实服务回归通过（`artifacts/webui-live-nAGmuI/report.json`）。浏览器注入读取失败、执行真实并发写入、读取新基线，再要求新预览/确认，以不同请求标识完成第二次更新，旧证据仍可查看。已检查 `repeat-edit-accepted.png`。单元测试另覆盖精确整数、无效身份/ETag、重复读取、清除后的迟到响应及创建隔离。未修改后端或远程环境，未晋级内嵌制品，也不代表最终视觉验收。

### 顶栏会话入口与响应式焦点

已验证身份与清除本机会话移入顶栏，移除正文重复行。原生身份展开区保留身份、角色、已验证到期时间及读取策略；Enter/Space 切换，Escape 关闭并返回焦点，外部点击/焦点移出关闭且不抢焦点。受限高度面板可用键盘聚焦和滚动。手机入口换行，中等桌面宽度精简品牌展示。原有草稿、提交中及未知写入的清除保护不变。

98 项前端测试、候选构建、固定数据对照（`artifacts/webui-selected-C2Yr8E/report.json`）和真实服务浏览器回归（`artifacts/webui-live-3KKHHW/report.json`）通过。新增断言覆盖 320/375/900/1440px、矮屏键盘滚动及焦点返回。全流程复现了导航断点竞态：媒体回调前 CSS 可能已隐藏聚焦链接。导航现记录焦点归属、仅在适当时恢复，并验证不抢走外部焦点。中间失败证据保留于 `webui-live-oc1daW` 和 `webui-live-8RITVV`。已检查桌面/手机身份截图；最终原图设计 QA 仍受剩余密度/保真问题阻挡。未修改后端、远程服务或内嵌生产制品。

### 摘要阅读说明与证据层级

四组解释现使用原生展开控件：整页刷新行为、独立 Stream 观测语义、主 Consumer 指标语义、副本解释。收起标题仍保留只读/不可汇总/非健康结论等关键边界。Consumer 身份、全部指标、每一副本行、来源时间、单副本限制及已有失败/不一致/缺 Leader 警告均不折叠。未删减证据字段或权限/变更逻辑。Stream 刷新与标题同行，副本配置数量采用紧凑可换行排列。

98 项前端测试、构建、固定数据捕获（`artifacts/webui-selected-gh56io/report.json`）及真实服务回归（`artifacts/webui-live-FKFBnf/report.json`）通过。截图测试用键盘展开/收起全部四组说明，确认文字可访问且不触发 API 请求。已将同数据桌面截图与此前 `webui-selected-ZscyyF` 一起对照：指标/表格位置前移，但资源标题区和面板密度仍需调整，不能关闭视觉门禁。本步仅改善信息层级，不代表选定原图或完整无障碍验收通过。未修改后端、远程或嵌入生产资源。

### 移动端可访问导航展开区

将移动端主导航横向滚动替换为明确的可展开菜单，并显示当前路由名称。原生按钮提供展开状态和受控内容关系，收起后链接不进入键盘/无障碍导航。Enter 展开、Escape 关闭并返回焦点，正常路由跳转后关闭并将焦点返回可见按钮，后退也会收起；修饰键点击保留浏览器原有链接行为。桌面仍为固定侧栏，跨响应式断点时重置展开状态；若焦点原本位于导航内，将其移至可见按钮或桌面当前链接。权限筛选不变，不冒充模态弹窗或焦点陷阱。

98 项前端测试、构建、同数据捕获（`artifacts/webui-selected-ZscyyF/report.json`）及真实服务浏览器回归（`artifacts/webui-live-ioWWJy/report.json`）通过。新增真实浏览器断言覆盖键盘顺序、隐藏链接、Escape/焦点返回、auditor 入口数量、跳转收起、视口无溢出及双向断点切换；既有流程现明确操作菜单。首次完整运行发现测试在登录完成前判断菜单状态的竞态，已改为等待认证导航。截图还发现移动资源名被操作按钮挤压，标题/操作现分行，并已复核新截图。本步补齐已知移动主导航交互缺口，不代表完整无障碍/设计审查通过。未修改后端、远程或嵌入生产资源。

### 精确时长与明确时区的读取时间展示

Queue 摘要配置现显示精确、易读的时长（如 24h、30s），原始纳秒值仍保留在提示及配置标签的 Plan 中。采用整数运算，不舍入，支持非负 int64 范围，无效/不安全输入显示未知。零仍显示 0s，并保留无时间上限解释；仅改变显示，不重写声明。

Queue 声明、Stream 和主 Consumer 的读取完成时间现使用浏览器解析出的时区，包含毫秒、时区名称及该时刻的 UTC 偏移。原始 ISO 保留在 `datetime`/title 中；偏移可区分夏令时回拨产生的重复本地时间。格式化器只接受读取模块生成的规范毫秒 ISO，审计纳秒事件时间及筛选不变。不把读取时间变成服务端观测时间或健康结论。

98 项前端测试、候选构建、同数据捕获（`artifacts/webui-selected-ThfQJC/report.json`）和真实服务回归（`artifacts/webui-live-450uSu/report.json`）通过。测试覆盖 int64 边界、单纳秒精度、无效值、跨日及夏令时回拨两侧；浏览器核对 24h/30s、固定上海时间及原始 ISO 属性，已检查桌面截图。首次单测发现运行时对 UTC 零偏移文字处理不同，已统一。完整视觉 QA 仍因其他已记录问题未通过；未修改后端、远程或嵌入生产资源。

### 可复现的候选同数据截图

新增独立 `test:selected` 脚本及仅测试使用的固定 API 响应，采用选定图的 Queue、计数、R3/离线成员、声明 ETag 和固定时刻。实际候选构建在中文/operator 状态下运行，捕获 1487 × 1058 视口/全页、局部及移动端截图，并记录原图/构建校验和。不涉及 NATS、真实凭据、代理或写入。测试服务器拒绝所有非 GET 方法及未定义路由；模拟数据位于生产源码导入之外。不复制生成图中的错误语义：Leader 证据保持未知，Consumer 计数明确作用域。

95 项前端测试及选定数据捕获通过（`artifacts/webui-selected-7D2S98/report.json`）。首次尝试在启动服务前发现原图校验和录入笔误，修正值与已归档图片一致。已检查桌面截图，确认固定数值与明确的模拟标注。现已具备同数据截图基础，但 `visualQAPassed` 仍为 false：截图显示纵向密度、时间/时长格式和最终布局仍有明显待办。同数据视觉 QA 及其余原始门槛仍未完成；未改变生产 UI、后端或远程服务。feature-dev 流程指导测试隔离与验证边界。

### Queue 标题读取生命周期与导航保留

Queue 详情现提供“所有 Queue”面包屑和位于资源标题/编辑入口旁的“刷新 Queue 页面”。声明 Plan 版本、原始 ETag 和声明读取完成时间移到标签前，与观测时间保持区别。刷新立即清除旧声明/面板证据，重新读取声明后只挂载当前面板读取；不是原子快照，不执行变更。“所有 Queue”明确打开未筛选列表，浏览器后退恢复历史。已有编辑器草稿独立保留，不因详情刷新而刷新或重定基线。

声明读取改为经测试的可取消、请求代次隔离模块，Queue 身份或 Plan 版本不匹配时不挂载面板。失败移除旧 ETag/读取时间，不把旧数据标成新鲜。Queue 标签链接现在同时携带事件游标和 Consumer 查询；标题刷新不改变 URL，保留两者。

93 项前端测试、候选构建和真实浏览器回归通过（`artifacts/webui-live-5ZWN3E/report.json`）。覆盖注入声明 503 后旧证据清除/恢复且无新增 PUT、面包屑/后退、Consumer 筛选保留、事件游标跨标签/刷新保留，以及详情刷新后已有编辑草稿仍保留。全页截图前现统一回到页面顶部，已检查修正后的桌面截图。feature-dev 流程指导读取模块与测试分离。标题功能已实现，但最终视觉布局、同数据比较和其余设计/发布门槛尚未完成。未修改后端、远程或嵌入生产资源。

### 主 Consumer 作用域摘要与诊断导航

摘要现精确读取声明主 Consumer 的 `(stream, name)`，不取列表第一行、不回退到优先级 Consumer、不求和。三项主要计数为 Stream 存储消息和明确标名的主 Consumer 待投递/待确认。身份链接进入精确详情；双栏下方诊断入口打开完整 Queue Consumer 列表并保留已有查询。存储字节和实际 Consumer 数另行保留。不因身份名称匹配就推断归属或收敛。

Stream 与 Consumer 独立读取，分别刷新并展示完成时间。Consumer 加载、缺失、拒绝、无效或不可用时，不补零、不保留旧计数；大整数完整保留。无效/不匹配的声明身份不会发出 Consumer 查询。导航/卸载取消已有有界读取；先列表再筛选及声明/变更契约不变。

89 项前端测试、候选构建和真实服务浏览器回归通过（`artifacts/webui-live-nPoxKY/report.json`）。浏览器核对真实主 Consumer 身份/计数、精确详情及列表/返回；明确注入 503 和 404 读取响应，验证 Consumer 指标未知而独立 Stream 计数仍可见，随后恢复真实 API。这些注入不是真实 Broker 删除测试。已检查移动端截图，无溢出/双栏断言通过。feature-dev 流程指导复用已有精确读取 API。本步补齐指标作用域/诊断入口功能，不代表选定设计保真、全 Consumer 汇总、自动协调或其余原始门槛完成。未修改后端、远程或嵌入生产资源。

### 选定布局接入：候选外框与辅助配置

候选界面现复用选定原型的深绿导航、250px 侧栏、70px 顶栏、字体回退及固定版本 Tabler 图标。主导航集中管理并标注当前路由组，审计/创建入口继续按权限显示。身份证据可折叠，不隐藏清除会话或改变提交中/未知写入的确认逻辑。Queue 摘要采用 2.04:1 证据/配置双栏，列出六项声明配置；移动端按证据在前上下堆叠。现有真实指标仍是存储消息/字节/Consumer 数，不虚构待投递/待确认汇总。副本观测保留全部字段，使用横向滚动避免列挤压。

87 项前端测试、候选构建和真实浏览器回归通过（`artifacts/webui-live-8Uk6qS/report.json`），包含新增权限导航/身份展开/双栏/移动端断言及原有写入安全流程。未修改后端或嵌入生产资源。image-to-code 与 feature-dev 技能指导原图资源复用和接入验证；[候选设计 QA](../admin-ui/design-qa.zh-CN.md)明确为 **blocked**，并非通过：同数据/状态截图、标题操作区、指定 Consumer 证据、诊断条、内容密度及移动端导航无障碍仍待完成。不替代独立模拟原型此前的 QA，也不代表原始设计/发布需求完成。

### JetStream 节点指标：实测零与未知

修复监控序列化将 jsz 缺失字段补零、却省略明确报告的零值元数据待处理数/集群规模的问题。数值字段现保留存在性：缺失/null 保持省略，明确的零保留，解码/HTTP 失败不暴露部分解码指标。uint64 精确值完整保留。这是有意修正缺失数据响应；调用方不能假设这些可选字段总会存在。

节点详情现逐项标注七个 JetStream 指标，缺失/无效/来源不可用时显示未知，原始观测默认折叠。明确元数据待处理数不是消息积压。移动端截图发现长 Node ID 撑宽标题，已增加标题换行和真实详情页无溢出断言，并复核替换截图。启用状态推断、路由完整性及跨来源身份证据仍是独立待办；来源读取成功不代表完整数据或健康。

验证：`go test ./management/... ./api`、对应 `go vet`、87 项前端测试、本机候选构建及真实服务浏览器冒烟通过（`artifacts/webui-live-p39OZj/report.json`）。六组后端用例覆盖缺失/null/零/部分精确值/HTTP 失败/解码失败；前端覆盖不安全及不可用计数。浏览器核对移动端真实零值和未知元数据字段。未修改远程服务、冻结发布或嵌入生产 UI。完整视觉设计、其余流程和原始发布门槛仍待完成。

### 真实三节点副本回归

可选本机集群脚本现可在三个隔离 NATS 进程上创建 R3 Queue，停止一个实际 Follower，等待服务端真实报告离线，核对浏览器对应行，再使用同一测试进程的数据目录重启并验证两个 Follower 重新同步。首次变更前要求元数据 Leader 一致，并连续三次观测到对等节点已同步；不重试业务写入来掩盖启动问题。

最初运行暴露了测试启动假设和离线期限过短的问题。Windows 强制终止不会发送正常下线通知；固定版本服务端采用 150 秒孤立阈值和 90 秒扫描周期。修正为 270 秒期限后保留真实离线断言；通过的运行在终止约 175 秒后观测到离线。不能把 `offline: false` 解释为当前仍可连接的证明。

86 项前端测试通过。三节点浏览器回归通过（`artifacts/webui-live-dJQUKI/report.json`，23 项检查），默认单节点回归亦通过（`artifacts/webui-live-Aj8QEM/report.json`，22 项检查）。已检查离线截图：状态和身份行正确，但窄表格导致角色及长活动间隔换行不佳，仍属待改视觉问题。本次覆盖空 Stream 的 Follower 失联/重新加入，不代表负载下消息持久性、Leader 切换、网络分区、原生 Linux 或发布资格验收。本步仅修改测试脚本和双语记录；未触碰已有服务、远程验收、后端及嵌入生产资源。

### 副本证据展示

Queue 摘要和 Stream 详情现共用副本证据组件：展示观测配置副本数、可用时单列声明副本数、报告的 Leader/Follower 行、同步/离线标记、精确操作落后量和活动间隔。只有 Leader 元数据时不补造已同步、在线或零落后值。无拓扑、缺 Leader、行数与配置不一致、重复身份均有明确状态，不虚构未知成员名称。副本名称不当作稳定 Node ID。单副本配置明确不具备副本故障容错。这些观测均不证明法定人数、健康、所有权或发布资格。

86 项前端测试、候选构建和真实单节点浏览器冒烟通过（`artifacts/webui-live-PzVPOE/report.json`）。单元测试覆盖含离线副本的三成员、精确大整数落后量、缺 Leader/拓扑及重复身份；真实浏览器核对单副本限制。不代表真实三节点故障/Leader 切换验收。选定视觉布局、跨来源节点身份定位、完整声明/观测诊断、写入流程和全部其余原始门槛仍待完成。本步未更改后端、远程或嵌入生产资源。

### Queue 详情五面板接入

Queue 详情现提供摘要、配置、路由、消费者、事件五个可导航面板。摘要精确读取实际 Stream；配置展示规范声明与原始 Plan；路由区分声明输入 Subjects 和优先级存储 Subjects，并展示只读绑定映射；消费者保留先列表后筛选的流程；事件读取按资源名关联的管理审计窗口，并提供完整筛选/导出入口。不是消息投递历史或自动结果归因。审计权限独立控制。只挂载当前面板的相关读取，退出时取消过期读取。

面板状态与 Consumer 筛选保存在 URL 中，事件窗口使用精确 `ebefore` 游标。裸详情链接现默认打开摘要；已有 Consumer 筛选 URL 仍打开消费者。面板使用原生导航链接，不冒充未实现键盘语义的 ARIA 标签控件。事件窗口与摘要仍是独立观测，不代表健康或收敛。

83 项前端测试、候选构建和真实服务浏览器冒烟通过（`artifacts/webui-live-TL8Goc/report.json`）。首轮测试错误地期待优先级 Stream 使用声明输入 Subjects；核对真实映射后，路由明确展示两者，修正测试通过。浏览器遍历五面板并回归 Consumer 详情/返回、编辑及服务端失联。完整副本诊断摘要、引导配置展示、危险区/删除流程、最终选定视觉及剩余验收仍待完成。未修改远程或嵌入生产资源。

### 受控审计 JSON 导出

审计现可按已提交筛选准备证据导出，明确从最新保留记录开始，而非当前页游标或未提交输入。逐个读取并校验窗口，包含无匹配的窗口；只有达到观测到的保留下界后才提供下载。JSON 保留精确整数、初始最高序号、各窗口游标、筛选回显、读取时间、缺失位置和覆盖限制。不是原子/完整历史快照或操作结论；遍历期间保留策略可能移除记录，初始上界之后新增的记录不包含在本次导出中。

浏览器上限为 256 个窗口或 16 MiB 序列化证据。超限或读取/校验失败不提供部分下载。取消、查询/导航变化和卸载会中止当前读取并抑制延迟完成；下载 URL 在替换/卸载时撤销。已下载文件独立于会话清除而保留，需安全保管。大规模/服务端导出仍为单独待办，不以静默截断冒充成功。

82 项前端测试、候选构建和真实服务浏览器冒烟通过（`artifacts/webui-live-W9A1gP/report.json`）。测试覆盖空窗口继续、固定初始上界、忽略当前页游标、失败/超限及取消；浏览器证据包含下载后解析的真实筛选事件 `audit-export.json`。其余视觉、工作流和发布要求仍待完成。未更改远程或生产部署。

### 审计事件时间范围

全局审计窗口及 UI 现支持可选 `from`（包含）和 `until`（不包含）。采用明确带时区、最多九位小数的 RFC3339 格式；同时指定两端时必须严格递增。Go 比较原始事件时间，JavaScript 按精确纪元纳秒校验比较，不舍入为毫秒。有时间条件时排除未知/零值事件时间。时间条件与原精确筛选共同生效；扫描仍按序号推进，不假设时间戳单调。提交条件重置游标，URL 历史保留边界。无效日期/区间在后端枚举前被拒绝。

79 项前端测试、`go test ./management/... ./api`、对应 vet、候选构建和真实服务冒烟通过（`artifacts/webui-live-onAox1/report.json`）。测试覆盖相差一纳秒的边界、时区等价和非法日期/时区；浏览器覆盖时间/资源/阶段组合筛选及返回恢复。OpenAPI 已同步。导出、更完整导航、最终视觉和其余全部原始验收仍待完成；未更改生产或远程部署。

### 审计页面接入

候选版已提供 `/admin/audit`，按 `audit:read` 权限展示，使用全局窗口接口。支持六项精确筛选、重试当前窗口、明确从最新开始、读取更早窗口，以及展开原始事件证据。筛选与无损 uint64 游标保存在 URL 中，提交筛选重置游标，浏览器返回恢复查询状态。扫描/缺失/匹配计数明确仅属于当前窗口，不是匹配总数；空窗口仍可能有更早游标。到达保留下界或不存在存储均不能证明未执行操作。读取失败清除旧结果，过期响应不能恢复已清除或已导航离开的状态。共用序号校验器支持全局窗口，但不放宽写入证据的精确请求检查。

78 项前端测试、候选构建和真实服务浏览器冒烟通过（`artifacts/webui-live-QiDO5B/report.json`）。浏览器覆盖 auditor 身份、资源/阶段组合筛选、展开实际事件、空匹配窗口及返回恢复；单元覆盖大整数游标、非法查询、筛选回显不匹配及过期读取。时间范围、导出、更丰富事件导航、全局窗口多页浏览器覆盖、选定视觉及全部原始验收仍待完成。候选版尚未替换嵌入生产资源或远程环境。

### 全局有界审计窗口

新增 operator/auditor 可用的 `GET /api/v1/audit/windows`，支持排他 uint64 `before` 游标及区分大小写、精确且同时满足的 `requestId`、`resource`、`actor`、`phase`、`action`、`outcome` 筛选。空筛选值不施加约束。字符串最多 256 个 UTF-8 字节且不能含控制字符；phase 为 空/intent/outcome。未知、重复或编码错误的参数在访问存储前失败。原请求证据接口共享同一扫描内核，契约不变。每窗口最多扫描含缺口在内的 256 个序号，响应提供保留边界、扫描/缺失计数、筛选回显和 `nextBefore`，不返回匹配总数。空匹配窗口仍可能继续。新记录需要重新开始遍历，保留策略可能移除旧证据。读取不创建审计存储，也不进行操作结果归因。

管理服务/API 测试、vet、本机候选构建及真实服务冒烟通过（`artifacts/webui-live-cmx5o5/report.json`）。测试覆盖 300 条记录中首窗口无匹配后继续、翻页间新增记录、精确筛选、非法/未授权查询、uint64 游标边界及损坏记录。真实 auditor 访问验证已提交但响应未知写入的审计事件按请求/资源/阶段组合筛选。OpenAPI 已记录新路由；最初契约测试正确阻止了未文档化路由，补充后通过。这是有界扫描，不是已建索引或通过性能验收的查询。审计页面接入、时间范围筛选、导出、完整历史/保留策略体验和所有其余原始要求仍待完成。未更改远程部署。

### 原全局审计 offset 正确性

修复 `ListAudit` 将 offset 当作序号位置、而非保留记录数的问题。存在序号缺口时，原实现可能在后续页重复返回之前的记录。现从观测到的最高序号向后扫描，按实际保留记录跳过。非法分页被拒绝；取消、解码失败或事件身份缺失时不返回部分证据页。受保护审计存储策略不变。

`go test ./management/... ./tools/rjsctl/...` 及对应 `go vet` 通过。新增测试覆盖三页之间的内部序号缺口、越界读取、取消扫描、损坏/缺身份记录与非法参数。注入缺口时明确同步测试替身的保留记录计数。这是后端测试替身验证，不是真实保留策略竞争验收。offset 深页现需要正确扫描更多记录，仍受调用方超时约束；并发新增/过期仍可能使不同请求间 offset 移动。稳定的全局游标/过滤查询、完整 Audit UI 及其余原始发布要求仍待完成。未执行部署。

### 浏览器到期证据保留验证

真实服务测试脚本新增 `RJS_TEST_SESSION_EXPIRY=1`。仅对一次真实认证后的会话响应注入未来到期时间，并推进在应用初始化前安装的浏览器时钟；资源读取、写入和审计仍为真实服务。写入已提交但响应不可读、进入结果未知状态后，到期会隐藏所有写入/检查控件，同时在只读视图保留原始草稿、请求 ID、提交计划和检查证据。取消清除确认会保留证据，确认清除才会移除。不发出额外 PUT。

可选场景通过，证据为 `artifacts/webui-live-eDT17Q/report.json`。首次 `webui-live-s6xQZN` 因测试在应用初始化后安装时钟及告警定位不唯一而失败；脚本已修正并重跑。此为注入到期元数据的浏览器生命周期证据，不是签发方真实 OIDC 到期验收。本步未更改生产代码，原始剩余范围仍待完成。

### 已验证凭据到期

具有已验证到期时间的会话现在会进入锁定的到期视图。有界计时器检查截止时间，API 也会在发送前立即检查，避免后台计时器延迟时继续发送请求。到期会移除 Token、阻止新请求，并在内存中保留身份及调用方持有的草稿/请求证据。只读保留草稿视图继续订阅状态，已发出的写入可以完成并展示结果；到期不表示取消、回滚、重试或结果已确认。换身份登录前，仍执行原有清除确认与提交中保护。不为未知/null 到期时间虚构期限。客户端检查是服务端鉴权的补充，不替代服务端鉴权。

75 项前端测试及候选构建通过。确定性测试覆盖计时到期、身份保留、Token 移除、旧计时器失效，以及后台计时器延迟时不会发起网络请求。真实服务浏览器回归通过（`artifacts/webui-live-E2ZzN7/report.json`），使用静态凭据，不代表真实 OIDC 到期或时钟偏差验收。自动角色/撤销重验证、续期认证后的同身份草稿恢复、OIDC 浏览器到期验收和完整生命周期体验仍待完成。未更改远程或生产部署。

### 总览来源接入

React 候选版已提供 `/admin/overview`，独立读取管理服务/账户、Queue 声明和已配置监控来源。成功读取分别记录完成时间；失败仅清除对应来源的值。Queue 数使用单行分页响应中的服务端完整集合总数，不使用已加载行数。账户 Stream/Consumer 总数与声明数、待投递数明确区分。监控问题优先于统计展示，并提供资源列表入口；端点覆盖范围不等于完整自动发现成员或健康结论。不推断速率、历史图表、HA 资格、已发送告警或部署规格。

73 项前端测试及候选构建通过。真实服务冒烟（`artifacts/webui-live-JDb9tx/report.json`）验证三类来源及服务端停止场景：账户数据不可用保持错误，监控区仍保留端点不可达证据。单元测试覆盖完整总数、精确大整数、独立失败及过期响应抑制。`/admin/` 仍保留原 Queue 列表入口。声明部署规格/能力、近期审计、完整问题钻取、选定视觉布局及所有剩余发布要求仍待完成，不代表 S-01 全部验收通过。本步未更改后端或远程部署。

### 节点标量存在性与指标展示

varz 内存、CPU、核数、连接数、订阅数、慢消费者数和累计收发计数现通过指针解码与 JSON 序列化保留字段存在性。明确零值仍为 JSON 零值；缺失/null 字段保持省略，并显示未知。来源读取成功不再使这些省略的标量变成零。节点详情新增这些指标、Go 版本、集群名称和已连接对等节点的本地化文案；明确累计消息数不是速率或待投递数。对等节点仅在 routez 读取成功时展示，不据此推断完整成员或法定人数。

`go test ./management/...`、`go vet ./management/...`、70 项前端测试、候选服务/UI 构建及真实服务浏览器冒烟通过（`artifacts/webui-live-NH3K0S/report.json`）。新增序列化测试对比字段省略与明确零值；浏览器验证真实慢消费者零值在中英文下均正确显示。jsz 字段级完整性、结构化指标/副本视图、其余页面、最终视觉及发布门槛仍待完成。未替换远程或生产环境。

### 节点浏览与来源可用性

候选版已接入 `/admin/nodes` 和 `/admin/nodes/:node`，按观测到的服务端 ID 定位，不使用名称或列表序号。读取的是完整已配置监控端点集合，不是自动成员发现。不可达端点仍在列表中，不虚构 ID。详情重新读取时缺失或重复的 ID 不能证明节点已删除：监控不可达、服务端重启或端点重复配置均可能导致无法唯一定位。

监控响应新增 `sources`，记录已尝试的 `varz`、`routez`、`jsz` 读取，各含 `available` 和管理服务侧完成时间 `read_at`。成功表示 HTTP/JSON 解码成功，不代表字段完整或节点健康；未尝试来源不出现。原有服务端观测时间仍独立保留。jsz 不可用时 UI 隐藏其指标，不显示结构体零值；缺失的标量指标保持未知。节点状态文案明确描述监控可用性，不描述健康。刷新失败会清除旧观测。

验证：70 项前端测试、监控/API Go 测试及 vet、本机候选构建、真实服务浏览器冒烟通过（`artifacts/webui-live-YcMOxH/report.json`）。覆盖精确 ID 路由、异常响应、来源失败及尝试时间、过期响应抑制、真实节点详情/返回，以及服务端停止后保留不可达端点且不保留旧 Node ID。OpenAPI 已记录新增来源契约。最终布局、完整本地化指标展示、副本及对等节点钻取、全部其余页面和发布门槛仍待完成。未替换生产资源或远程环境。

### Stream Consumer 服务端查询

候选版已补齐此前缺失的 Stream Consumer 查询控件：名称或实际过滤 Subject 搜索、Pull/Push 模式、名称排序和每页条数。所有筛选和排序均在服务端分页前执行。变更查询会回到第一页，深链接和浏览器返回保留查询条件。筛选后为空不会描述成整个 Stream 没有 Consumer。非法查询和后端失败会清除旧结果，不转换成空成功。

`GET /api/v1/streams/{stream}/consumers` 支持 `q`、`mode`、`sort=name`、`order`、`offset`、`limit`。搜索为去除首尾空白、忽略大小写的字面子串，规范化后上限 256 个 UTF-8 字节；复数过滤 Subject 字段优先于旧单数字段。未知、重复或编码错误的参数现在返回 400，不再静默忽略（兼容性变化）。枚举沿用五秒上下文，失败不返回部分页。仍是完整枚举，不代表已有索引或通过规模验收。OpenAPI 已同步。

验证：66 项 Node 测试、`go test ./management/internal/api ./api`、`go vet ./management/internal/api ./api`、本机候选服务及 UI 构建、真实服务浏览器冒烟均通过（`artifacts/webui-live-juqeuO/report.json`）。后端测试覆盖 201 个 Consumer、Subject/模式查询、先排序后分页、后端原切片不变和失败场景。浏览器覆盖从第二页搜索后回到第一页、Push 模式空结果、恢复 Pull、排序、精确详情跳转及返回。本节取代下方记录的仅分页限制。未执行远程操作或晋升生产静态资源，完整剩余范围仍待完成。

### Stream 资源浏览候选实现

React 候选版现已提供 `/admin/streams`：服务端名称搜索、排序、分页，以及精确 Stream 详情、分页观测 Consumer 列表和精确 Consumer 详情链接。列表查询和 Consumer 页位置保存在 URL 中，浏览器返回可恢复。配置与观测状态分开展示；配置副本数不代表在线副本数，存储消息数不代表 Consumer 待投递数或健康状态。详情与 Consumer 成员分别读取，各自保留完成时间和错误状态。缺失计数显示未知，大整数计数保持精确。

复用现有后端接口和 Queue 列表生命周期。Stream Consumer 接口目前仅支持分页，因此未提供搜索或模式筛选控件。此列表包含实际存在的 Consumer，不包含缺失的声明项。选定最终布局、完整 Stream 配置及副本展示、其余页面、写入结果恢复、嵌入资源晋升和发布门槛仍未完成。未更改远程主机或已有演示实例。

验证：`cd admin-ui; npm.cmd test` 通过 65 项测试；`npm.cmd run build` 通过。`npm.cmd run test:live` 在本机隔离真实服务上通过，证据为 `artifacts/webui-live-S8JM2y/report.json`。浏览器覆盖 auditor 的 Stream 搜索、观测详情、Consumer 跳转及返回，并回归既有创建、编辑、写入结果未知和断连场景。首次执行因测试遗漏返回 Queue 详情而失败；脚本已补齐导航，并显式设置 15 秒操作超时。不代表全部页面视觉或规模验收通过。

### 仅创建 Queue 流程

Operator 创建现要求明确填写名称、Subject、副本数（1/3/5）、存储类型及正 int64 消息数上限，然后准备会话持有的 JSON 草稿。不根据节点数猜测 HA 能力或暗示生产资格。名称/副本/存储/基本整数约束与后端一致；完整 Subject/拓扑校验和省略字段默认值仍由服务端预览负责。准备前的表单值及已准备草稿在站内导航中保留，未保存输入在清除会话/刷新时有警告。高级配置仍通过 JSON，这不是完整引导表单。

创建 preview/apply 始终使用 `If-None-Match: *`，不会获取已有 ETag 来覆盖同名资源。创建冲突不能使用编辑变基 UI。复用已有确认提交、未知结果锁定和只读检查保护。目前单个活动创建表单绑定草稿，直到清除会话；多创建草稿管理尚未完成。

60 条 Node 测试、候选构建及真实浏览器冒烟通过（`artifacts/webui-live-Ccxilp/report.json`）。测试创建合法名称 `new`，证明预览仍不存在声明，确认单副本创建，再次同名尝试并验证存储 ETag 不变且无变基/覆盖控件。此前读取/编辑/冲突/故障检查也通过。完整引导配置、删除、其余资源页、选定视觉布局及发布门禁仍待完成。

### 审计证据续页

未知结果检查现支持明确读取更旧窗口，包括当前匹配为空的情况。成功窗口分别保留保留范围/扫描数/缺口/时间字段；读取失败保留已取得证据和游标。严格校验游标与事件，保持 uint64 精度并阻止不递进遍历。不据此判定结果、变基或解锁写入。57 条 Node 测试、候选构建及真实分页冒烟通过（`artifacts/webui-live-BFP2Ya/report.json`）。完整审计页、可扩展索引和结果判定仍待完成。

### 有界请求审计证据

已实现并接入[请求关联审计窗口](webui-audit-request.zh-CN.md)。已有存储不变，每次认证调用最多扫描 256 个序号位置，明确返回游标、保留边界及缺口。未知结果检查读取首窗口，不归因、不解锁写入。相关 Go 测试/vet、55 条 Node 测试、构建及真实连接重置冒烟通过（`artifacts/webui-live-ctJ2bf/report.json`）。索引化审计性能、更旧窗口 UI 和持久结果判定仍待完成。

### 修复传输层重发导致的结果歧义

已通过代理证据复现此前响应头之前断连失败：同一请求标识两次到达真实后端，第一次返回 200（响应被丢弃），第二次返回 409。因此最后的错误状态不能证明原始写入没有生效。提交失败现保持未知，保留原始草稿/ETag/请求证据，不允许直接按普通冲突变基，即使收到结构化拒绝也如此。只读预览冲突仍支持明确三方恢复。最后 HTTP 状态单独展示，不当作原始请求的结果。这是保守结果处理，不是服务端去重或持久结果查询。

55 条 Node 测试和候选构建通过。真实断连运行 `artifacts/webui-live-D8mqwP/report.json` 验证 200→409 透明重发后仍保持未知锁定；该运行旧版检查标签写作“one-put”，但具有决定意义的 `responseFault`/`faultRequests` 明确记录两次 PUT，脚本标签和计数断言随后已修正。默认截断 JSON 回归 `artifacts/webui-live-l9vqf4/report.json` 通过，检查阶段没有额外 PUT。完整幂等/结果恢复、浏览器矩阵和其余产品/发布范围仍待完成。

### 明确确认提交与未知结果检查

候选编辑页仅在当前非 noop 预览及明确确认后开放提交。编辑、变基及新预览都会清除确认。提交同步锁定编辑器，提交中拒绝清除会话。成功标为已接受，不代表已收敛。不可读/未知结果保留精确草稿、原始 ETag 和请求标识，禁用后续写入，并提供声明/前 200 条 Consumer 的只读证据。检查不归因成功、不变基、不解锁写入。明确清除会话会警告丢弃未决证据，不声称取消或回滚操作。应用层不自动重试。

55 条 Node 测试及候选构建通过。真实服务运行 `artifacts/webui-live-IYh5p7/report.json` 验证浏览器确认提交，以及后端已提交但 JSON 响应被故意截断：编辑器保持未知/锁定，检查仅读取当前状态。首次运行因测试定位器歧义失败；另一次响应头之前断开连接的注入未产生预期未知状态，仍需调查传输层（可能涉及浏览器重试），不能以已通过的 JSON 截断场景代替资格验证。审计关联判定、完整网络故障矩阵、删除、创建、设计和发布门禁仍待完成。候选版本尚不具备发布资格。

### 候选编辑器的明确冲突恢复

冲突 UI 现分别读取和展示原始基线、本地草稿及最新规范声明。最新 ETag 仅作为证据，直到 operator 提供并确认合并文档。合并框从本地文本开始，明确不是自动合并。修改合并稿会重置确认；无效 JSON/身份阻止采纳。确认后仅更新本地草稿/基线，旧预览失效，必须重新预览。再次预览清除旧比较证据和确认。完整安全流程接齐前，apply/delete 仍不可用。

54 条 Node 测试、构建和真实服务冒烟测试通过（`artifacts/webui-live-c2TPw8/report.json`）。真实测试在编辑器捕获 ETag 后从另一调用方修改声明标签，验证冲突，确认保留并发标签的合并稿，再次预览，并证明持久化 retention 设置未被覆盖。此前读取/角色/草稿清除/broker 故障检查亦通过。未进行生产/远程部署；完整变更、创建、设计和发布要求仍未完成。

### 规范草稿编辑器与真实预览

具备 `queue:preview` 的 operator 身份可从 Queue 详情打开编辑路由。会话持有的控制器加载规范文档和原始 ETag，提供无损 JSON 编辑，并调用已有只读预览状态机。无效 JSON 或身份重命名立即使旧预览失效。站内导航时草稿仅保留于内存；清除含修改草稿的会话需确认，刷新/关闭请求浏览器未保存警告（浏览器策略可能限制警告）。候选编辑器尚不提供 apply/delete 按钮或变更请求。冲突、不可用或不可编辑状态不会静默替换原始基线。

53 条 Node 测试、候选构建及隔离真实服务冒烟测试通过（`artifacts/webui-live-ttHLni/report.json`）。浏览器证据覆盖 operator 预览时持久声明保持不变、路由切换保留草稿、无效 JSON 禁用预览、取消/确认清除会话。此前 auditor/读取/broker 故障检查也通过。引导式创建/表单、明确三方冲突恢复、未知写入处理、安全提交/删除、选定布局及完整发布门禁仍待完成。

### 首次候选真实服务门禁

新增[隔离真实服务冒烟测试](webui-live-testing.zh-CN.md)，使用本机构建的 NATS/管理服务、同源候选服务器与 Chromium。当前读取流程已有 `artifacts/webui-live-BStzaL/report.json` 真实后端证据：优先级集合、精确详情、auditor 授权、只读 preview 和 broker 故障行为通过。测试自有进程均已退出，专用证据/数据目录保留。截图复核另修复了移动端表格压缩并折叠原始 Plan。这取代对应检查仅有模拟证据的状态，不代表其余完整工作流/设计/发布门禁通过。

### Consumer 精确详情与集合历史

候选集合身份现已链接到 `/admin/streams/{stream}/consumers/{consumer}`，仅读取精确 API 资源，不通过枚举某一页查找。详情展示模式、过滤条件、单 Consumer 计数、ACK 设置和无损原始观测，不声称健康或 Queue 汇总。手动刷新加载时清除旧数据；缺失、拒绝、无效响应和不可用保持区分。卸载抑制迟到响应。集合中的缺失行也可打开并重新查询，不伪造观测对象。

Queue 详情 URL 现保留 Consumer 的 `cq`、`cmode`、`corder`、`coffset`、`climit`；筛选变化重置分页，历史恢复保留明确偏移。从 Consumer 详情通过浏览器后退恢复集合查询。没有来源集合也能直接打开详情链接。参数不包含凭据或草稿。最终标签页、更完整的详情布局、直接返回入口及真实服务门禁仍待完成。

49 条 Node 测试及候选构建通过。已构建页面的 Chromium 模拟 API 验证筛选集合第二页 → 精确详情 → 404 刷新清除旧观测 → 后退恢复搜索、模式和偏移。测试另覆盖同名不同 Stream 身份拒绝、Pull/Push 变化及不可用与缺失区分。不声称已部署或 WebUI 整体完成。

### Queue Consumer 集合接入

候选声明视图已先加载 `/api/v1/queues/{queue}/consumers` 集合，再提供筛选。支持服务端名称/Subject 搜索、Pull/Push 选择、名称排序和分页。每行区分声明成员、观测存在性和所有权；期望但缺失的 Consumer 保留声明配置，指标为未知；外部观测 Consumer 不伪造声明。整数计数精确保留，属于单个 Consumer，绝不汇总当前页。响应形状/身份/模式/过滤字段校验拒绝不完整或矛盾的分页。读取失败/冲突清除行与观测时间。集合版本/读取时间独立于声明证据；声明 ETag 缺失或不一致时明确提示不是同一快照。

44 条 Node 测试及候选构建通过。已构建页面的无头 Chromium 模拟 API 检查验证缺失计数保持未知、外部行、服务端筛选、版本不一致、冲突清除及 375px 页面无横向溢出。这不等于真实服务验证。Consumer 精确详情导航、URL 保留 Consumer 筛选、最终标签页/布局接入及完整生产门禁仍待完成。未改变嵌入式或远程制品。

### Queue URL 状态与声明导航

候选版本已使用基于 History 的 Queue 路由。搜索/排序/偏移/每页条数可通过前进后退和刷新恢复；改变筛选重置偏移，明确恢复历史时保留偏移。查询 URL 拒绝未知/重复参数及无效分页。`/admin/queues/new` 保持独立的未实现创建入口；`/admin/queues/by-name/new` 读取名为 `new` 的 Queue。链接支持常规组合键点击，凭据和草稿不写入 History 状态。

详情通过共用 API 发起有超时边界的精确声明 GET，分别保留 Plan 版本和原始 ETag，卸载后抑制迟到响应。这是临时声明回读视图，不是选定的完整 Queue 详情布局、观测健康、Consumer 集合或编辑器。未知/未实现路由不会自动创建或修改资源。浏览器后退保留来源列表状态，明确的 Queue 列表链接则打开新列表。旧版 hash 别名及其他页面路由仍待完成。

39 条 Node 测试及候选构建通过。已构建页面的 Chromium 模拟 API 检查验证 URL 筛选、`new` 身份、精确大整数 ETag、前进后退和刷新后重新认证并保留详情 URL。真实后端、完整无障碍/设计保真及生产嵌入式切换尚未验证。

### 生产候选版本：Queue 集合

已认证的 React 候选页面通过共用无损 API 客户端接入真实 `/api/v1/queues` 契约。搜索、名称排序及每页条数均由服务端查询，变化时重置偏移，不对已加载页做全局筛选。加载/错误状态清除旧结果，无效/不完整分页不展示，迟到响应不能覆盖新查询或已清除会话。集合并发减少造成的当前页为空与零匹配资源不同，并可返回上一页。行仅展示声明名称和 Plan 版本，不推断健康，也不把 Plan 版本当作写入用 KV ETag。

包括 Queue 查询模块在内的 35 条 Node 测试通过，Vite 候选构建通过。无头 Chromium 使用合成同源 API 验证已构建页面的登录、第二页、第 201 项搜索重置偏移、503 与空结果区分、清除会话卸载及 375px 无页面横向溢出。这不证明真实 NATS 集成、规模性能、完整无障碍或选定设计保真度。详情导航、完整列表列/URL 状态、其他页面和嵌入式切换仍待完成。

### React 生产候选版本与会话基础（2026-09-10）

`admin-ui/src/main.jsx` 已采用 React/Vite 和无损 API 访问层，通过 `/api/v1/session` 验证 Bearer 身份。已实现中英文登录、明确清除内存、服务端验证的身份/角色/到期时间及凭据/角色/禁用/不可用的不同状态。提交时重置 Token 输入，不使用浏览器存储、Cookie、URL Token、夹具身份或自行解码 JWT 推断权限。代次检查抑制清除或新登录后的迟到成功/失败响应。这是会话基础，不代表到期生命周期、资源认证失败恢复或编辑器/会话集成已完成。

API 访问层、Queue 编辑器、旧版并发和会话共 29 条本地 Node 测试通过。输出目标为 `admin-ui/build-candidate/`；嵌入式 `dist/`、发布打包和远程制品不变。见[构建说明](../admin-ui/README.zh-CN.md)。生产页面、同源浏览器测试、选定设计对照及嵌入资源切换仍未完成。单独启动 Vite preview 没有 API 时必须如实验证失败。

显式调用项目内 Vite 6.4.2 的候选构建通过（此前意外调用父目录 Vite 7 的结果不算资格证据）。无头 Chromium 使用合成同源 session 响应验证已构建页面：正确 Bearer、已验证身份、清除内存、不写浏览器存储/Cookie、语言切换及 375px 无横向溢出，页面无错误。这是有限的模拟接口浏览器检查，不是完整 UX/无障碍或真实服务门禁。首次 npm 镜像安装仍在运行，锁文件及干净安装证据待补。

随后安装成功结束（65 个包），已生成 `package-lock.json`。`npm ci --ignore-scripts --offline --no-audit --no-fund`、`npm run build`（Vite 6.4.2）及 `npm test`（29 条）均通过，取代上文安装待完成状态。干净安装后的候选资源哈希未变化。

| 工作 | 当前证据 | 仍需提供的完成证据 |
| --- | --- | --- |
| S-01–12 生产页面 | 旧内嵌控制台及独立的选定设计夹具原型 | 在生产资源流水线实现全部规划页面；同源真实 API、深链接、历史导航、缺失/错误状态、响应式中英文 UI |
| C-01 访问/会话 | 已有角色检查；已验证身份/权限接口已实现；资源读取匿名 | 非回环默认策略决策、读取强制校验、生产会话 UI、内存凭据和明确清除 |
| C-02 能力/规范编辑数据 | 已有计划/解析器；无损可编辑 Queue 回读已实现 | 明确部署意图、支持/已验收范围分离、完整版本化 Schema、真实编辑器接入 |
| C-03 全局列表 | Queue 关联 Consumer 及 Queue/Stream 名称筛选/排序后分页已实现；已有 201 条资源测试 | 生产 UI 页码重置/取消及指定环境规模验收；全局枚举的索引/有界成本工作 |
| C-04 观测 | 已有资源和监控模型 | 必要证据、逐来源时间/错误、陈旧/缺失/未知/健康区分及实际 UI 映射 |
| C-05 写操作 | 已有条件写入、审计和原型恢复；只读预览现已在源码实现 | 真实编辑/删除全过程原始 ETag、表单/JSON 一致、类型化阶段/副作用、真实服务不确定结果恢复 |
| C-06 关联 Consumer | 精确详情和 Queue 关联集合已实现/测试 | 生产列表/详情接入、观测/预期与归属展示、导航/筛选恢复、真实环境验收 |
| C-07 审计 | 已有分页意图/结果记录 | 有界关联查询、筛选/游标、精确详情、保留空洞和缺失结果 UI |
| 共用 API 访问层 | 无框架依赖源码及 7 条 Node 测试 | 接入生产运行时并验证真实服务响应；模块测试不等于集成 UI 门槛 |
| 发布和质量门槛 | 现有 Go 测试；仅原型浏览器证据 | 生产 Chromium/Firefox、无障碍、375/1440 布局、五条操作旅程、混合失败、写入安全、指定负载环境指标、双语运维文档和候选版回归 |

WEB-001–016 详细验收仍以需求文档为准；每一项具备当前生产证据后才能通过 P0。不得以模拟数据、路由清单检查或通过的单元测试证明更大的业务流程。

## 架构决策点

- D-04：负责人已确认 React/Vite 构建 Go 内嵌静态资源，不增加运行时前端服务/CDN。下一步生产接入；原型夹具不是可用控制台。
- D-05：负责人已确认默认读取认证，仅保留显式字面量回环演示例外。源码已实现，见[访问策略与兼容性](webui-access.zh-CN.md)，尚未部署。下文迭代历史中此前“待决”描述以本确认记录为准。
- 选定 Queue 详情布局仍为视觉目标。构建留在本机；本记录不授权远程源码/构建/部署或修改冻结验收制品。

## 本轮进展

新增[共用 API 访问层](../admin-ui/src/api.mjs)和[测试](../admin-ui/tests/api.test.mjs)，未改变现有生产资源入口。保留 HTTP 状态、响应头、ETag/关联标识、结构化/原始错误正文，包括操作阻断响应。凭据仅在闭包内存中保存。请求限于同源版本化 API，拒绝重定向，不自动重试。取消/失败/无法解析的写响应保守标为结果不确定；这不能代替后端阶段/副作用语义。

JSON 解码在 Go int64/uint64 整数超出 JavaScript 安全范围时保留为 BigInt；编码拒绝已经舍入的不安全 Number。拒绝错误/过深嵌套 JSON 和不安全的指数表示整数。最新读取序号机制确保传输层即使忽略取消，迟到响应也不能覆盖新资源。请求超时及会话凭据清除均有测试。

```sh
node --test admin-ui/tests/api.test.mjs
```

结果：本地 7 条测试通过。本轮不声称已完成浏览器渲染、真实 NATS 集成、部署或整体目标。下一步：确定 D-04/D-05，建立正式应用流水线，再接入 Consumer 路由并完成其余契约/页面，补齐全部验收证据。

### 后续进展：只读预览

已实现 [Queue 预览](webui-preview.zh-CN.md)，包含共享 apply 前置条件/DLQ 校验、仅 operator 可用的 HTTP 路由、原始版本检查、无副作用测试和 OpenAPI。相关测试及静态检查通过。全仓回归在无关的裸机生命周期测试中遇到 Windows 临时端口绑定失败，未停止已有监听。UI 工具链/访问策略选择仍待确认，但独立后端开发已取得进展。

重跑：`go test -count=1 ./tools/baremetal-run` 通过（42.071 秒），未改动主机。这是失败包重跑通过，不代表原全仓命令成功。

### 后续进展：无损 Queue 文档

已实现[规范可编辑回读](webui-queue-document.zh-CN.md)。返回文档前必须通过完整 Plan 往返检查；不支持的旧声明保持可查看，但不能变成有损草稿。原响应字段/ETag 不变。四个相关 Go 包、七条共用访问层 Node 测试和 `go vet ./...` 通过。生产页面入口及 D-04/D-05 不变；这是后端进展，不是 UI 集成完成。

### 后续进展：已验证身份

已实现[会话身份](webui-session.zh-CN.md)，复用授权和已验证的 OIDC 过期时间，不创建 cookie 或服务端会话。API/认证/契约测试和全仓静态检查通过。匿名资源读取策略和前端工具链保持不变，等待负责人决策。这完成了身份读取部分，不代表完整 C-01 安全策略或生产会话 UI 完成。

### 后续进展：全局 Queue/Stream 查询

`GET /api/v1/queues` 和 `/api/v1/streams` 现支持 `q`（去首尾空白、不区分大小写的名称字面子串，规范化后最多 256 UTF-8 字节）、`sort=name`、`order=asc|desc`。先筛选、排序，再执行原 offset/limit 分页；total 为完整筛选结果总数。空结果为 `[]`；offset 超过总数时截到总数。示例：`/api/v1/queues?q=orders&sort=name&order=asc&offset=0&limit=50`。

兼容性变化：这两个路由对未知/重复/编码错误参数返回 400，不再静默忽略。基本分页默认值/限制和权限不变。非法查询在后端枚举前被拒绝；枚举失败返回错误，不返回空成功。不会重排后端持有的切片。仍使用现有五秒全量枚举路径，不代表索引、原子快照或万级 Queue 性能目标通过。其他列表路由不变。

`go test -count=1 ./management/internal/api ./api` 和 `go vet ./management/internal/api` 通过。两个路由均覆盖 201 条资源检索、降序、筛选总数、分页、非法查询和失败场景。未改生产 UI 或部署；完整 C-03 性能和集成门槛仍未关闭。

### 后续进展：调用实际 API 的编辑流程模块

新增 [queue-editor.mjs](../admin-ui/src/queue-editor.mjs)，独立于尚未决定的前端工具链。通过共用 API 接口调用规范 Queue GET、POST 预览及条件 PUT，保留原始不透明 ETag 和精确草稿值。草稿修改使预览失效；异常/身份不符预览不能授权提交。提交立即锁定，重复操作不能再次派发。接受写入不标为已收敛。未知/部分/无法解析的写结果保留草稿并阻止继续写入或导航。已识别的权限/冲突响应需重新显式预览；不静默获取新 ETag。清除会话要求明确确认，迟到响应不能恢复已清除状态。

`node --test admin-ui/tests/api.test.mjs admin-ui/tests/queue-editor.test.mjs`：14 条测试通过。测试使用注入 API，不是真实后端/浏览器证据。生产入口尚未导入此模块。实际表单/JSON 控件、冲突比较/重新应用、不确定结果核查/恢复及删除仍需实现并接入；不能据此宣称 C-05 或 WebUI 完成。

### 后续进展：显式冲突合并

编辑模块现可读取最新规范文档作为冲突证据，同时分别保留原基准、原 ETag 和本地草稿。只有明确确认调用方提供的合并文档后，才更新基准/ETag；不自动合并、提交或重试冲突。合并后始终作废预览。当前数据缺失/不可编辑时保留旧草稿并阻止合并。不确定写结果不能借冲突恢复绕过写入锁。读取期间禁止编辑；显式清除会话后丢弃迟到响应。

同一 Node 命令现通过 17 条测试，包含精确大 ETag、新旧字段保留、无自动 PUT、读取失败保留和旧会话响应抑制。这仅是流程模块证据：实际三方比较界面、真实服务集成、不确定结果解除及删除仍待完成。

### 后续进展：不确定写入证据

每次 apply 现生成密码学随机关联 ID 并发送 `X-Request-ID`；请求 ID、返回关联标识、已提交计划及结构化失败证据保留在内存。关联不等于幂等。`inspectUncertain()` 仅读取声明及 Queue Consumer 首个 200 条分页，保留分页总数/偏移，不冒充完整集合。各来源保留独立响应、读取时间、缺失/不可用区分及 ETag。核查不更新编辑基准，不将当前状态归因于失败请求，不自动重试或解除写锁。清除后丢弃迟到证据。

Node API/编辑器测试现为 19 条通过，仍是注入 API 测试，不是生产运行证据。审计关联查询、基于证据的显式结果确认、完整分页证据 UI、删除及实际页面接入仍待完成。

### 后续进展：现有生产编辑器并发修复

已修改 `admin-ui/dist/management.js`，不改变框架或布局：编辑时保存首次读取声明的 ETag；apply/delete 使用该原始字符串，不再在提交前获取替代版本。缺失原始 ETag 时阻断两种写入。创建明确使用仅创建条件（`If-None-Match: *`），输入已存在名称不再隐式覆盖。冲突时编辑器和草稿保持打开。

`node --test admin-ui/tests/legacy-concurrency.test.mjs`（3 条）和 `go test -count=1 ./admin-ui` 通过。测试执行实际交付 JS 源码，使用 DOM/网络替身验证请求顺序、精确大 ETag、仅创建及缺失版本拒绝，不代表浏览器/真实服务门槛通过。运行中的冻结制品未重建或替换，行为未改变。旧表单映射、无损解码、不确定结果安全恢复及完整重设计接入仍待完成；此修复不代表旧 UI 已可发布。

生产脚本现增加 apply/delete 共享的进行中提交锁，禁用修改/编辑/关闭控件，并在请求未结束时阻止对话框 Escape。行为测试覆盖两种操作先后顺序，验证仅发送一次网络请求，并在冲突后恢复控件。旧编辑器并发测试现为 4 条通过，Go 内嵌 UI 测试也通过。这仅保护进行中的请求，不是出错后的持久不确定状态保护；后者仍待接入。未执行部署。

### 后续进展：精确节点范围 CID 搜索

Connections 列表现支持在选定节点完整范围内按 CID 精确搜索。管理服务以 `limit=1`、`offset=0`、`state=open`、`auth=false` 和 `subs=false` 将 `cid` 传给 NATS `connz`；校验选定 server 身份及返回 CID 完全一致后，对外返回零或一的过滤后总数。CID 以 uint64 无损解析，不经过浮点转换。搜索不是当前页过滤，不表示全局连接身份，也不暴露客户端名称、账户或订阅 Subject。客户端名称搜索仍需先确定单独的敏感元数据策略。

路由、解码器和 UI 均拒绝畸形、零值和溢出 CID；CID 搜索不能与非零 offset 组合。普通分页继续保留节点报告总数。相关 Go 测试、vet、353 条前端测试（跳过一条 Windows 符号链接测试）、候选构建以及候选/嵌入式 Chromium 测试均通过。两套浏览器测试各完成 130 项检查和 24 份可访问性快照，证据为 `artifacts/webui-live-jkehTk/report.json` 和 `artifacts/webui-live-h3Dn3w/report.json`。晋升后的四文件嵌入式资源身份为 `82d6771387c41341d9230a196da59d4addd65b0cf371074dcd8a745c5376bdd5`。未使用远程主机或已有服务。
