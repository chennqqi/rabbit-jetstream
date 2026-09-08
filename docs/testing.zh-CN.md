# 生产测试策略

[English](testing.md) | [简体中文](testing.zh-CN.md)

替代 RabbitMQ 意味着正确性和可恢复性是发布要求，而不是可选 QA。

## Admin UI 浏览器资格验证

`make test-admin-ui` 使用 Docker Compose 启动真实的单节点 JetStream 和管理服务，再通过 Playwright 在 Linux Chromium 与 Firefox 中操作内嵌控制台。测试覆盖仪表盘加载、筛选、键盘操作、优先级 Queue 创建/更新/删除、精确删除确认、Bearer 认证失败、revision 冲突、部分 API 故障、窄屏行为、凭据不持久化，以及 WCAG A/AA serious/critical 自动检查。本地 RC 的 `Full` 和 `Release` 模式均包含此门禁。静态资源或仅 HTTP Smoke 不能替代真实浏览器测试。

## 测试分层

1. 单元与契约测试覆盖配置、API 校验、资源命名、策略调和和失败分支。
2. 集成测试针对固定的 NATS Server 版本运行单节点和三节点部署。
3. 故障测试覆盖 leader 丢失、滚动重启、网络中断、进程故障、重复投递和重连。
4. 兼容性测试联合 Native SDK 验证 Queue、Ack、重投、TTL、DLQ、路由、顺序和优先级语义。
5. 性能与长稳测试使用明确的硬件、副本、消息大小和工作负载与固定基线比较。

## 合并与发布门禁

- 仓库总语句覆盖率不得低于 80%；`internal/topology` 和 `management/internal/controller` 分别不得低于 90%。
- 新增或修改的 Go 代码必须包含有效测试，不得降低覆盖率。
- 发布候选必须通过 Linux race、Docker 集成、三节点故障、备份恢复、滚动升级/回滚、安全扫描、性能回归和 24 小时长稳测试。
- 不稳定测试按生产缺陷处理；隔离必须指定负责人、关联问题和失效日期。

```bash
make coverage-check
make test-security
make test-linux-smoke
make test-linux-fault
```

Linux/AMD64 是首版生产资格平台。Windows 和 Docker Desktop 用于开发反馈，不能替代原生 Linux 资格证据；ARM64 可交叉构建，但在取得原生环境验证前不具备生产资格。完整命令和证据规则见英文默认文档；如有歧义以英文版本为准。

### 影子迁移开发复跑

生产镜像构建使用 BuildKit 模块缓存，以及可配置的 `GOPROXY` / `GOSUMDB` 构建参数。NATS 和 management 构建采用与 operator 一致的默认值。间歇下载失败可重试，但必须保留校验和验证及漏洞门禁阈值。

`tests/integration/shadow-capture.ps1` 默认重新构建生产测试镜像。在 Linux 上，迁移容器使用调用者的 UID/GID，使证据保持私有 `0600` 权限且宿主断言可读。切流动作使用包含 `/bin/cp` 的专用测试镜像运行相同 CLI，生产镜像仍为 distroless。

开发期间依赖下载不可用时，可使用已有测试镜像和已填充的宿主模块缓存：

```powershell
$ShadowGoCache = (go env GOMODCACHE).Trim()
pwsh -NoProfile -File tests/integration/shadow-capture.ps1 -SkipBuild -GoModCache $ShadowGoCache
```

`-GoModCache` 禁止 publisher 辅助程序下载模块。`-SkipBuild` 不能证明当前发布 revision 已取得资格。CI 和本地 `Release` 门禁使用默认完整构建流程，并保留全部迁移断言。
