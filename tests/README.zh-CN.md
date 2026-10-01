# 测试套件

[English](README.md) | [简体中文](README.zh-CN.md)

- `integration/`：真实单节点和三节点 JetStream 部署的黑盒测试。
- `fault/`：节点丢失、网络中断、进程崩溃、磁盘压力和恢复场景。
- `compatibility/`：与客户端 SDK 仓库共享的 RabbitMQ 风格语义契约。
- `performance/`：可重复的吞吐、延迟、资源和长稳基准。

单元测试与 Go package 放在一起。Fixture 必须确定且不得包含凭据。

## 本机连接 API 验收

套件还逐一读取 201 个客户端的精确详情，区分节点缺失与 CID 缺失，拒绝非法 CID/查询输入，验证匿名 GET/HEAD 拒绝，并确认关闭 CID 后该详情缺失而存活 CID 仍可读取。如需保留较早候选二进制，构建 `go build -o bin/rjs-management-connection-detail.exe ./management/cmd/rjs-management`，运行脚本前设置 `RJS_TEST_MANAGEMENT_BINARY=bin/rjs-management-connection-detail.exe`（PowerShell：`$env:RJS_TEST_MANAGEMENT_BINARY='bin/rjs-management-connection-detail.exe'`）。覆盖路径相对仓库根目录，非 Windows 省略 `.exe`。选定二进制必须包含详情 API，运行前后会核验指纹。这是单节点功能证据，不是 WebUI、集群/重启或负载验收。

本机构建 `go build -o bin/rjs-management-connections.exe ./management/cmd/rjs-management`，然后在仓库根目录运行 `node tests/integration/connections-live.mjs`。需要本机构建的 `bin/nats-server-candidate.exe` 和 Node.js；非 Windows 主机的两个二进制文件名均省略 `.exe`。运行期间不得替换任一二进制。

脚本启动专用回环地址服务及 201 个空闲 NATS 客户端，验证精确节点身份及完整五页可达性（含管理连接），关闭一个客户端并检查总数变化。不发布消息，不使用容器或远程主机，仅停止本次创建的进程/连接。随机 API 凭据仅保留在内存；`artifacts/connections-live-*` 报告包含运行前后二进制指纹，不含凭据。临时 Broker 数据保留在该证据目录中。这不是负载、长稳、多节点或 WebUI 验收。
