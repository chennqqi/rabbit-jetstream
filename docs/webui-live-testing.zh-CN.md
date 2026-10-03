# 候选 WebUI 真实服务冒烟测试

[English](webui-live-testing.md) | [简体中文](webui-live-testing.zh-CN.md)

节点刷新测试验证共享手动偏好、真实周期详情读取、模拟 503 后旧快照/时间保留并显示旧数据提示，以及模拟 Node ID 缺失后清除详情再恢复。独立真实 Broker 停机测试仍检查不可达端点不携带旧身份链接。这些是监控/读取测试，不是节点管理操作或原生主机验收。

Queue/Stream 列表刷新测试等待真实周期读取，核对 URL 和未提交搜索输入不变，并验证读取失败保留/恢复。随后将共享设置偏好切换为手动：两个列表须在进入时读取、11 秒内无周期请求，显式刷新恰好发送一次 GET。这些检查与其他测试一样使用隔离服务，不使用外部账户或已有监听服务。

## 浏览器选择

Routing-probe 请求证据只记录方法、开始时间、HTTP 状态及响应头／完成／失败耗时，不记录查询值、认证信息或响应正文。这样可在不把敏感路由数据写入证据的前提下，区分路由不可用、传输停滞与 UI 断言超时。

Queue 模板辅助测试在本地审阅 DLQ 和优先级 JSON，切换模板时不带入未选中字段，保留超过 JavaScript 安全整数范围的数值，保存中文移动端截图并检查页面溢出。随后准备和预览优先级草稿，确认一次仅创建写入，读取实际保存的声明，并核对未创建 DLQ 目标。“创建另一个 Queue”须清除模板选择。该检查仅使用自有隔离夹具，启动运行不等于通过。参见[模板范围](webui-queue-templates.zh-CN.md)。

Consumer 副本辅助测试现从原始观测摘要用 Tab 进入有名称的滚动区域，用 ArrowRight 滚至右边界，核对 Follower 最后一个指标的精确文本及水平可见性，保存 `consumer-replica-keyboard-right-columns-zh.png`，再用 Shift+Tab 返回摘要。这只验证所测移动端键盘路径，不是完整无障碍或真实集群健康验收；运行未完成不能计作通过。

Consumer 副本诊断覆盖注入合成命名 Leader／Follower 观测，检查离线／未同步及精确大整数落后提示，再注入重复身份，要求不展示部分拓扑或副本提示。过期诊断须同时清除计数及副本证据。移动端截图含合成集群数据，不是实际集群故障、法定人数或节点健康的证据。

Consumer 诊断检查要求本机构建新后端：`go build -o bin/rjs-management-consumer-diagnosis.exe ./management/cmd/rjs-management`，设置 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-consumer-diagnosis.exe`（非 Windows 省略 `.exe`）。这取代下方历史可执行文件选择。辅助测试验证自有 broker 默认的正数 ACK 上限，再注入明确标记的合成精确计数，检查阈值／无等待 Pull／重投提示。验证双语移动端展示、503 清除当前诊断但保留原观测时间、畸形上限处理及真实读取恢复，只允许 GET 请求。`consumer-diagnosis-synthetic-mobile-zh.png` 是合成计数证据，不是实际积压饱和、消息投递或吞吐验收。

候选输入证据覆盖构建目录内所有普通文件，不仅是 HTML 引用的入口 chunk。运行后复查完整文件清单及内容指纹，新增／删除／修改文件均导致验证失败，并拒绝符号链接。测试期间不得重建。这种前后检查不能识别两次观测之间修改后又恢复的文件，也不是文件系统原子快照。

当前循环错误契约需在本机构建 `go build -o bin/rjs-management-dlq-errors.exe ./management/cmd/rjs-management`，设置 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-dlq-errors.exe`（非 Windows 省略 `.exe`）。这取代下方此前 `dlq-chain` 可执行文件的选择；测试运行期间不得覆盖输入。

批次未知写入覆盖实际转发一次创建，再以畸形 JSON 替换成功响应。确认根项存在而依赖方仍缺失，界面保留原请求标识并阻止提交／准备依赖方／归档；只读检查和站内导航不能解锁或重发写入。这验证未知结果的约束，不代表权威结果判定；检查仍不作归因。

