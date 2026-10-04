# MCP 工具集成指南

蜜蜂助手可以通过 MCP 接入外部工具和数据源。MCP 服务在 `~/.fkteams/config/config.toml` 的 `[[tools.mcp_servers]]` 中配置。

## 配置示例

```toml
[[tools.mcp_servers]]
id = "filesystem"
name = "文件系统"
description = "文件系统操作工具"
enabled = true
timeout = "30s"
url = "http://127.0.0.1:3000/mcp"
transport = "http"

[[tools.mcp_servers]]
id = "postgres"
name = "PostgreSQL"
description = "数据库查询工具"
enabled = true
timeout = "30s"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-postgres"]
env = { DATABASE_URL = "postgresql://localhost/mydb" }
transport = "stdio"
```

字段说明：

| 字段 | 说明 |
| ---- | ---- |
| `id` | 稳定服务 ID，工具名使用 `mcp-<id>` |
| `name` | 展示名称 |
| `description` | 服务描述 |
| `enabled` | 是否启用 |
| `timeout` | 初始化超时，使用 Go duration 格式，如 `"30s"` |
| `transport` | `http`、`sse` 或 `stdio` |
| `url` | HTTP/SSE 服务地址 |
| `command` | stdio 本地进程启动命令 |
| `args` | stdio 命令参数 |
| `env` | 当前 MCP server 独立环境变量，推荐 inline table |

## 在自定义智能体中使用

```toml
[[agents.items]]
id = "data"
name = "数据处理专家"
description = "专门处理数据相关任务"
model_id = "main"
prompt = "你是一个数据处理专家。"
tools = [
  "file",
  "mcp-filesystem",
  "mcp-postgres",
]
enabled = true
```

MCP 工具命名规则：

- 服务 `id = "filesystem"` 对应工具名 `mcp-filesystem`。
- 智能体 `tools` 中填写完整工具名。
- `env` 只属于当前 `[[tools.mcp_servers]]`，不会和其他 MCP 服务混用。

## 常用 stdio 示例

```toml
[[tools.mcp_servers]]
id = "github"
name = "GitHub"
description = "GitHub API 工具"
enabled = true
timeout = "30s"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env = { GITHUB_TOKEN = "your_github_token" }
transport = "stdio"

[[tools.mcp_servers]]
id = "brave-search"
name = "Brave Search"
description = "Brave 搜索工具"
enabled = true
timeout = "30s"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-brave-search"]
env = { BRAVE_API_KEY = "your_api_key" }
transport = "stdio"
```
