# Polaris AI Registry integration for tRPC-Agent-Go

[English](README.md) | [简体中文](README.zh_CN.md)

`trpc-agent-naming-polarismesh` connects tRPC-Agent-Go to Polaris AI
Registry. It builds and freezes the A2A AgentCard, injects instance metadata
through `go-naming-polarismesh`, assembles Skill repositories, resolves healthy
A2A instances, creates A2A clients, and optionally registers and discovers MCP
servers and clients.

The module keeps the same ownership boundary as the broader AI Registry design:

- `polaris-go` owns Polaris protocols, AI Agent models, and Skill transport.
- `go-naming-polarismesh` owns instance registration, heartbeat, discovery, and
  governance.
- This module owns the tRPC-Agent-Go adapter and A2A assembly.

## Package layout

The implementation is organized by domain, following the layout used by
`trpc-agent-go`:

- `agent/`: Runtime, lifecycle, options, and provider integration.
- `agentcard/`: A2A AgentCard building, validation, and metadata encoding.
- `config/`: adapter configuration and Skill references.
- `naming/`: Polaris discovery, sticky selection, and A2A client resolution.
- `skill/`: Skill installation and executable repository assembly.
- `mcp/`: Optional MCP metadata, Service Contract, client pool, and watcher.
- `server/a2a/`: A2A server construction.
- `internal/`: errors, metrics, and test helpers.
- Root package: a thin compatibility facade over the domain packages.

## Requirements

- Go 1.21 or newer
- tRPC-Agent-Go v1.11.0 or newer
- A2A v1 and `trpc-a2a-go/v2`
- `go-naming-polarismesh` with dynamic `MetadataProvider` support
- `trpc-mcp-go` v0.0.10 when using the optional `mcp/` package

## Provider quick start

Create one Runtime before the tRPC server starts, call `Prepare`, and let
`go-naming-polarismesh` register the instance. The adapter does not register the
instance itself.

```go
import (
	polarisagent "trpc.group/trpc-go/trpc-agent-naming-polarismesh/agent"
	polarisconfig "trpc.group/trpc-go/trpc-agent-naming-polarismesh/config"
)

config := polarisconfig.DefaultConfig()
config.Namespace = "Production"
config.ServiceName = "customer-agent"
config.AgentName = "customer-agent"
config.AgentVersion = "1.2.3"
config.Description = "Customer support agent"
config.AdvertiseHost = "127.0.0.1"
config.AdvertisePort = 18080
config.RegistrationPort = 1
config.A2APath = "/a2a"
config.A2AVersion = polarisconfig.A2AVersionV10

runtime, err := polarisagent.New(
	polarisagent.WithConfig(config),
	polarisagent.WithSDKContext(registry.SDKContext()),
)
if err != nil {
	return err
}
if err := runtime.Prepare(ctx); err != nil {
	return err
}
defer runtime.Close(ctx)

// Start Serve only after Prepare succeeds.
```

`Prepare` builds an immutable AgentCard and registers this metadata provider
before `Serve`:

```text
ai-agent-name  = <service name>
ai-agent-card  = <complete A2A AgentCard JSON>
ai-a2a-version = 0.3 | 1.0
```

The instance host and version come from the AgentCard. Its port comes from
`RegistrationPort` (default `1`), and the instance protocol is the empty
string. One card produces one instance even when it advertises JSON-RPC,
HTTP+JSON, and gRPC interfaces. Every advertised interface must use the same
IP; business interface ports are never replaced by the registration port.

`Prepare` installs a complete host/port/version/protocol/metadata provider.
Go-naming reads it once for each registration; the application supplies only
generic lifecycle settings such as TTL. `RegistrationSnapshot()` is available
for inspection and explicit integration, with `SetProtocol=true` to clear the
instance protocol. No AgentCard parsing or AI policy remains in go-naming.
`UpdateAgentCard` and `UpdateAgentCardJSON` replace the full frozen snapshot;
a validation failure preserves the previous one.

Static or legacy instance metadata is converted explicitly by the extension:

