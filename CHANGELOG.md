# Changelog

[English](CHANGELOG.md) | [简体中文](CHANGELOG.zh_CN.md)

All notable changes to this project will be documented in this file.

## v0.1.0 - Unreleased

- Add Runtime configuration, optional SDK context ownership, and lifecycle
  management.
- Build and validate A2A 0.3/1.0 AgentCards, complete multi-interface cards,
  raw JSON/signature preservation, and the three-key instance metadata
  contract.
- Integrate dynamic metadata providers with `go-naming-polarismesh`.
- Add atomic metadata/host/version/registration-port/protocol/TTL snapshots and
  registration port `1` defaults with empty instance protocol.
- Own all AgentCard/A2A/MCP preparation and static configuration conflicts in
  the extension; go-naming consumes only generic frozen fields. Add explicit
  `WithAgentInstanceMetadata` migration for canonical or legacy inputs.
- Add MCP `PrepareRegistryConfig` for multiple ordinary endpoint registrations,
  with native Streamable HTTP/SSE examples and lifecycle checks.
- Guard the MCP SSE demo response writer against writes after the HTTP handler
  returns, with native wire and concurrent shutdown race regressions.
- Pin the merged Core version without a Core filesystem replace; CI retains
  only the matching naming branch checkout until that interface is published.
- Add the adapter-owned `AgentAPI` using ConsumerAPI for default service reads
  and integrate SkillAPI through the borrowed SDK context. Migrate Console
  management, request models, configuration, and regression tests into adapter
  internals after removing Core ServiceAdminAPI and AgentAPI, with no Core shim.
- Require explicit `agentapi.WithConsole` for optional service management.
  Preserve tri-state updates, revision/owner/Force guards, unknown metadata,
  uncertain-write recovery, and the final-Skill empty-list server limitation.
- Keep missing resource revisions empty and preserve borrowed SDKContext ownership.
- Add `names`, `published`, and `mounted` Skill modes, paginated published
  loading, service `namespace:name` parsing, failure thresholds, bounded
  capacity, stale-cache repair, and atomic periodic repository refresh.
- Add healthy-instance discovery, AgentCard schema/host validation, interface
  selection by `protocolBinding`, sticky selection, bad-card skipping, metrics,
  and cached-card fallback rebound to the requested interface.
- Restrict the fixed tRPC-Agent A2A client to JSONRPC and HTTP+JSON, reject
  unsupported bindings before discovery, and skip incompatible wire versions
  while allowing a 0.3 card to advertise a consumable 1.0 additional interface.
- Route A2A client calls to the selected interface URL and serve JSON-RPC from
  its advertised interface path, including multi-interface cards.
- Add optional MCP server metadata, Contract collection/reporting, SSE,
  Streamable HTTP, stdio, client pool/watch replacement detection, and
  `InstanceTypeMCP` call reporting. Contract reporting uses
  `ProviderAPI.ReportServiceContract`; queries use `ConsumerAPI.GetServiceContract`.
- Bound MCP initialization and tool calls, protect in-flight connectors during
  pool refresh/close, and report MCP result-level errors as failures.
- Preserve legacy mounted-plus-explicit Skill merging and make empty or
  overlapping Skill refreshes converge deterministically.
- Preserve MCP notification names longer than the Polaris 32-character
  InterfaceDescriptor method limit in contract JSON instead of truncating
  them.
- Add A2A client and server helpers.
- Add runnable quickstart, real Skill, cancellation, and MCP demos with
  bilingual guides and guarded cleanup.
- Add a 28-case real Polaris E2E suite with state-scoped cleanup, Provider
  identity checks, and explicit service/Skill deletion.
- Require Polaris `v2.2.0.0` or newer for clearing service Skill references
  with explicit empty `extended_metadata`.
- Add tests, race validation, README documentation, and CI.
- Require Go 1.21.
