# 发布许可证记录

[English](release-license-records.md) | [简体中文](release-license-records.zh-CN.md)

固定版本的 NATS Server 子树采用其附带的 Apache-2.0 `LICENSE`，发布包将它复制为 `NATS-LICENSE`。容器基础镜像及第三方模块保留各自许可证；镜像 SBOM attestation 列出其软件包，供审查使用。

server 管理代码和 Native SDK 仓库目前没有顶层许可证授权。本记录不替其指定许可证。唯一负责人应在对外分发前确定授权条款，并根据生成的 SBOM 审查第三方声明义务。本地资格验收和私有候选包打包不代表获得公开再分发授权。

BuildKit SBOM/provenance attestation 嵌入 OCI 归档。它们是本地构建记录，不是经过发布者身份签名的公开证明。镜像仓库发布和身份签名仍是独立的发布动作。
