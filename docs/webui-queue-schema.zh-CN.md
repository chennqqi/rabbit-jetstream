# Queue 编写 Schema

[English](webui-queue-schema.md) | [简体中文](webui-queue-schema.zh-CN.md)

## Schema 控制的路由与标签集合

v1 适配器现在同时编译集合契约和标量字段。核对字符串标签值、可选标签数量/长度范围、允许 null 的唯一字符串 Subject 数组、Subjects/Bindings 互斥分支、字符串 exchange/key 字段、key 唯一性及 direct/topic/fanout 的 key 数量。规则摘要及可展开的字段规则详情来自该契约。浏览器不执行元数据提供的正则表达式，pattern/format 规则仍由服务端验证。

不支持的值替代规则、映射结构、不安全/反向数量、变化的互斥关系或未知分支结构会关闭适配器，不继续启用旧控件。重读保留原始草稿。数组行、重复 Subject/key、空值和多行标签值仍保留供修正；规则展示不会排序、删除、填充或转换草稿。模式/key 移除仍保留原有明确确认及共享 JSON 行为。

这是面向已发布 Queue 契约的版本化适配器，集合控件和布局为专用实现，不是任意 Schema 的布局生成器，也不替代后端权威验证。新的不支持契约结构需要适配器支持，不会静默变成可编辑状态。

## Schema 支持的创建初始表单

创建初始表单现在加载同一认证能力/Schema 契约及编译控件后才允许准备草稿。副本/存储选项及精确消息数范围来自该契约。创建流程仍有意要求明确的存储/副本选择和正数初始消息配额，即使通用 Queue Schema 允许省略存储或保留限制为零。不会向表单插入默认值。

契约缺失、失败或变化时禁用准备草稿和部署选择器，但保留全部已填内容，包括当前不可用的选项和精确整数文本。明确重读只验证当前契约，不创建 Queue、不预览或写入。准备函数也检查就绪状态及选择值，不只依赖按钮禁用。后续服务端预览和有条件的仅创建提交仍然必需。

创建另一个资源或暂存当前草稿时，先重置规则授权再读取新规则，不复用旧创建批准。释放契约会抑制迟到响应并取消观察订阅。表单值仍由原有会话设置独立保留；导航/重新挂载时重读契约，不覆盖输入。

## Schema 驱动的标量控件

候选草稿编辑器现在读取已声明的 Schema 后才启用结构化控件。九个标量字段从已核对契约获取类型、枚举选项、必填性、精确整数范围及省略默认值注解；路由类型选项也来自 Schema。标签/顺序和受支持的 v1 适配器仍在本地。未知字段类型、不安全范围、非法枚举或不支持的路由契约会关闭适配器。不插入默认值、不钳制数值，留空仍表示省略。为精确编辑 int64，输入保持文本形式，最终仍由服务端预览验证。

表单元数据在会话内独立于原始输入和预览授权保留，明确重读只进行读取。观测失败/变化移除表单规则，但保留原文/合并草稿及原始 ETag；迟到元数据不会改写非法 JSON。新的成功预览可恢复匹配规则。成功后开始下一次编辑会重新读取规则。预览/提交、归档或丢弃前会在逻辑上取消未决表单读取，避免迟到结果改写保留状态。原有完整写操作保护仍独立执行。

创建初始设置和受支持的路由/标签集合现已按上文消费同一契约。集合布局仍为专用 v1 控件，未实现任意 Schema 生成布局或未来契约版本。

状态：本机候选契约及界面接入，不代表冻结发布资格或已完成 Schema 自动生成表单。

## 发布与身份

单一来源为 [internal/topology/queue-schema.json](../internal/topology/queue-schema.json)，由共享拓扑包嵌入。描述全部 Queue、元数据、路由、保留、投递及死信字段。反射测试在 Go 文档新增字段而 Schema 未覆盖时失败；默认值和优先级范围与解析器交叉检查。

