# Queue 详情设计原型

[English](README.md) | [简体中文](README.zh-CN.md)

本地预览：[http://127.0.0.1:18224/](http://127.0.0.1:18224/)。

按 [选定主辅双栏设计](../../docs/webui-selected-design.zh-CN.md) 实现，使用模拟 Queue/Stream/Consumer 数据。支持五个标签、键盘导航、模拟刷新、限定范围配置校验、复核/应用、放弃确认、内存事件历史、中英文切换及响应式导航。Queue 面包屑打开单行夹具列表；其余侧栏入口明确解释尚未实现的原型范围。

**不是生产 WebUI。** 不连接 NATS、管理 API，不使用真实身份或凭据，不持久存储，不修改实际队列；刷新页面后修改消失。原型只编辑三个字段，因此使用简短弹窗；规划中的完整生产编辑页仍属后续工作。模拟刷新成功后观测已接受的模拟配置；读取失败保留上次值及时间。

## 逻辑审查修正

最新写操作恢复轮次：在编辑器展开**写入模拟场景**，可检查冲突、无权限、过期、未知、部分生效及审计缺失。恢复必须明确复核；未知/部分生效关闭后仍有入口，不能盲目重提。[设计、证据和限制](../../docs/webui-mutation-design.zh-CN.md)。最新计数：**26 项浏览器检查、15 项模型测试、4 项打包测试、15 个无障碍状态自动扫描零违规**，替代下方早期计数。

Consumer 先列表后详情：在全样例范围筛选名称/Subjects/模式、分页、打开指定记录、保留上下文返回。展开模拟场景可选普通/优先级 0/优先级 0–7，以及失败/无权限/缺失读取。原型每页 5 条用于演示分页，不是生产默认 50 条。刷新保留安全路由状态，不保留模拟修改。概况指标实名关联 Consumer；声明与观测分离；等价时长不触发写入。[未关闭的生产门槛](../../docs/webui-logic-review.zh-CN.md)。

最新验证替代下方早期计数：19 项浏览器检查、8 项单测、4 项打包测试，11 个无障碍状态自动扫描零违规。不代表生产验收。

## 本地开发及检查

在本目录执行：

```powershell
npm.cmd ci --prefer-offline --no-audit --no-fund
npm.cmd run build
npm.cmd run preview -- --host 127.0.0.1 --port 18224 --strictPort
```

保持预览运行，在另一终端执行：

```powershell
npx.cmd playwright install chromium
npm.cmd test
npm.cmd run test:ui
npm.cmd run format:check
npm.cmd run test:sites
```

本地账号需要启动 Chromium 和读取浏览器缓存的权限，不需要远程构建。预览只绑定回环地址，不修改生产 Compose 项目或远程服务。在此预览自己的终端用 Ctrl+C 停止，不要终止其他 Node 进程。

修复首轮四项问题后，[设计验收](design-qa.zh-CN.md) 已通过。证据为 13 项浏览器检查、六个无违规的无障碍扫描状态、3 项单测、4 项模板打包测试、构建和格式检查。浏览器仅执行 Chromium。截图及结果生成在忽略版本管理的 `evidence/`；本地缺失时应重建，不能假定本次执行已通过。

图标使用 MIT 授权的 [Tabler 官方 React 包](https://github.com/tabler/tabler-icons/blob/main/packages/icons-react/README.md)，字体采用本机系统回退。保留 Product Design 模板及可选托管文件，但未发布站点，也不意味着迁移生产前端框架。
