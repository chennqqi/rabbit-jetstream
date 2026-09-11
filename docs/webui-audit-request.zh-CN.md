# 请求关联审计证据

[English](webui-audit-request.md) | [简体中文](webui-audit-request.zh-CN.md)

`GET /api/v1/audit/requests/{requestID}?before={sequence}` 是 operator/auditor 只读接口。请求标识允许 1–128 个 ASCII 字母/数字或 `_.:-`；`before` 是可选的、不包含该序号自身的 uint64 游标。未知/重复/无效参数返回 400。已有审计授权仍具有最终决定权；不可用/数据损坏/后端失败返回 503，不返回部分证据。五秒请求期限包含认证时间。

每次从新到旧最多检查 256 个保留序号位置。响应字段为 `requestId`、`items`、`streamPresent`、`firstSequence`、`lastSequence`、`scanned`、`missing`、`nextBefore`。即使 `items` 为空，也应按 `nextBefore` 继续。缺失位置计入扫描上限。序号整数需要无损解析。该读取不创建审计存储。

这是兼容已有审计数据的有界扫描，**不是**索引式全历史搜索、原子快照、幂等服务或操作结论。首尾序号描述当前保留存储。空游标只表示遍历抵达本次观测下界，不证明过期、缺失、未来或未记录证据不存在。Stream 缺失或无匹配绝不能证明未执行。翻页期间保留范围可能变化。请求标识可能重复使用，需检查 actor/resource/action 与 intent 关系，不能假定仅一次尝试。审计 intent 和 outcome 分别持久化，缺少 outcome 不证明失败。持久请求结果判定仍待完成。

未知写入检查与声明、Consumer 证据一起读取首个审计窗口。“读取更旧审计窗口”遵循排除式游标，即使前一窗口没有匹配事件也可继续。各已读取窗口分别保留范围、缺口计数及读取时间。失败保留旧窗口/游标，需手动重试。校验游标递进、事件身份、顺序及安全 uint64 值。清除会话抑制迟到响应。到达观测下界不代表未执行。检查和翻页均不解锁写入、不替换原始 ETag。完整独立审计 UI 和明确结果判定仍待完成。

后续证据：57 条 Node 测试通过。设置 `RJS_TEST_AUDIT_PAGING=1`，隔离真实测试脚本执行 130 次无关 no-op apply，仅用于生成较新的审计记录，随后验证首窗口无匹配、继续翻页找到真实 outcome、写入锁定保持及检查期间无新增 PUT。`artifacts/webui-live-BFP2Ya/report.json` 通过。这是功能分页证据，不是大规模索引查询性能结果。

验证：`go test -count=1 ./management/internal/api ./management/internal/jetstream ./api`、相关包 `go vet`、55 条 Node 测试及候选构建通过。后端测试覆盖授权、uint64 游标边界、300 事件遍历、保留缺口、取消、损坏和不创建存储。隔离真实服务证据 `artifacts/webui-live-ctJ2bf/report.json` 确认 200 响应丢失并触发传输重发后，可按请求查询真实成功 outcome。这不是发布资格验收。
