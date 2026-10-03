# Consumer 积压排查

[English](webui-consumer-diagnosis.md) | [简体中文](webui-consumer-diagnosis.zh-CN.md)

状态：源码已实现；真实服务默认上限及合成浏览器诊断路径已通过 Chromium `artifacts/webui-live-nVA7Ed/report.json` 和 Firefox `artifacts/webui-live-fEnK5l/report.json` 验证（各 122 项）。这是 WEB-018 的组成部分，不代表 Queue／节点／历史联合诊断或实际饱和验收已完成。

后续源码版本新增 Consumer 范围的副本证据，以及未报告 Leader、Follower 报告离线／未同步和正数落后量的提示。复用身份验证，不虚构 Leader 指标、配置副本数、成员覆盖、法定人数或 Node ID 链接。缺少拓扑表示未报告，不是健康；重复／无效身份会隐藏拓扑及其提示。过期／失败／在途观测清除当前副本提示。落后数遵循 uint64 边界，活动时间遵循非负 int64 边界。Chromium `artifacts/webui-live-20P3Jl/report.json` 和 Firefox `artifacts/webui-live-VmzuCP/report.json` 各通过 124 项检查，包含合成副本拒绝／恢复，以及移动端原生键盘进入、横向滚动到最后指标并退出。两张右侧列截图均已查看，完成后全部六个冻结输入的磁盘哈希一致。副本观测仍不等于节点健康或积压因果证明。

后续本地化版本使用与语言一致的可访问区域名称及面向用户的角色标签，不改变机器证据。Chromium `artifacts/webui-live-H6Vvpo/report.json` 和 Firefox `artifacts/webui-live-RehSGp/report.json` 针对最终响应式 CSS 各通过 125 项。明确的移动端断言要求存在内部滚动区域且角色单元格不被压窄；原生键盘导航仍可到达精确的最右侧指标。因视觉复核发现 CSS 依赖英文标签，之前仅语义通过的报告不作为最终移动布局证据。

精确 Consumer 响应现增加 `max_ack_pending`，直接复制 broker 配置，不替换默认值，保留零和 -1。当前 OpenAPI 响应要求此字段；旧服务可能缺少，前端将证据标为不可用，不猜测上限。不修改 NATS 子树、数据面、配置或授权策略。

Consumer 详情使用精确 Consumer 读取生成建议性比较。现另行独立读取 `/api/v1/nodes`，仅将报告的 Consumer Leader／Follower 名称关联到 server name 唯一匹配且 varz 身份有效的已配置端点。界面单独显示节点读取完成时间。重复 server name 表示歧义；无匹配表示未解析，绝不证明节点缺失。节点读取失败、过期、未来时间或不完整时撤销当前关联，但不使独立保留的 Consumer 观测失效。节点监控可用性不等于节点健康、完整集群成员，也不与 Consumer 读取构成原子快照。

待投递数为正时提示对照后续观测，不宣称积压增长。仅在 explicit／all ACK 策略下，待确认数达到或超过配置的正数上限时提示检查处理与确认情况。待投递消息与零等待 Pull 请求同时出现时提示检查拉取循环，不判定客户端断开。报告重投数为正时提示调查，不给出速率或根因判定。零计数和没有触发提示不代表健康。

计数经过验证并使用精确整数比较，不使用舍入后的 JavaScript 数值。待投递数支持 uint64，其余受支持计数支持非负 int64。配置上限另保留 -1，仅正数上限触发比较。缺失、畸形、越界或不支持的值保持不可用，不以猜测默认值生成证据。

只有 30 秒内完成的成功读取生成当前提示。隐藏标签暂停、刷新在途、刷新失败、较旧数据或未来时间戳会清除当前提示／事实；页面其他位置保留的历史观测不会被当作当前诊断。缺失／非法时间戳为不可用。此新鲜度阈值是展示策略，不是 Consumer 健康边界。

八项诊断单元测试覆盖精确整数边界、阈值相等、有限／非有限上限、ACK／Pull 模式门禁、过期／未来／失败读取、未知值、零计数、副本身份／边界、节点名称匹配／歧义／未解析状态及输入保留。后端测试核对上限原值、副本映射与 API Schema 一致。当前候选与嵌入式 Chromium 证据为 `artifacts/webui-live-VCWwR0/report.json` 和 `artifacts/webui-live-yycovk/report.json`：各通过 131 项检查和 24 份可访问性快照，包含真实配置节点匹配、一个未解析合成成员、节点读取失败／恢复、双语布局和键盘行为。晋升后的四文件身份为 `fff7060cf6b9ce5a95681d12eaf8b76ca8a286a695499b6aed15b358bca50511`。历史趋势工具、权威节点健康契约及真实负载验收仍待完成。不声明已验证投递、处理停滞或端到端健康。
