# 更新日志

[English](CHANGELOG.md) | [简体中文](CHANGELOG.zh_CN.md)

本文件记录项目的重要变更。

## v0.1.0 - 未发布

- 增加 Runtime 配置、可选的 SDK Context 所有权和生命周期管理。
- 构建并校验 A2A 0.3/1.0 AgentCard、完整的多接口 Card、原始 JSON
  和签名保留，以及三键实例元数据契约。
- 将动态元数据 Provider 与 `go-naming-polarismesh` 集成。
- 增加元数据、host、版本、注册端口、协议和 TTL 的原子快照，并将默认注册端口
  设为 `1`，实例协议保持为空。
- 扩展层独占 AgentCard/A2A/MCP 准备及静态配置冲突策略，通过通用快照接入 naming
  唯一实例生命周期 owner；naming 不再解析 Card 或分类 metadata。
- 增加 `WithAgentInstanceMetadata`，显式转换静态三键及 legacy 四键输入，
  保留原始 Card JSON，旧输入必须指定 schema 且成功准备后发出迁移告警。
- 增加 MCP `PrepareRegistryConfig` 生成多个普通 endpoint，以及原生 Streamable
  HTTP/SSE 双 transport 示例和生命周期验证脚本。
- 保护 MCP SSE 示例的 response writer，拒绝 HTTP handler 返回后的写入，
  并补充原生 wire 调用与并发退出的 race 回归。
- 固定已合入 Core 的可获取版本并移除 Core filesystem replace；CI 只保留
  matching naming 分支依赖，naming 发布与独立 checkout 仍为单独门禁。
- 增加由适配层维护的 `AgentAPI`，使用 ConsumerAPI 完成默认服务读取，并
  通过借用的 SDK Context 集成 SkillAPI。移除 Core 中的
  ServiceAdminAPI 和 AgentAPI 后，将 Console 管理、请求模型、配置和回归
  测试迁移到适配层内部，不保留 Core 兼容层。
- 可选服务管理必须显式启用 `agentapi.WithConsole`。保留三态更新、
  revision/owner/Force 防护、未知元数据、结果不确定的写入恢复，以及
  服务端对最终 Skill 空列表的限制。
- 当资源缺少 revision 时保持为空，并保留借用 SDKContext 的所有权约定。
- 增加 `names`、`published` 和 `mounted` Skill 模式、分页加载已发布
  Skill、`namespace:name` 服务解析、失败阈值、容量上限、过期缓存修复
  和原子周期刷新。
- 增加健康实例发现、AgentCard schema/host 校验、按
  `protocolBinding` 选择接口、粘性选择、跳过错误 Card、指标上报，以及
  绑定到请求接口的缓存 Card 兜底。
- 将固定 tRPC-Agent A2A 客户端限制为 JSONRPC 和 HTTP+JSON，在服务发现前
  拒绝不支持的绑定，跳过不兼容的 wire 版本，同时允许 0.3 Card 声明
  可消费的 1.0 附加接口。
- 将 A2A 客户端调用路由到选定接口的 URL，并从其声明的接口路径提供
  JSON-RPC 服务，包括多接口 Card。
- 增加可选的 MCP 服务端元数据、Contract 采集与上报、SSE、Streamable
  HTTP、stdio、客户端池/Watch 替换检测和 `InstanceTypeMCP` 调用上报。
  Contract 上报使用 `ProviderAPI.ReportServiceContract`，查询使用
  `ConsumerAPI.GetServiceContract`。
- 为 MCP 初始化和工具调用增加超时限制，在连接池刷新或关闭期间保护正在
  使用的连接器，并将 MCP 结果级错误上报为失败。
- 保留旧的 mounted 与显式 Skill 合并行为，并让空或重叠的 Skill 刷新
  以确定的方式收敛。
- MCP 通知名称超过 Polaris 32 字符 InterfaceDescriptor 方法名限制时，
  在 Contract JSON 中保留完整名称而不截断。
- 增加 A2A 客户端和服务端辅助工具。
- 增加可直接运行的 quickstart、真实 Skill、取消和 MCP Demo，并提供
  双语指南和受保护的清理流程。
- 增加包含 28 个用例的真实 Polaris E2E 套件，支持按运行状态限定清理
  范围、校验 Provider 身份，并显式删除 Service/Skill。
- 使用显式空 `extended_metadata` 清理服务 Skill 引用时，要求 Polaris
  `v2.2.0.0` 或更新版本。
- 增加测试、race 验证、README 文档和 CI。
- 要求 Go 1.21。
