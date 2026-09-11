# 全局 Consumer 索引规模原型

[English](webui-global-consumer-scale.md) | [简体中文](webui-global-consumer-scale.zh-CN.md)

本记录测量约定上限 10,000 个 Stream／100,000 个 Consumer 下的进程内采集、合并与查询算法。它**不包含** NATS 请求延迟、浏览器渲染、生产并发流量或多轮 p95 分布，因此不能认定端到端延迟要求已达标。

环境：Windows 11 build 26100、AMD64、Intel Core Ultra 7 155H、22 个逻辑基准线程、Go 1.25.13。本机 WMI 权限拒绝读取主机内存，因此不猜测该数据。

在仓库根目录复现：

```powershell
$env:GOCACHE = "$PWD\.tmp\gocache"
go test ./management/internal/api -run '^$' -bench 'Benchmark(Collect|Query)GlobalConsumers100k$' -benchtime=1x -benchmem -count=3
```

初始结果：

| 操作 | 行数 | 单轮耗时 | 分配字节 | 分配次数 |
| --- | ---: | ---: | ---: | ---: |
| 完整代次采集／合并 | 100,000 | 149.55–184.85 ms | 258.01–258.08 MB | 1,510,391–1,510,595 |
| 优化前筛选 `consumer-09` 得到 10,000 行 | 扫描 100,000 | 10.50–10.93 ms | 10.45 MB | 100,022 |
| 移除逐行拼接后的同一查询 | 扫描 100,000 | 八个单次样本为 7.15–12.11 ms | 5.65 MB | 22 |

查询优化消除了 100,000 个临时 haystack 分配，没有改变大小写不敏感的字面匹配或确定性身份排序。剩余查询分配包含 10,000 行结果切片。采集内存包含生成的 Consumer 投影、合并 map、精确编码行预算核算，以及基准数据源逐 Stream 响应的分配。

该实现已在此原型中执行算法规模上限；单次计时不构成统计有效的 p95。端到端验收仍需要：在注明硬件的真实 broker 数据集上重复采集和查询、观察 management RSS/GC、统计 broker API 请求成本、测量 UI 首次可用时间并计算 p95。已尝试 Windows race 测试，但本机默认未启用 CGO 且没有 C 编译器，无法执行；仍要求 Linux CI race 作为最终证据。