```go
runtime, err := polarisagent.New(
    polarisagent.WithService("Development", "customer-agent"),
    polarisagent.WithAgentInstanceMetadata("1.0", metadata),
)
```

Remove card metadata and `registration_port` from the naming YAML. Configure
the extension with `WithRegistrationPort` instead (default `1`, range
`1..65535`). New metadata requires `ai-a2a-version`; legacy four-key input
requires an explicit schema and emits only the three new keys, preserving the
card JSON bytes. Mixed sources and conflicting framework configuration fail
before registration. Naming remains the sole registration/heartbeat owner.

If the provider is already installed by the current tRPC process, the Runtime
can reuse its SDK context explicitly:

```go
sdkContext, ok := polarisagent.SDKContextFromFramework("customer-agent")
if !ok {
	return errors.New("polaris registry context is not available")
}
```

## A2A server

`NewA2AServer` injects the frozen AgentCard. Start the server on the advertised
address, and call `Stop` after it has started. The example owns its TaskManager
and HTTP lifecycle when mounting `Handler` directly.

```go
import polarisa2a "trpc.group/trpc-go/trpc-agent-naming-polarismesh/server/a2a"

a2aServer, err := polarisa2a.NewA2AServer(runtime, agentRunner)
if err != nil {
	return err
}
// Start blocks until shutdown; an application shutdown handler calls
// a2aServer.Stop(shutdownCtx) after Start has begun.
if err := a2aServer.Start("127.0.0.1:18080"); err != nil {
	return err
}
```

The Runtime does not start a listener, manage a tRPC server, or close a borrowed
SDK context.

## Consumer quick start

The resolver filters out unhealthy and isolated instances, parses the instance
AgentCard with the `ai-a2a-version` schema, validates interface hosts against
the instance IP, and keeps a sticky instance for each caller key.

```go
runtime, err := polarisagent.New(
	polarisagent.WithSDKContext(sdkContext),
	polarisagent.WithService("Production", "customer-agent"),
)
if err != nil {
	return err
}
if err := runtime.Prepare(ctx); err != nil {
	return err
}
defer runtime.Close(ctx)

remoteAgent, err := runtime.NewRemoteAgentWithBinding(ctx, "tenant-a", "JSONRPC")
if err != nil {
	return err
}
_ = remoteAgent
```

The requested `protocolBinding` selects an interface from the card; the
instance port is not used to build the business request URL. The most recent
successfully resolved card is used only when Polaris discovery fails, and the
fallback is rebound to the requested interface. Invalid cards and host
mismatches are skipped before selection. The adapter does not silently retry a
streaming call after an event has been delivered. Invalid instances and
discovery fallback are reported through tRPC metrics and logs.

Custom A2A options are supplied by a `naming.RemoteAgentOptionsFactory`. Each
factory call owns a new client configuration; it can reuse a concurrency-safe
`http.RoundTripper`, but must create a new `http.Client` before applying options
such as `WithTimeout`. The factory may run concurrently for different remote
agents. Its context is only the construction budget, not the lifetime of later
requests. The adapter reuses the client while the card is unchanged. Card
updates do not wait for older event streams, which remain on their original
instance. Callers must consume each event channel or cancel its Run context.
Instance discovery and service-Skill reads use one bounded SDK attempt and
explicitly disable inherited retries. Synchronous SDK operations still cannot
be forcibly interrupted; cancellation is checked before and after each call.
If discovery changes during construction, the adapter discards the stale build
and retries at most three times. Factories must be safe to invoke again; empty
or permanently rejected snapshots fail without installing a stale client.

```go
remoteAgent, err := runtime.NewRemoteAgent(ctx, "tenant-a",
	func(ctx context.Context) ([]a2aagent.Option, error) {
		return []a2aagent.Option{
			a2aagent.WithA2AClientExtraOptions(
				a2aclient.WithHTTPClient(&http.Client{Transport: http.DefaultTransport}),
				a2aclient.WithTimeout(30 * time.Second),
			),
		}, nil
	},
)
```

