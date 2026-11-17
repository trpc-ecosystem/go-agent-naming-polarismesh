English | [Chinese](README.zh_CN.md)

# MCP Demo

This example starts a deterministic `trpc-mcp-go` Streamable HTTP server,
registers it through Polaris, reports a real MCP Service Contract, discovers the
instance, and calls one Tool through the adapter client pool.

No LLM key is required.

```text
Provider -> PrepareServer -> go-naming metadata -> MCP Service Contract
Client   -> Polaris Discover -> MCP Pool -> tools/call -> Consumer Contract
```

## Run

Set the Polaris host:

```bash
export POLARIS_SERVER=your-polaris-host
```

Terminal 1:

```bash
go run ./examples/mcp/provider
```

Terminal 2:

```bash
go run ./examples/mcp/client -message hello
```

The Client prints:

```json
{
  "success": true,
  "content": "mcp|trpc.polaris.demo.mcp|hello"
}
```

Defaults:

| Item | Value |
| --- | --- |
| Namespace | `go-naming-polarismesh-mcp-demo` |
| Service | `trpc.polaris.demo.mcp` |
| Transport | `mcp-streamable-http` |
| Endpoint | `/mcp` |
| Provider port | `18494` |
| MCP protocol version | `2025-03-26` |
| Polaris gRPC | `8091` |

The Provider writes only these instance metadata keys:

```text
mcp-server-name
mcp-protocol-version
mcp-endpoint-path
```

The Service Contract uses type `mcp`, feature types `101/102/103`, and the core
MCP request methods. Core methods are stored as InterfaceDescriptors. Because
Polaris currently limits `service_contract_detail.method` to 32 characters,
longer notification names are retained in the contract JSON
`notifications` field instead of being truncated.

`ContractReportPolicy` defaults to `fail-fast`. Set it to `warn` only when the
application explicitly accepts a running MCP server without a persisted
contract.

## Verify

```bash
./examples/mcp/verify.sh --polaris-server "$POLARIS_SERVER"
./examples/mcp/cleanup.sh --dry-run
./examples/mcp/cleanup.sh --force
./examples/mcp/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```

`verify.sh` leaves the Provider running for inspection. Default cleanup stops
the tracked process and removes local artifacts; `--delete-service` also
removes the owned MCP service, instances, and Service Contracts. Cases are described in
[test.md](test.md).

For native HTTP and SSE endpoints under one service, use the
[multi-transport example](transports/README.md). It prepares endpoint metadata
in the extension with `PrepareRegistryConfig`; naming only registers the
ordinary endpoints and owns their lifecycle.
