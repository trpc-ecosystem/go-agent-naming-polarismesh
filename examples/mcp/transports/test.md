English | [Chinese](test.zh_CN.md)

# MCP Transports Test

The following seven cases match [verify.sh](verify.sh). Prerequisites are
reachable Polaris gRPC/OpenAPI ports, Go, curl, jq and no active instances for
the owned target service. Use an isolated namespace, never `default`.

## Cases

- **5.1 MCP metadata and two protocols**: HTTP/SSE instances advertise
  `127.0.0.1`, ports 28216/28217, protocols `mcp-streamable-http`/`mcp-sse`,
  versions `1.0.0`/`2.0.0`, weights 80/120 and stable `-http`/`-sse` IDs.
  Both carry the standard MCP name/version/path metadata.
- **5.2 MCP service type**: the service has `SERVICE_TYPE_MCP_SERVER`.
- **5.3 TTL and heartbeat**: both enable health checks with TTL 6, remain healthy
  beyond TTL, become unhealthy while the process is paused, then recover.
- **5.4 two wire calls and Provider/Consumer contracts**: each native transport
  returns `mcp|trpc.polaris.demo.mcp-transports|hello`; each matching Consumer
  contract has a digest and Console has the persisted `echo` feature.
- **5.5 deregistration and offline service**: stop the provider and verify zero
  instances and an `OFFLINE` service.
- **5.6 stable instance IDs and revision after re-registration**: restart the
  same endpoints and IDs, then verify both revisions are non-empty and changed.
- **5.7 repeat deregistration**: stop the second provider and verify zero
  instances again.

## Evidence and Cleanup

The script exits on a failed assertion. Local native-wire tests do not replace
this deployed-server run; the new suite must be run before release.
The service, contracts and logs remain after lifecycle assertions.
`cleanup.sh --force --delete-service --polaris-server <host>` deletes only
resources accepted by the existing ownership checks.