The example uses `net/http`, `time`,
`trpc.group/trpc-go/trpc-agent-go/agent/a2aagent/v1`, and
`trpc.group/trpc-go/trpc-a2a-go/v2/client`. Passing saved, mutable options directly
is replaced by this factory contract in the unreleased adapter API.

## Skills

Skill loading supports three modes:

| Mode | Behavior |
| --- | --- |
| `names` | Install only explicit `SkillRefs`; skip List. |
| `published` | Page through published Skills, bounded by list/max limits, and install each active version. |
| `mounted` | Read `extended_metadata` from an AI Agent service and install its `namespace:name` references. |

`SkillRefs` take precedence over service-level references. References are
deduplicated by `namespace:name@version` and sorted before installation.

```go
config.SkillCacheDir = "./var/polaris-skills"
config.SkillMode = polarisconfig.SkillModeMounted
config.SkillRefs = []polarisconfig.SkillRef{
	{Namespace: "Production", Name: "weather", Version: "1.0.0"},
}
config.UseServiceSkills = true
config.MountSkills = true
```

The Runtime installs every selected Skill through the Polaris `SkillAPI`,
verifies that each result stays inside the cache root and contains `SKILL.md`,
then creates a `skill.Repository`.

Published and mounted repositories can refresh atomically through
`SkillListRefresh`. A failed refresh keeps the previous complete repository.
`SkillFailureThreshold`, `SkillListLimit`, `SkillMaxSkills`, and
`SkillCacheMaxAge` bound failures, pagination, capacity, and stale cache repair.

`MountSkillsReplace` defaults to true in `DefaultConfig`: replacement mounts and
installs only the explicit set, including an empty set to clear old references.
Merge mounts submit only the configured ID, name, and version. An explicit
reference to an existing ID replaces its summary with those fields, matching
the Polaris Writer contract; unmentioned IDs retain their latest Console data.
With `UseServiceSkills` and merge enabled, installation uses the service returned
by the mount operation. Each package must expose exactly its selected `SKILL.md`,
whose repository name matches the reference. Mounting is a service mutation and
is not rolled back if a later installation fails.

## MCP

The optional `mcp/` package adapts `trpc-mcp-go` without moving MCP framework
types into the root package or `polaris-go`.

- `PrepareServer` builds and validates `mcp-server-name`,
  `mcp-protocol-version`, and the transport-specific endpoint path.
- `CollectContract` reads Tools, Resources, and Prompts and reports contract
  type `mcp`, feature types `101/102/103`, request methods, and notification
  metadata. Contract reporting uses `ProviderAPI.ReportServiceContract`; queries use
  `ConsumerAPI.GetServiceContract`. Reporting defaults to fail-fast and can use
  `warn` for an explicit best-effort mode.
- `NewClient` supports SSE and Streamable HTTP; `NewStdioClient` keeps stdio
  local and never creates a network instance.
- `Pool` and `Watcher` maintain one initialized client per healthy instance,
  detect endpoint/path/protocol replacements, and report call results as
  `InstanceTypeMCP`.

Polaris currently stores `service_contract_detail.method` as `VARCHAR(32)`.
MCP notification names longer than that are retained in the contract JSON
`notifications` field instead of being truncated or sent as an invalid
InterfaceDescriptor. Core request methods remain stored as server interfaces.

## Polaris APIs

The Runtime creates this adapter's local `AgentAPI` and the `polaris-go` `SkillAPI`
from the same borrowed SDK context used by `go-naming-polarismesh`. Default Agent
service reads use ConsumerAPI, including mounted Skill metadata, without Console.
Service Contracts are reported through ProviderAPI and queried through ConsumerAPI.
Core has no `ServiceAdminAPI`, `AgentAPI`, or `MCPAPI`.

`WithAgentAPI` and `WithSkillAPI` are available for custom implementations and
isolated tests. Production callers normally only provide `WithSDKContext`.

Optional service writes require an explicitly configured adapter API:

```go
agentAPI := agentapi.NewByContext(sdkCtx,
    agentapi.WithConsole([]string{"http://127.0.0.1:8090"}, consoleToken, 5*time.Second),
)
defer agentAPI.Destroy()
```

