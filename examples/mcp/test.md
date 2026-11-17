English | [Chinese](test.zh_CN.md)

# MCP Demo Test

This document matches `verify.sh`.

## Prerequisites

- Go 1.21 or newer.
- A reachable Polaris host with MCP Server and Service Contract support.
- Polaris gRPC defaults to port `8091`; override it with `--polaris-grpc-port`.
- The Provider listens on `127.0.0.1:28214` by default.

No LLM key is required.

## Cases

### [case 4.1 MCP metadata registration and contract report]

Operation: pre-create the owned `SERVICE_TYPE_MCP_SERVER` service, start the
real `trpc-mcp-go` Provider, register it through go-naming, collect Tools,
Resources, and Prompts, then report and verify the Service Contract.

Expected: the instance is healthy with `mcp-server-name`,
`mcp-protocol-version`, and `mcp-endpoint-path`; report succeeds, the ready
file contains a non-empty content digest, and the persisted feature types
include MCP Tool plus persistable core method descriptors.

### [case 4.2 MCP Polaris discovery, pool, and tool call]

Operation: the Client discovers the MCP service through Polaris, creates a
Streamable HTTP client in the pool, calls `echo`, and queries the Service
Contract with ConsumerAPI from the same SDKContext.

Expected: the response contains `mcp|trpc.polaris.demo.mcp|hello`, and the
Consumer query returns type `mcp`, a digest, and interface descriptors. If the
server omits structured features from the gRPC query, the Console feature API
must still return the persisted `echo` feature. A local direct call or a
fabricated result is not accepted.

## Cleanup

`verify.sh` does not stop processes. Default `cleanup.sh --force` stops the
tracked Provider and removes local artifacts. Add `--delete-service` to delete
the owned remote service after instance ownership checks, then delete the
matching Service Contracts.