归档覆盖先取消、再接受确认，验证归档不调用 Queue API，使用精确整数／请求标识解析只读历史，导航离开／返回后启动另一批次。复用此前保留名称必须失败且不发送 PUT。归档新的未提交项须保持其未创建，并逐字保留此前归档。保存中文移动端历史截图。不代表会话过期展示或未知写入恢复已验收。

单文件导入覆盖使用实际文件选择，先选畸形文件，再选合法 Queue 文档。验证拒绝文件和拒绝替换时保留原表单，要求显式审阅，预览前不得有 Queue API I/O，并保留未知 spec 字段供服务端拒绝。修正后预览仍不得创建资源；只有独立确认才能发出一次仅创建 PUT。检查精确 int64／优先级／标签和保留名称拒绝，并保存中文移动端导入面板截图。这不代表多文件依赖迁移已验收。

绑定辅助测试还验证原生键盘操作：从探测按钮用 Tab 进入有名称的结果区域，用 ArrowRight 滚至最后一列，再用 Shift+Tab 退出。`routing-bindings-keyboard-zh.png` 记录最后一列视图。故障用例注入未知／畸形 404／409 和畸形 400／403 正文，要求不误报缺失／变化、清除证据并通过真实读取恢复。取消请求的等待使用有界断言，并在 `finally` 中始终释放。

**当前完整套件**要求路由、声明导出、包含 `review_order` 的批次导入规划 API、当前 DLQ 目标 Stream 检查及有界传递循环验证。在仓库根目录本机构建 `go build -o bin/rjs-management-dlq-chain.exe ./management/cmd/rjs-management`，然后设置 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-dlq-chain.exe`（非 Windows 省略 `.exe`）。这取代历史上 external-review／批次执行／规划／导出／路由／连接详情的二进制选择。浏览器运行期间不得替换候选输入。

传递循环样例只在测试自有 broker 上创建三个 Queue，再提交闭合 DLQ 链的修改。预览和条件 PUT 均须以 HTTP 400 `dlq_dependency_cycle` 拒绝循环，保留原声明及 ETag。此错误契约修订要求重建后端；此前 120 项报告测试的是旧 503 响应。这是浏览器测试框架内的 API 回归，不是浏览器交互或并发更新保证。未知写入只读检查另要求声明、Consumer、审计来源明确返回 `available`，不得以 `unavailable` 满足断言。

外部依赖覆盖以缺失外部 Queue 为根的两项依赖链。内部顺序须保持为空，预览顺序按前置项优先。目标缺失时预览必须失败且无 PUT。测试随后仅在自有 broker 中显式创建目标；两次依赖项仅创建 PUT 前，须重新预览并独立确认。检查已保存精确整数和传递前置项门禁。不代表消息投递测试，也不是目标漂移／未知结果的穷尽验收。

逐项执行覆盖独立确认后启动批次、前置项接受前阻止依赖项、通过实际编辑器预览／提交两项，并检查按依赖顺序恰好发送两次仅创建 PUT。仅预览时各 Queue 仍不存在。切换项使依赖项预览失效；站内导航及从创建页返回仍保留批次。检查已保存 int64 数值和中文移动端结果截图。不覆盖未知写入恢复、外部依赖执行或完整无障碍。

批次规划覆盖实际多文件选择、无法读取文件时阻止请求、精确整数传输、来源先于目标的输入被按依赖优先重排，并通过 API 读取证明规划没有创建这两个 Queue。检查重复／无效声明及存在／缺失外部引用保持阻塞、畸形响应清除证据、显式重试与清除文件。中文移动端截图包含长本地文件名。这是规划覆盖，不代表批次执行验收。

声明导出覆盖使用无损解码器读取浏览器实际下载的 JSON，验证精确 int64／优先级零值及默认／显式标签策略，观测策略切换和离开面板后 Blob URL 撤销。检查真实声明变化、畸形响应、取消／重试和中文移动端展示。辅助测试仅拦截 URL 撤销以作观测，仍调用原生方法。文件只含合成测试值，不含真实凭据。下载测试不代表导入或完整依赖迁移已验收。

路由覆盖在测试所属 broker 中创建独立 Queue，包含 direct/topic/fanout 绑定。检查目标／非目标绑定行、末尾 `#` 的零层匹配、中文移动端展示、真实已保存声明修订变化、刷新恢复及取消读取后重试。测试样例写入仅用于准备环境，生产探测仍为只读。移动端检查同时要求页面不溢出，以及绑定表保持可读最小宽度并在容器内横向滚动；此前仅无溢出断言漏掉了文字被挤成竖列的问题。检查每份报告的 `routing-bindings-mobile-zh.png` 和精确输入指纹。局部对比度扫描不代表完整无障碍验收。