`GET /api/v1/console/queue-schema` 要求 operator 或 auditor 认证，local-demo 也不例外。不读取后端/monitor，以 `application/schema+json` 返回嵌入字节，设置 `Cache-Control: no-store` 和强 `"rjs-queue-schema-v1:<SHA-256>"` ETag。查询参数返回 400；凭据缺失/无效返回 401，角色不足 403，认证未配置 404。发布前检查可选的原始 `X-RJS-If-Capabilities-Match`，不匹配返回 412。

Capabilities 声明 `queue-schema` 和 `queue.schema.{id,version,url,etag}`。Schema ID 为 `urn:rabbit-jetstream:queue:v1alpha1:schema:1`，消费契约版本为 `rjs.queue-schema.v1`。内容版本参与 capabilities ETag，因此即使报告的二进制版本不变，Schema 字节变化也会使旧能力绑定审阅失效。两种 ETag 均不证明二进制身份、容量或生产资格。

OpenAPI 的 Queue 组件引用同一认证接口，不再维护另一份不完整字段列表。离线工具应使用源码 Schema 制品；在线解析引用需要凭据。

## 含义与限制

方言为 [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-validation)。`default` 是[注解](https://json-schema.org/understanding-json-schema/reference/annotations)，不是填充或修改草稿的指令。副本仍须明确选择，优先级省略与显式零不同，保留限制为零仍表示不设置该上限。

Schema 约束对象字段、必填身份/副本、枚举、整数范围、Subject/key 唯一性、路由互斥及 direct/topic/fanout 的 key 数量。Bindings 规范化文档允许 `subjects:null`。描述的是明确类型的编写文档，不覆盖全部旧 YAML 标量转换；例如数值标签可能被 YAML 解析器转换接受，但不是 Schema 合法的字符串标签。发布 Schema 不改变原有 API 解析宽松程度。

普通校验器将自定义 `rjs-duration`、`rjs-byte-size`、`rjs-subject` 和 `rjs-routing-key` 格式视为注解。Go 时长纳秒范围、容量单位乘法溢出、通配符语法、死信自引用、生成计划兼容性、归属及声明前置条件仍由服务端验证。Schema 通过绝不授权写入。精确 int64 边界要求无损数字处理，不得使用 JavaScript Number 保存这些值。

## 候选界面行为

Settings 同时读取 capabilities 及其 Schema，核对固定同源路径、已识别的描述符/所消费结构、声明的正文值以及匹配的不透明 ETag；展示 Schema，不插入默认值。刷新失败/不兼容会清除旧展示元数据；迟到读取不能恢复已清除或被新请求取代的模型。

声明 `queue-schema` 后，所有能力绑定的创建/编辑/删除预览及发送前检查都使用原始能力条件读取 Schema。审阅保留 Schema，后续同时比较正文及元数据。缺失/不兼容读取阻止发送，须明确重新审阅。Schema 读取通知保留的未提交审阅；失败/变化使确认失效，但保留草稿/ETag。不解锁正在处理/未知/成功/归档状态。旧凭据响应不能通知新会话。

现有结构化字段、路由和标签控件仍是显式组件，完整 Schema 驱动表单仍待完成。服务端诊断现可定位精确已知控件，组合/未知/不可用字段回退原始 JSON，见[预览诊断](webui-preview.zh-CN.md)。Schema 元数据不替代服务端预览或持久化操作结果解析。

## 验证

在本机运行：

```powershell
go test ./internal/topology ./management/internal/api ./api
python tests/admin-ui/queue-schema-check.py
cd admin-ui
npm.cmd test
npm.cmd run build
npm.cmd run test:live
```

独立 Python 检查需要 `jsonschema` 4.x（使用 4.25.1 验证），使用已安装的元 Schema，不联网下载；与 Go 解析器测试共享 14 组用例。用例明确包含结构通过/服务端失败，以及旧类型转换差异，不是穷尽语义等价证明。前端测试覆盖描述符 URL 拒绝、不兼容结构/版本、精确 int64、Schema 正文变化、刷新失败和迟到/旧会话抑制。浏览器使用真实认证 Schema，注入不兼容/读取失败响应验证 Settings 恢复及零 PUT 发送。
