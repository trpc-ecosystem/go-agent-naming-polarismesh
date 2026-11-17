English | [Chinese](README.zh_CN.md)

# MCP Multi-Transport Registration

One native `trpc-mcp-go` process serves Streamable HTTP and SSE. The extension
validates both configurations with `PrepareRegistryConfig`, then naming
registers the two ordinary endpoints and owns their heartbeats and removal.
Each transport reports its own Service Contract through ProviderAPI.

No LLM key is required.

## Run

```bash
export POLARIS_SERVER=<polaris-host>
go run ./examples/mcp/provider -multi-transport
```

The default ports are 18494 for HTTP and 18495 for SSE, with server versions
`1.0.0` and `2.0.0` respectively. Call them from another terminal:

```bash
go run ./examples/mcp/client -message hello
go run ./examples/mcp/client -transport mcp-sse -server-version 2.0.0 -message hello
```

Both clients use Polaris discovery and the adapter pool, invoke the native
`echo` Tool and query the matching contract through ConsumerAPI.

## Verify

```bash
./examples/mcp/transports/verify.sh --polaris-server "$POLARIS_SERVER"
./examples/mcp/transports/cleanup.sh --dry-run
./examples/mcp/transports/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```

Verification uses service `trpc.polaris.demo.mcp-transports`, ports 28216/28217,
TTL 6, stable IDs and endpoint-specific versions/weights. Its seven lifecycle
cases stop both provider runs and retain the owned service, contracts and logs.
Only explicit cleanup deletes those remote resources. Assertions are listed in
[test.md](test.md).

The server's structured feature readback remains a separate capability: the
script checks persisted features through Console as well as the Consumer
contract digest. A successful digest is not a claim that gRPC fills
`ServiceFeatures`.