精确连接详情检查要求 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-connection-detail.exe`（非 Windows 省略 `.exe`），从支持详情的后端本机构建。套件从每页 25 行的列表上下文进入真实 CID，等待真实周期详情读取，验证不可用历史保留与节点/CID 缺失、拒绝、歧义、畸形响应清空，并通过真实读取恢复。检查双语展示、桌面/移动端可访问性检查点、375px 溢出、返回页大小及浏览器前进/后退。移动端截图在溢出断言前保存，使失败也保留视觉证据。这些检查不验收客户端搜索、订阅详情或集群故障。

DLQ 回归现注入 `lastRun`/`lastSuccess` 非法日历日期及畸形控制器 JSON，检查不兼容展示、导出值排除、独立目标/Stream 观测保留及真实读取恢复。两次受控控制器响应验证该来源等待时刷新按钮保持禁用；第三次持续挂起至生产十秒传输期限将其转为不可用，随后导出及新读取须可正常完成。仅此主动超时断言允许十五秒，正常导出就绪仍保留五秒断言。这些模拟故障不证明此前自发就绪失败的原因。

连接时间回归分别向 `observed_at` 和 `read_at` 注入不可能的日历日期，要求页面和时间清空，再通过真实读取恢复。启用可访问性时，将连接首行的六个数字单元格逐一完整滚入视野，要求实际通过 `color-contrast`，且无违规或未确定结果。`accessibility-connections-mobile-visible-column-*.json` 及对应截图保留这一限定范围的证据，原始裁剪扫描保持不变。

如需仓库内浏览器缓存，在安装和测试前均将 `PLAYWRIGHT_BROWSERS_PATH` 设置为 `artifacts/playwright-browsers` 的绝对路径。从 `admin-ui` 运行 `node node_modules/playwright/cli.js install chromium firefox`。下载的测试工具不是发布制品。

连接页面集成需要包含节点连接 API 的管理可执行文件。设置 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-connections.exe`（相对仓库根目录），可使用单独构建的文件而不覆盖较早的默认候选。非 Windows 主机省略 `.exe`。与默认文件相同，运行前后会核验所选可执行文件指纹；运行期间不得替换它或前端资源。新增连接检查覆盖节点入口、真实总数与周期读取、模拟失败保留/清空、页大小历史、中英文渲染及桌面/移动端可访问性检查点。通过证据必须来自对应精确输入的报告。

移动端对比度复测保留原裁剪内容的扫描报告，在不改变样式的前提下将每个对比度未确定目标滚入可见区域，对该精确元素运行 `color-contrast`。每次复测必须明确通过且检测节点数大于零，同时无违规、无未确定结果，否则运行失败。`accessibility-mobile-contrast-visible-*.json` 及对应截图保留证据。这仅解决该渲染状态下的具体目标，不替代其他人工可访问性工作。之后恢复滚动位置再继续原导航测试。

设置 `RJS_TEST_A11Y=1` 可在登录页、桌面兼容性页、桌面/移动端 Queue 总览及写入结果未知状态运行固定版本 axe-core 4.10.3 检查。启用标签为 `wcag2a`、`wcag2aa`、`wcag21a`、`wcag21aa`、`wcag22aa` 和 `best-practice`，不禁用规则。收集检查点后，确认违规会使运行失败。`accessibility-*.json` 及总报告同时保留违规和未确定项目，仅保存规则/检查标识和选择器，不保存原始 HTML、正文或输入值。未确定项目须人工复核，不计为自动通过。检测引擎是由本机测试脚本注入的开发依赖，不进入发布浏览器资源。原有 72 项功能检查保持执行。这仅覆盖这些已渲染状态，不覆盖全部对话框、语言、对比度/缩放组合或屏幕阅读器流程。

同一套隔离真实服务回归现支持 Chromium（默认）和 Firefox，不为任一引擎减少断言。报告在候选制品指纹之外记录 `browserEngine` 和实际 `browserVersion`。缺少匹配测试浏览器时在本机安装，再显式选择：

