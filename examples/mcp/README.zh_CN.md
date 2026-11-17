[English](README.md) | 中文

# MCP 示例

该示例启动确定性的 `trpc-mcp-go` Streamable HTTP Server，通过 Polaris 注册实例、
上报真实 MCP Service Contract，再由 Client 发现实例并通过适配层 Client Pool
调用 Tool。

不需要 LLM key。

```text
Provider -> PrepareServer -> go-naming metadata -> MCP Service Contract
Client   -> Polaris Discover -> MCP Pool -> tools/call -> Consumer Contract
```

## 运行

设置 Polaris 地址：

```bash
export POLARIS_SERVER=your-polaris-host
```

终端 1：

```bash
go run ./examples/mcp/provider
```

终端 2：

```bash
go run ./examples/mcp/client -message hello
```

Client 输出：

```json
{
  "success": true,
  "content": "mcp|trpc.polaris.demo.mcp|hello"
}
```

默认配置：

| 项目 | 值 |
| --- | --- |
| Namespace | `go-naming-polarismesh-mcp-demo` |
| Service | `trpc.polaris.demo.mcp` |
| Transport | `mcp-streamable-http` |
| Endpoint | `/mcp` |
| Provider 端口 | `18494` |
| MCP 协议版本 | `2025-03-26` |
| Polaris gRPC | `8091` |

Provider 只写入以下实例 metadata：

```text
mcp-server-name
mcp-protocol-version
mcp-endpoint-path
```

Service Contract 使用 type `mcp`、feature type `101/102/103` 和核心 MCP request
method。核心 method 会写入 InterfaceDescriptor。当前 Polaris 的
`service_contract_detail.method` 上限为 32 字符，超过限制的 notification 名称不会
被截断，而是保留在 Contract JSON 的 `notifications` 字段中。

`ContractReportPolicy` 默认 `fail-fast`。只有应用明确接受“MCP Server 可运行但
Contract 未持久化”时，才配置为 `warn`。

## 验证

```bash
./examples/mcp/verify.sh --polaris-server "$POLARIS_SERVER"
./examples/mcp/cleanup.sh --dry-run
./examples/mcp/cleanup.sh --force
./examples/mcp/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```

`verify.sh` 会保留 Provider 供检查。默认 cleanup 只停止已记录进程并删除本地产物；
增加 `--delete-service` 才会删除已归属的 MCP 服务、实例和 Service Contract。用例见
[test.zh_CN.md](test.zh_CN.md)。

同一服务的原生 HTTP/SSE endpoint 见 [多 transport 示例](transports/README.zh_CN.md)。
扩展层通过 `PrepareRegistryConfig` 校验并生成 endpoint metadata，naming 只注册普通
endpoint 并持有实例生命周期。
