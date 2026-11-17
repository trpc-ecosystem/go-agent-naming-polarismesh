[English](test.md) | 中文

# MCP Transports 测试

以下七项与 [verify.sh](verify.sh) 一致。需要可访问的 Polaris gRPC/OpenAPI、
Go、curl、jq，且归属的目标服务无运行中实例。使用隔离 namespace，禁止 `default`。

## 用例

- **5.1 MCP metadata and two protocols**：HTTP/SSE 使用 `127.0.0.1`、
  端口 28216/28217、protocol `mcp-streamable-http`/`mcp-sse`、
  version `1.0.0`/`2.0.0`、weight=80/120 和稳定 `-http`/`-sse` ID。
  两个实例携带标准 MCP name/version/path metadata。
- **5.2 MCP service type**：服务类型为 `SERVICE_TYPE_MCP_SERVER`。
- **5.3 TTL and heartbeat**：两个实例启用健康检查且 TTL=6；超过 TTL 仍健康，
  暂停进程后变为不健康，恢复后重新健康。
- **5.4 two wire calls and Provider/Consumer contracts**：两个原生 transport 均返回
  `mcp|trpc.polaris.demo.mcp-transports|hello`；各自的 Consumer Contract
  digest 非空，Console 包含真实持久化 `echo` feature。
- **5.5 deregistration and offline service**：停止 Provider 后实例数为零，
  服务为 `OFFLINE`。
- **5.6 stable instance IDs and revision after re-registration**：以相同 endpoint
  和 ID 重启，两实例 revision 非空且与之前不同。
- **5.7 repeat deregistration**：再次停止 Provider，实例数再次为零。

## 证据与清理

断言失败时脚本返回失败。本地原生 wire 测试不替代部署服务端验证，
新套件须在发布前真实运行。生命周期断言后服务、Contract 和日志仍保留。
`cleanup.sh --force --delete-service --polaris-server <host>` 仅删除通过既有
ownership 校验的资源。
