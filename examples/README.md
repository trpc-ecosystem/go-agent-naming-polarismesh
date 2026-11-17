English | [Chinese](README.zh_CN.md)

# Examples

The examples are ordered from a user-facing quickstart to focused runtime
behaviors. They contain runnable Go Provider/Client programs; shell scripts are
optional verification and cleanup wrappers.

| Example | What it demonstrates | LLM key |
| --- | --- | --- |
| [quickstart](./quickstart) | Deterministic Agent, Polaris registration/discovery, A2A unary and streaming | No |
| [skill](./skill) | Published Skill download/install and deterministic execution | No |
| [cancel](./cancel) | Streaming cancellation and Provider release | No |
| [mcp](./mcp) | Real MCP metadata, Service Contract, discovery, pool, and Tool call | No |
| [mcp/transports](./mcp/transports) | Native HTTP/SSE endpoints, independent contracts and lifecycle verification | No |
| [ai-registry](./ai-registry) | 28-case full E2E suite and inspection tools | No |
| [ai-registry/cloud](./ai-registry/cloud) | Package and run the real AI Registry E2E suite on a remote target host | No |

Start with `quickstart`, then run `skill`, `cancel`, and `mcp`.

A Polaris host is always required at runtime:

```bash
export POLARIS_SERVER=your-polaris-host
```

Each demo README contains direct `go run` commands. `verify.sh` runs the same
binaries and intentionally leaves the Provider running. `cleanup.sh` owns
process cleanup; `--delete-service` additionally removes remote resources.

```bash
./examples/<demo>/cleanup.sh --dry-run
./examples/<demo>/cleanup.sh --force
./examples/<demo>/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```
