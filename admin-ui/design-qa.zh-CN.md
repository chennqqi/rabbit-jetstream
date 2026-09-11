# 候选界面视觉接入 QA

[English](design-qa.md) | [简体中文](design-qa.zh-CN.md)

final result: blocked

顶栏跟进：身份与清除会话入口不再单独占用正文行。已检查桌面/手机展开状态，当前证据为 `../artifacts/webui-selected-C2Yr8E/identity-mobile.png` 及同目录桌面/收起截图。键盘、外部焦点、矮屏滚动与响应式焦点回归通过（`../artifacts/webui-live-3KKHHW/report.json`）。本次缩减标题区占位，未隐藏身份字段或修改清除保护；面板密度/保真问题仍需最终原图对照，不构成新的视觉 QA 通过。

密度跟进：已在相同数据/视口下共同查看 `../artifacts/webui-selected-gh56io/selected-desktop.png` 与此前 `webui-selected-ZscyyF/selected-desktop.png`。四组长说明改为可用键盘操作的原生展开控件；关键边界标签、全部数值证据和有效警告保持可见。指标与副本表前移，但标题/面板密度与选定原图仍有明显差异。测试证明说明可访问且不调用 API，不构成新的原图设计 QA 通过。

移动导航跟进：主导航横向滚动已替换为展开区，验证 Enter/Tab/Escape、焦点返回、跳转收起及断点切换。证据：`../artifacts/webui-live-ioWWJy/mobile-navigation-open.png` 与 `../artifacts/webui-selected-ZscyyF/selected-mobile.png`。截图复核后移动资源标题/操作也已分行。早期主导航问题的交互部分已修复，完整无障碍/保真对照及其他 P1/P2 仍待完成，不代表新增视觉 QA 通过。

格式跟进：`../artifacts/webui-selected-ThfQJC/selected-desktop.png` 现显示 24h/30s，并将固定时刻显示为明确带 GMT 偏移的上海时间；原始时长/ISO 证据保留。功能断言和真实服务回归通过，不构成新的视觉对照通过记录，也不关闭其余密度/布局问题。

同数据截图现已提供：`../artifacts/webui-selected-7D2S98/selected-desktop.png`（1487 × 1058，密度 1）、`selected-desktop-full.png`、`resource-header.png`、`evidence-configuration.png` 和 `selected-mobile.png`。采用实际候选组件、中文/operator/orders_events/R3/离线模拟数据及选定时刻，不是真实后端；报告记录原图/构建校验和。只将候选提示标注为模拟数据。ETag 12 与模拟 Plan 内容版本分开，Leader 指标有意保持未知。已解决缺少可控截图机制的问题，仍需新的原图/候选对照及 P1/P2 修复，不宣称新增视觉 QA 通过。

标题功能跟进：面包屑、整页只读刷新和标签前的声明元数据已接入（`../artifacts/webui-live-5ZWN3E/report.json`），并验证查询/游标/草稿保留。补齐的是缺失操作，不代表标题密度、图标、健康证据或同状态保真对照已完成。全页截图前已统一滚动位置。下文原始问题仍为基线，等待新的同状态视觉 QA。

功能跟进：主 Consumer 作用域待投递/待确认及列表诊断入口已实现并通过回归（`../artifacts/webui-live-nPoxKY/report.json`），取代下文早期证据/诊断问题的功能部分。同状态视觉比较及其他问题仍待完成，本次跟进不是设计 QA 通过记录。

本记录针对真实 API 候选界面首次接入选定布局，不是此前已通过的模拟原型；不宣称视觉交付完成。

## 证据与归一化

- 原图：`../docs/design/webui/queue-detail-selected.png`，1487 × 1058。
- 实现：`../artifacts/webui-live-8Uk6qS/queue-summary-desktop.png`，全页 1487 × 1303；CSS 视口 1487 × 1058，deviceScaleFactor 1。
- 移动端：`../artifacts/webui-live-8Uk6qS/queue-summary-mobile.png`，CSS 视口 375 × 812，deviceScaleFactor 1，全页截图。
- 原图和实现已在同一比较输入中共同打开，未缩放/裁剪来掩盖实现内容更长。
- 状态不同：原图为中文/operator/orders_events/R3/离线模拟状态，实现为英文/auditor/live_candidate/R1/真实可用状态。因此只能比较结构，不能作为最终同状态保真验收。待取得同一数据和状态再做局部像素对照；全图已经足以确认阻断差异。

## 问题

- P1：缺少同状态对照。应在隔离截图环境用选定数据驱动实际候选组件，不能把模拟观测混入真实监控。最终对照前统一语言、角色、视口、Queue 和故障状态。
- P1：资源标题区结构不同。候选提示和身份展开区位于标题前，声明元数据在标签后，刷新位于证据面板内，缺面包屑。需按选定标题区整合操作/元数据，同时保留声明与观测时间区别及权限控制。
- P1：证据语义与诊断条未完成。当前为存储消息/字节/实际 Consumer 数，不是原图所需指定 Consumer 的待投递/待确认。不能累加 Consumer 或虚构健康来模仿图片；需先完成明确作用域的 Consumer 证据与诊断入口。
- P2：内容密度不同。更多解释文字和六列副本表让面板超出原图高度；应优化层级，不为匹配高度丢弃证据或未知状态。
- P2：移动端主导航采用横向滚动而非此前原型菜单。冒烟中路由仍可通过键盘/点击访问，但可发现性和焦点行为需专门无障碍复核。

## 必查视觉面

- 字体：复用原型 Arial/Microsoft YaHei UI/系统回退；中文同状态排版尚未验收，当前英文标题与元数据层级有差异。
- 间距：已实现 250px 侧栏、70px 顶栏、内容 x=283、2.04:1 双栏及 16px 间隔；纵向节奏和操作位置仍有上述差异。
- 颜色：复用深绿导航、浅底、绿色选中链接及浅边框；完整状态/对比度审查待完成。
- 素材：原图没有定制位图资产，复用文字 RJS 和同版本 Tabler 图标；无整页图片背景或手工图标替代。操作/状态图标尚未完整接入。
- 文案：真实观测与声明配置分开，Leader 未报告指标保持未知，元数据不代表健康；不复制原图错误的指标/lag 解释。同状态内容尚未完整，阻止保真验收。

## 验证与迭代历史

第 1 轮：接入选定导航/外框、可折叠身份证据、六字段辅助配置。真实浏览器回归通过，桌面左右双栏和移动端证据先于配置断言通过，包括页面无溢出。验证身份展开及 auditor 导航权限；原 Consumer、审计、草稿、冲突、未知结果和 Broker 失联流程通过，无页面异常。87 项 Node 测试及候选构建通过。截图对照发现上述待办，尚无修复后的保真通过记录。

## 下一步

1. 建立候选界面同状态截图，完成标题/操作区。
2. 接入明确作用域的 Consumer 指标和诊断入口，不削弱读写契约。
3. 重拍桌面/移动端并对照局部，执行无障碍和交互检查，关闭 P1/P2 后再交付视觉成果。