```powershell
cd admin-ui
node node_modules/@playwright/test/cli.js install firefox
$env:RJS_TEST_BROWSER='firefox'
npm.cmd run test:live
$env:RJS_TEST_BROWSER='chromium'
npm.cmd run test:live
```

不支持的浏览器名称会在启动服务前失败。安装只影响本机 Playwright 缓存，不修改默认浏览器或远程验收主机。运行仍使用隔离回环服务，只删除自建的空 Queue 样例。测试进行中不得重建候选制品。双引擎测试不替代完整可访问性审计、发布冻结或原生主机资格验收。实际完成的运行见当前开发记录；支持选择引擎本身不等于验证通过。

Schema 回归从真实认证接口核对版本/ETag 和精确 int64 文本，并保存 `queue-schema.json`。Settings 注入不兼容 Schema 后清除旧元数据，显式刷新恢复；提交前注入 Schema 503，验证零 PUT、保留草稿且必须重新预览/确认。独立离线检查运行 `python tests/admin-ui/queue-schema-check.py`（jsonschema 4.x）；其结构/服务端语义边界见 [Schema 文档](webui-queue-schema.zh-CN.md)。

共享观测回归保留编辑/删除审阅，导航到 Settings 并注入能力变化响应，再通过浏览器历史返回。验证旧确认失效、编辑草稿保留，明确重新预览/确认之前对应写请求为零。单元测试还覆盖读取不可用、多模型保留、旧凭据、观察者异常及正在处理/未知/归档状态不变。证据：`artifacts/webui-live-gWvCzY/report.json`；不代表真实滚动升级测试。

接收实例前置条件验证向两种预览及 PUT/DELETE 发送格式有效但不同的能力标识，要求全部返回 412 且 Queue 声明 ETag 保持不变。正常浏览器流程发送认证能力接口的原始不透明标识；合成正文变化响应保留该标识，以单独验证前端指纹失效。这验证契约不匹配拒绝，不是多版本部署或二进制证明。

正常预览/写入流程使用真实能力接口。在 apply 及每次测试删除的发送前读取中，注入一次有效但部署模式已变化的契约，验证对应写请求为零、草稿或资源保留、审阅/确认清空，并可明确重新预览。该响应属于合成契约变化覆盖，不是真实服务升级或原子性测试。`capability-change-before-apply.png` 保存未发送写入的状态。

Settings 验证使用 auditor 读取真实认证能力接口，并验证匿名拒绝。默认隔离进程报告 unknown/unspecified 部署期望、解析器支持值、标准默认值及未报告的资格。注入一次不兼容响应，确认移除旧能力值，再手动刷新真实接口。`console-capabilities.json` 和 `access-settings-mobile.png` 保留证据。显式 standalone/cluster 配置及非法启动由 Go 测试覆盖；浏览器不按测试节点数推断部署期望。

删除测试资源还验证编辑器交接：一个归档已收到成功响应的编辑，另一个归档未提交的非法草稿。浏览器先取消再接受交接确认，验证归档期间声明 ETag 不变，下载 `{fixture}-archived-editor.json`，并确认另一 Queue 草稿未变化。此前结果未知的编辑器必须显示禁用的交接/预检按钮。交接本身不写入；之后的删除仍保留全部确认/预检要求。

删除证据验证在每次专用测试删除后，对测试自有 `live_candidate` 增加 130 次带审计的无变更 apply。声明内容不变；新增审计记录使删除落到首个 256 条扫描窗口之外。界面必须跨过无匹配记录窗口、找到更早请求记录、刷新观测后保留此前窗口，并在不包含凭据、不解锁重试的情况下下载。`{fixture}-evidence.json` 保存浏览器原生下载。这一补充数据步骤属于标准测试，绝不能指向非测试自有服务。

标准测试现在仅在其自有隔离回环 broker 内创建并删除两个专用空 Queue（`live_delete_fixture`、`live_delete_unknown`），验证手输确认、force/刷新失效、原始 ETag DELETE、单次发送及结果证据。第二个测试资源会真实转发删除，但替换成无法读取的响应；界面必须保持未知，且回读后不得重试。审阅/结果 PNG 和报告保留证据。不得将这一破坏性测试指向已有服务。

元数据选项还会在修复漂移后，向四个固定夹具资源注入其他 Queue 的归属标记，验证预览 blocked、提交 HTTP 409、声明 ETag 不变及冲突归属标记仍保留。证据保存在 `metadata-ownership-conflict.json` 和 `metadata-verify-conflict.log`。这些专用测试修改仍只针对脚本拥有的 Broker，不要对已有服务调用工具。

