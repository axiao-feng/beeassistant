# 圆桌会议模式详解

圆桌会议模式让多个配置成员按顺序讨论同一个问题，并由主持流程汇总观点。

## 启动方式

```bash
beeteams -m group
beeteams -m group -q "讨论一下微服务架构的优劣"
beeteams web
```

## 配置

成员模型通过 `model_id` 引用 `[[models]].id`。发言顺序由 `[[roundtable.members]]` 在数组中的顺序决定。

```toml
[[models]]
id = "deepseek"
name = "DeepSeek"
provider = "deepseek"
base_url = "https://api.deepseek.com/v1"
api_key = "your_deepseek_key"
model = "deepseek-chat"

[[models]]
id = "claude"
name = "Claude"
provider = "claude"
base_url = "https://api.anthropic.com"
api_key = "your_claude_key"
model = "claude-3-sonnet"

[roundtable]
max_iterations = 2

[[roundtable.members]]
id = "logic"
name = "深度求索"
description = "擅长逻辑分析"
model_id = "deepseek"
prompt = ""

[[roundtable.members]]
id = "creative"
name = "克劳德"
description = "擅长创意思维"
model_id = "claude"
prompt = "你是一个擅长创意思维的圆桌讨论者。请优先提出不同视角和替代方案。"
```

成员字段：

| 参数 | 说明 |
| ---- | ---- |
| `id` | 稳定成员 ID |
| `name` | 展示名称 |
| `description` | 能力描述 |
| `model_id` | 引用 `[[models]].id` |
| `prompt` | 可选自定义提示词，留空使用内置圆桌讨论者提示词 |

## 建议

- 为成员选择不同特点的模型，以获得更多元的观点。
- `max_iterations` 建议设置为 1-3，过多轮次可能导致观点趋同。
- 用清晰的 `description` 标注成员专长，方便讨论过程保持角色边界。
