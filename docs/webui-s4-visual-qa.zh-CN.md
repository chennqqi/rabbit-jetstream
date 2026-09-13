# WebUI S-4 视觉验收

[English](webui-s4-visual-qa.md) | [简体中文](webui-s4-visual-qa.zh-CN.md)

## 结果

S-4 已完成。Queue/Stream 与全局 Consumer 只读集合现使用共享的原生表格和 offset 分页基础组件。密度调整保留 caption、带 scope 的表头、可通过键盘聚焦的横向溢出、链接、分页范围实时播报以及明确的错误/陈旧证据。

2026-09-13，确定性视觉矩阵在由本地工作树构建的隔离 Docker Desktop 部署上通过。该轮生成了 96 张 Chromium 截图：

- 宽度：1440、1280、1024、375 CSS 像素；
- 语言：英文、简体中文；
- 主题：明、暗；
- 状态：正常、空、错误、陈旧、长标识符、最大列填充。

每个用例均断言页面无整体溢出，button、input、select 无裁切。Firefox 另在 375 CSS 像素下证明表格区域可获得键盘焦点、确实存在横向溢出、可以横向滚动且不会造成页面级溢出。

## 人工复核

已人工复核覆盖各维度的代表性截图及完整矩阵。第一轮发现一个真实的 1280 像素布局缺陷：Consumer 筛选区只按视口宽度选择宽网格，没有扣除 250 像素导航栏。宽布局断点已移至 1451 像素，并重新运行完整矩阵。随后又修正了截图文件名中英文标记相反的证据缺陷：测试现在同时固定浏览器 locale 和 `rjs.language`，之后再次运行完整矩阵并通过。

截图和机器可读的 `manifest.json` 生成在已忽略的 `artifacts/s4-visual-qa/` 下。它们是本地验收证据，不是发布制品。

## 复现

在 `http://127.0.0.1:18223` 启动使用恢复 Token `s4-visual-qa-token` 的一次性同源 management 部署，然后运行：

```powershell
cd admin-ui
npm.cmd run test:visual-qa
```

测试夹具只模拟全局 Consumer 集合响应，以保证每个视觉状态使用冻结数据；认证、路由、嵌入资产、会话行为、布局、浏览器渲染及响应式行为仍为真实实现。