可选真实元数据漂移验收（在仓库根目录的独立 PowerShell 进程中执行）：

```powershell
go build -o bin/metadata-drift-candidate.exe ./tests/helpers/metadata-drift
$env:RJS_TEST_METADATA_DRIFT='1'
Push-Location admin-ui
npm.cmd run test:live
Pop-Location
```

请先按下文构建当前 NATS/管理服务/UI 常规候选。此选项只在测试脚本拥有的 Broker 中创建 `live_metadata_check`，向其 Stream 和三个 Consumer 注入过期受管标签、移除空值受管标签并添加外部元数据。验证声明差异为空但存在四项资源修复、预览后 ETag 不变、条件提交、外部元数据保留及随后 noop。专用测试工具要求明确的回环 URL、固定夹具身份及三个受管 Consumer；不要手动对已有服务执行。报告输入包含工具 SHA-256，`metadata-drift-preview.json` 和工具日志保留证据。工具及夹具均不进入发布制品。这不验证外部写者并发或原生 Linux 执行资格。

候选测试运行本机构建的真实 NATS 和管理二进制，通过同源 API 代理及与生产等效的 CSP 提供 `admin-ui/build-candidate`，并驱动本机无头浏览器。设置 `RJS_TEST_EMBEDDED_UI=1` 后，代理会改为从显式指定的管理二进制获取 `/admin/` 及其资源，同时保留文档所述的 API 响应故障注入。可选 `RJS_TEST_SESSION_EXPIRY=1` 仅替换一次已认证会话响应的到期元数据并推进受控浏览器时钟；这是模拟的生命周期覆盖，不是真实 OIDC 到期资格验收。这是已实现流程的集成门禁，不是完整 WebUI、发布、性能或原生 Linux 资格验收。

如需包含到期证据保留验证，构建后在 `admin-ui` 目录执行 `$env:RJS_TEST_SESSION_EXPIRY='1'; npm.cmd run test:live`。请使用独立 PowerShell 进程，或执行后清除该测试变量以恢复默认覆盖。

## 本机 Windows 命令

每份真实服务报告在 `inputs` 中记录 NATS 可执行文件、management 可执行文件、候选 HTML 及其引用的 JS/CSS 的 SHA-256、字节长度和仓库相对路径。成功测试在设置 `passed` 前重新核对并记录 `inputsVerifiedAt`，输入发生变化则测试失败。这些哈希标识受测文件，不证明源码来源、可复现构建或发布签名。测试期间不要重新构建这些路径。

### 独立的选定数据视觉捕获

在 `admin-ui` 执行 `npm.cmd run build` 后运行 `npm.cmd run test:selected`，用选定的 `orders_events` 数据驱动当前候选组件截图。只启动临时回环静态/测试响应服务器，不启动 NATS 或管理进程，没有上游代理。仅定义四个精确 GET 路径，拒绝写入及未定义路由，并阻止浏览器访问其他来源。不能替代 `test:live`。

浏览器采用中文、Asia/Shanghai、固定时刻 `2026-09-09T08:20:00.000Z`，桌面 1487 × 1058、像素密度 1，移动端 375 × 812。断言存储/待投递/待确认 12480/8420/240、ETag 12、三行副本及离线 Follower，再测试只读刷新。Plan 内容版本与 KV ETag 保持不同。Leader 未报告指标保持未知，不继承生成图中的虚构零值/同步状态。仅将现有候选提示文字替换为明确模拟数据标注，不为截图改变任何数值或布局。

`artifacts/webui-selected-*` 保留视口/全页截图、标题/证据区域截图、原图校验和、JS/CSS 构建校验和及请求/异常报告。`passed` 只表示数据断言和截图流程通过；`visualQAPassed: false` 明确不代表保真、无障碍、生产健康或发布资格。测试数据不被 `admin-ui/src` 或嵌入生产资源导入。成功或失败都会关闭自有浏览器/服务器。后续应使用这些截图做同数据设计对照，不能再把旧的真实 R1/英文状态当作选定原图状态。

默认冒烟还会向摘要的精确主 Consumer 读取注入 503 和 404 响应，检查仅该来源变为未知，再恢复真实 API，并验证精确详情/列表/返回链接。这是浏览器响应测试夹具，不是真实 Consumer 删除或后端宕机。

