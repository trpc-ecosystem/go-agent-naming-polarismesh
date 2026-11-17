[English](README.md) | 中文

# 示例

示例按“用户 quickstart -> 重点运行时行为 -> 完整 E2E”的顺序组织。每个 Demo
都包含可直接运行的 Go Provider/Client，shell 脚本只负责可选的真实验证和清理。

| 示例 | 内容 | LLM key |
| --- | --- | --- |
| [quickstart](./quickstart) | 确定性 Agent、Polaris 注册/发现、A2A unary 与 streaming | 不需要 |
| [skill](./skill) | published Skill 下载、安装和确定性执行 | 不需要 |
| [cancel](./cancel) | 流式取消和 Provider 释放 | 不需要 |
| [mcp](./mcp) | 真实 MCP metadata、Service Contract、发现、Client Pool 和 Tool 调用 | 不需要 |
| [mcp/transports](./mcp/transports) | 原生 HTTP/SSE endpoint、独立 Contract 和生命周期验证 | 不需要 |
| [ai-registry](./ai-registry) | 28 项完整 E2E 和诊断工具 | 不需要 |
| [ai-registry/cloud](./ai-registry/cloud) | 构建部署包并在远端目标主机运行真实 AI Registry E2E | 不需要 |

建议依次运行 `quickstart`、`skill`、`cancel`、`mcp`。

运行时必须提供 Polaris 地址：

```bash
export POLARIS_SERVER=your-polaris-host
```

每个 Demo 的 README 都有直接 `go run` 命令。`verify.sh` 对真实环境运行同一组
二进制，并有意保留 Provider 进程；进程由 `cleanup.sh` 统一清理，增加
`--delete-service` 才会删除远端资源。

```bash
./examples/<demo>/cleanup.sh --dry-run
./examples/<demo>/cleanup.sh --force
./examples/<demo>/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```
