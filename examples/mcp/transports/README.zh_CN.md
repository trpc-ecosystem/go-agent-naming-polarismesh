[English](README.md) | 中文

# MCP 多 Transport 注册

一个原生 `trpc-mcp-go` 进程提供 Streamable HTTP 和 SSE。扩展层通过
`PrepareRegistryConfig` 校验两个配置，naming 注册普通 endpoint，并统一持有心跳
与反注册。两个 transport 分别通过 ProviderAPI 上报 Service Contract。

不需要 LLM key。

## 运行

```bash
export POLARIS_SERVER=<polaris-host>
go run ./examples/mcp/provider -multi-transport
```

默认 HTTP/SSE 端口为 18494/18495，server version 分别为 `1.0.0`/`2.0.0`。
在其他终端调用：

```bash
go run ./examples/mcp/client -message hello
go run ./examples/mcp/client -transport mcp-sse -server-version 2.0.0 -message hello
```

两个 Client 均经过 Polaris 发现和适配层 Pool，调用原生 `echo` Tool，并通过
ConsumerAPI 查询对应 Contract。

## 验证

```bash
./examples/mcp/transports/verify.sh --polaris-server "$POLARIS_SERVER"
./examples/mcp/transports/cleanup.sh --dry-run
./examples/mcp/transports/cleanup.sh --force --delete-service --polaris-server "$POLARIS_SERVER"
```

验证使用服务 `trpc.polaris.demo.mcp-transports`、端口 28216/28217、TTL=6、
稳定实例 ID 和独立 version/weight。七个生命周期用例停止两轮测试 Provider，
归属服务、Contract 和日志保留；仅显式 cleanup 删除远端资源。
详细断言见 [test.zh_CN.md](test.zh_CN.md)。

结构化 feature 回填是独立服务端能力。脚本同时验证 Console 的真实持久化 feature
和 Consumer 的 Contract digest，不能将 digest 成功解释为 gRPC 已回填
`ServiceFeatures`。