可选 `RJS_TEST_CLUSTER=1` 会在独立回环客户端/监控/路由端口、独立数据目录启动三个本机构建的 NATS 进程。脚本等待元数据 Leader 一致且两个对等节点已同步，创建 R3 Queue，停止一个实际 Follower，验证离线行，再仅使用该测试进程的数据恢复它并等待追平。之后运行已有工作流，最后停止全部测试 Broker 验证后端失联。在独立 PowerShell 进程的 `admin-ui` 目录执行 `$env:RJS_TEST_CLUSTER='1'; npm.cmd run test:live`。这是本机集成覆盖，不是原生 Linux 发布或网络分区资格验收。

强制停止 Follower 后，报告 `offline` 可能需要数分钟：固定版本服务端的孤立节点阈值为 150 秒，扫描间隔为 90 秒。测试允许等待 270 秒，并记录终止和观测时间，不以进程退出代替服务端报告。报告 `offline: false` 也不证明进程仍可连接。

从仓库根目录构建当前源码：

```powershell
go build -o bin/rjs-management-candidate.exe ./management/cmd/rjs-management
Push-Location upstream/nats-server
go build -o ../../bin/nats-server-candidate.exe .
Pop-Location
Push-Location admin-ui
npm.cmd ci --ignore-scripts
node node_modules/playwright/cli.js install chromium
npm.cmd run build
npm.cmd test
npm.cmd run test:live
Pop-Location
```

每条命令成功后再继续。仅当本机缺少匹配 Chromium 时才需要浏览器安装。Linux 使用相同的不带 `.exe` 的二进制名称及 `npm`；在本机构建，不在发布资格主机上构建。

脚本为 [candidate-live.mjs](../tests/admin-ui/candidate-live.mjs)。使用新选择的回环端口和唯一 `artifacts/webui-live-*` 目录，不连接已有服务或容器。端口预留释放后的短窗口可能出现绑定竞争：启动失败应让测试失败，不能停止占用者。NATS 使用独立数据目录。子进程环境移除继承的 `RJS_`、`OTEL_`、`NATS_` 变量，使用新生成的随机测试 operator/auditor Token。控制器关闭；元数据和初始 Queue 默认单副本，集群模式为三副本。只终止持有句柄的本次子进程，包括主动停止 broker 的检查；现有本机/远程验收进程不受影响。测试数据和脱敏日志保留供检查，不自动删除。

检查包含匿名读取拒绝；真实只读 preview 不创建声明；通过 API 初始化优先级 Queue 及三个 Consumer；auditor 登录/列表/集合/精确详情/后退；浏览器不存储凭据；auditor preview 拒绝；桌面/移动端渲染；真实 broker 停止后显示不可用并清除旧集合行。初始化写入通过隔离 broker 的 operator API 完成，**不证明**候选创建/编辑 UI 已完成。

证据为报告目录下的 `report.json`、`management.log`、`nats.log` 和桌面/移动端 PNG。2026-09-10 本地运行 `artifacts/webui-live-BStzaL/report.json` 的七项检查通过；49 条 Node 测试和候选构建亦通过。截图复核发现了此前“无溢出”断言遗漏的移动端列挤压问题。Consumer 表格现保留可读最小宽度，在可聚焦区域内滚动，原始 Plan 默认折叠。完整选定设计保真、Firefox、无障碍审查、更丰富的故障/变更流程和发布嵌入式构建接入仍未完成。

后续 `artifacts/webui-live-ttHLni/report.json` 通过此前检查及新增 operator 草稿/预览、持久声明不变、路由切换保留草稿和明确清除会话确认。当前 53 条 Node 测试通过。候选编辑器 apply/delete 仍未启用，预览门禁不是变更 UI 已完成的证据。

后续迭代已启用候选确认提交和未知结果检查，以当前[开发记录](webui-development.zh-CN.md)为准，取代历史仅预览状态。默认响应故障为已提交写入/截断 JSON；设置 `RJS_TEST_RESPONSE_FAULT=socket-reset` 可测试响应头前连接重置。报告包含 `responseFault` 以及故障阶段每次 PUT 的请求标识/状态；浏览器传输层可能用同一标识重发，产生先 200 后 409。编辑器必须保持未知/锁定，不能把该 409 当作未写入证明。检查阶段不得新增 PUT。应用层不重试不保证传输层不重发。
