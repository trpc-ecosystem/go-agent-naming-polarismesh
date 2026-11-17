[English](test.md) | 中文

# MCP 示例测试

本文档与 `verify.sh` 保持一致。

## 前置条件

- Go 1.21 或更新版本。
- 可访问且支持 MCP Server 与 Service Contract 的 Polaris。
- Polaris gRPC 默认端口 `8091`，可用 `--polaris-grpc-port` 覆盖。
- Provider 默认监听 `127.0.0.1:28214`。

不需要 LLM key。

## 用例编号

### [用例 4.1 MCP metadata registration and contract report]

操作：预创建已归属的 `SERVICE_TYPE_MCP_SERVER` 服务，启动真实 `trpc-mcp-go`
Provider，通过 go-naming 注册，再读取 Tools、Resources、Prompts 并上报、校验
Service Contract。

预期：实例健康且包含 `mcp-server-name`、`mcp-protocol-version`、
`mcp-endpoint-path`；上报成功，ready 文件包含非空 content digest，
feature type 包含 MCP Tool，可持久化的核心 method 写入 InterfaceDescriptor。

### [用例 4.2 MCP Polaris discovery, pool, and tool call]

操作：Client 通过 Polaris 发现 MCP 服务，由 Pool 创建 Streamable HTTP Client，
调用 `echo` Tool，再用同一 SDKContext 的 ConsumerAPI 查询 Service Contract。

预期：响应包含 `mcp|trpc.polaris.demo.mcp|hello`；Consumer 查询返回 type `mcp`、
digest 和 InterfaceDescriptor。如果服务端 gRPC 查询不回填结构化 feature，
Console feature API 仍必须返回已持久化的 `echo` feature。
本地直连或伪造结果不算通过。

## 清理

`verify.sh` 不停止进程。默认 `cleanup.sh --force` 只停止已记录的 Provider 并删除
本地产物；增加 `--delete-service` 才在实例所有权校验通过后删除远端服务及其
Service Contract。