Pass it through `agent.WithAgentAPI` when `EnsureService` is enabled. Without
Console, writes return `ErrCodeServiceWriteUnsupported`; instance registration
through go-naming remains available. Console request models, configuration, and
transport are adapter-owned. Destroy releases only adapter-owned resources, not
the borrowed SDK context. Final-Skill clearing still requires server support for
explicit empty extended metadata.

## Lifecycle

The supported order is:

```text
New -> Prepare -> Serve -> stop serving -> Runtime.Close
```

- `Prepare` is idempotent.
- `Close` is idempotent.
- Methods reject use before `Prepare` or after `Close`.
- A context passed with `WithSDKContext` is borrowed and never destroyed.
- The instance remains owned by tRPC and `go-naming-polarismesh`; `Close` does
  not deregister it.

## Development

Core is pinned to `polaris-go v1.7.3-0.20261117071435-aa5b423c60a7` without a
filesystem replacement. The new generic go-naming snapshot interface still
uses a sibling checkout (`v0.0.0` plus replace). Publish a fetchable naming
version and remove that replacement before publishing this module. Local
cross-repository tests do not establish independent-checkout or remote CI success.

```bash
go test -race -count=1 ./...
go vet ./...
golangci-lint run ./...
```

## Runnable Examples

Start with [examples/quickstart](examples/quickstart). It contains direct Go
Provider/Client code for deterministic registration, Polaris discovery, and A2A
unary/streaming. Additional focused demos:

- [examples/skill](examples/skill): download and execute a real published Skill;
- [examples/cancel](examples/cancel): cancel a discovered stream and verify
  Provider release;
- [examples/mcp](examples/mcp): register a real MCP Server Contract, discover
  it, and call a Tool through the client pool.
- [examples/mcp/transports](examples/mcp/transports): prepare two MCP transport
  endpoints in the extension, verify native wire calls and both contracts,
  and exercise heartbeat, stable ID, revision and deregistration behavior.

All four run without an LLM key and always require a Polaris host at runtime.

## Full Integration Tests

`examples/ai-registry/` provides standalone Provider and Consumer executables and
28 real E2E cases. The Polaris host must be supplied through `--server` or
`POLARIS_SERVER`; Discovery, Console, and Skill ports default to 8091, 8090, and
8094. Tests use a dedicated `go-agent-naming-e2e` namespace and unique per-run
resources. Explicit empty `extended_metadata` requires Polaris `v2.2.0.0` or
newer. No LLM API key is required.

The full suite requires a Polaris version that supports replacing
`extended_metadata` with an explicit empty list. On an older server, case 3.7
must fail rather than treating a no-op as success. The adapter does not convert
that server response into a successful Skill removal.

```bash
cd examples/ai-registry
export POLARIS_SERVER=your-polaris-host
bash verify_all.sh --server="$POLARIS_SERVER" --resources=cleanup
bash verify_all.sh --server="$POLARIS_SERVER" --resources=keep
bash cleanup.sh --state=.logs/<run-id>/state.json --dry-run
bash cleanup.sh --state=.logs/<run-id>/state.json --apply
bash cleanup.sh --state=.logs/<run-id>/state.json --apply --delete-service
bash cleanup.sh --delete-service --dry-run
bash cleanup.sh --delete-service --apply
```

Choose `keep` or `cleanup` explicitly. Credentials are read from environment
variables or an existing SDK configuration file and never saved to state.
State-scoped `--apply` stops recorded processes but keeps services and Skills.
`--delete-service` additionally removes state services/Skills and historical
empty `go-agent-e2e-*` resources owned by this example. `--delete-services` is
an alias. Cleanup validates ownership labels, process identity and real instance
state. See the [E2E guide](examples/ai-registry/test.md) for details.

For validation on a separate deployment host, build the self-contained cloud
package described in the
[remote E2E guide](examples/ai-registry/cloud/README.md).

## License

Copyright (C) 2026 Tencent. All rights reserved.

Licensed under the [Apache License, Version 2.0](LICENSE).
