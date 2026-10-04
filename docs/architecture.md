# 架构设计

蜜蜂助手将产品用例与智能体执行分开：应用管理会话、任务和权限，AgentKit 管理模型与工具的执行。CLI、Web、OpenAI 兼容 API、消息通道和后台调度复用同一条运行链路。

## 依赖边界

```text
cmd/fkteams
    └── bootstrap               创建并连接实例
          ├── adapters          协议、SDK、文件系统与外部服务
          ├── app               产品用例
          └── runtime           与产品和 SDK 无关的运行基础能力
                └── ports       能力契约
                      └── domain 领域数据
```

这是一组依赖约束，不要求每次调用经过所有目录：

| 层 | 负责 | 允许依赖 |
| --- | --- | --- |
| `domain` | 消息、事件、历史、会话等数据 | 标准库 |
| `ports` | 模型、工具、执行器、存储等契约 | domain |
| `runtime` | 事件、hooks、权限、路径校验、回合生命周期 | ports、domain |
| `app` | 对话、智能体目录、记忆、调度等用例 | runtime、ports、domain |
| `adapters` | 外部技术与协议实现 | app、runtime、ports、domain |
| `bootstrap` | 创建依赖并显式注入入口 | 以上各层 |

AgentKit、Eino 及模型 SDK 只允许在 `internal/adapters/runtime/agentkit/` 中出现。端口不使用 SDK 类型别名，应用测试可以注入 fake runtime。入口不能直接创建智能体执行器或自行写历史。

不建立进程级默认实例，不通过 `init()`、空白 import 或包级 setter 注册依赖。需要的能力由应用实例持有并注入。只有一个默认执行实现时，组合根直接创建它，不再维护未被消费的运行时注册表。

## 一次对话的链路

```text
CLI / HTTP / Channel / Scheduler
    → app/chat.Service
    → runtime/turn.Executor
    → ports/runtime.Runner
    → adapters/runtime/agentkit
    → AgentKit
```

`chat.Service` 装配输入、steering、审批、事件接收器和会话生命周期。`turn.Executor` 执行 before/after hooks，调用 Runner，并保证收尾回调收到结果和错误。它没有额外的 core、engine 或 runLoop 转发层。

Runner 每次运行创建独立的 AgentKit 会话，将输入历史交给库，在同一会话中完成 HITL 恢复，最后清理检查点并关闭会话。检查点 ID 由库生成，不再作为应用运行参数透传。缓存的是智能体声明和 Runner，不是聊天历史；同一 Runner 可服务不同会话，不共享对话状态。

模型错误会同时通过事件与返回值传递，不能只发错误事件后返回成功。取消和审批失败也进入用例收尾路径。事件接收器返回错误时取消执行并保留原始错误。

## 智能体开发

智能体以 `app/agent/catalog/common.Definition` 声明：

```go
def := common.Definition{
    Name:        "reviewer",
    Description: "审查代码并给出有证据的改进建议",
    Instruction: reviewerPrompt,
    Profile:     common.ProfileWorkspace,
    ToolNames:   []string{"file", "git"},
}
agent, err := common.BuildAgent(ctx, def)
```

`BuildAgent` 完成模型、提示词、工具、策略和中间件解析，直接生成一个 `ports/runtime.ChatAgentConfig`。不再保留内容重复的 `ResolvedAgent`，也不为无状态步骤建立 Resolver、Assembler 空类型。

| Profile | 用途 |
| --- | --- |
| Bare | 显式模型和工具，独立后台任务 |
| Workspace | 工作区基础工具和上下文 |
| Full | 工作区、摘要和技能 |
| Team | 协调者使用的完整能力组合 |

内置智能体目录保留定义与独立提示词。新内置角色在 `builtinAgentSpecs()` 注册一次元信息和默认定义。自定义角色通过配置生成相同的 Definition，不增加新的执行路径。

轻量任务使用 `app/agent/standalone.Service` 的 `RunText`、`StreamText` 或 `Run`，不用自行拼装历史、工具或执行循环。

## AgentKit 适配器

依赖固定为 `github.com/wsshow/agentkit v1.5.0`；其要求的 Go 版本是 `1.25.14`，底层 Eino 为 `v0.9.19`。

适配器按职责组织：

- `agent.go`、`factory.go`：可复用声明、能力转换、Runner 创建。
- `agent_tool.go`：把成员工具声明转换为 AgentKit `SubAgentConfig`。
- `runner.go`：会话生命周期、审批恢复和库事件转换。
- `model_events.go`：补齐产品协议所需的消息 ID、工具参数增量和多模态输出。
- `deep.go`：组合规划、工作区、命令和成员能力。
- `model.go`、`message.go`、`tool.go`、`middleware.go`：领域契约与 SDK 的转换。
- `engine/`：组合项目中间件与上述适配能力，避免中间件和核心适配互相导入。
- `providers/`、`middlewares/`：具体模型提供者和产品扩展。

普通智能体、团队成员、Deep 和 dispatch 后台子任务均由 AgentKit 执行。项目不再直接创建 ADK Runner、ChatModelAgent、LoopAgent 或 DeepAgent。

团队成员使用库提供的独立上下文和 HITL 能力，成员输出带稳定的父工具调用 ID。项目不再维护 ADK 事件指针的全局关联表、成员事件转发协程或错误包装智能体。

采用 AgentKit 的默认成员策略：最多 8 次委派、4 个不同成员并行、单次委派 10 分钟；同一个成员不能同时接受多个委派。当前产品目录是单层团队；不通过成员工具递归嵌套团队。

圆桌讨论按配置顺序逐个执行成员，把已有讨论传给下一位；`max_iterations = 0` 保留持续讨论直到取消的行为。Deep 以同一套执行模型组合规划、文件和命令能力，直接暴露成员工具，不再使用另一个预构建框架的 `task` 路由。工作区路径必须显式传入；`shell.streaming` 通过工具进度事件报告输出。

## 状态与事件

每种状态只有明确的所有者：

| 状态 | 所有者 |
| --- | --- |
| 会话标题、收藏、当前角色等 metadata | session store/service |
| 可恢复的聊天记录 | history transcript |
| 当前模型与工具执行、中断恢复 | 当前运行的 AgentKit 会话 |
| 待处理 steering、follow-up 与顺序 | `app/chat/taskstream` |
| 长期记忆 | memory service/store |
| 定时任务和执行记录 | schedule service/store |

AgentKit 的会话持久化和内置消息队列不与现有产品状态并行启用。传给库的历史是本次运行输入；应用仍以 transcript 为持久化事实来源。

产品队列需要稳定的 `queue_id`、编辑、删除、排序和断线恢复，因此保留在 taskstream。运行中的普通消息进入 follow-up；显式 steer 由 `SteeringSource` 在下一次模型调用前消费。已经消费的条目不可修改，队列变更通过 `queue_updated` 同步。终端暂停时将未消费 steering 回填输入框。

事件转换链路：

```text
模型观察 / AgentKit 工具事件
    → domain/event.Event
    → app 事件管线：session、turn、sequence、history
    → transcript / transport DTO
```

库的公共事件没有工具参数分片、产品消息 ID 和完整的多模态输出字段，所以适配器观察同一次模型响应以补齐这些信息，完整消息在产品中间件修复完成后发布；它不负责执行工具或推动模型循环。工具参数第一次出现就确定调用 ID，SDK 执行、`message_delta(tool_args)`、`message_end.tool_calls[]` 和 `tool_start/update/end` 使用相同的 `tool_call_ref`。

增量进入领域协议后统一使用 `Content`，不增加第二个 Delta 字段。UI DTO、历史投影和运行事实保持分层；显示需求不能改变持久化事实模型。

## 扩展点

- **工具组**：通过 `app/tools.ToolGroupRegistry` 注册。IO 和 SDK 实现放在 `adapters/tools`，依赖经 `ToolResolveContext` 注入。策略由 `toolpolicy.ClassifyTools` 标记。
- **模型**：在 `adapters/model/providers` 的实例工厂注册创建和列表能力，SDK 实现放在运行适配器中。
- **Hooks**：契约在 `ports/hooks`，实现总线在 `runtime/hooks`。payload 必须是明确结构体；未注入总线时不执行 hook。
- **消息通道**：adapter 导出接收 FactoryRegistry 的 Register，由 bootstrap 显式装配，通过 Bridge 调用应用用例。
- **后台服务**：实现 `app/lifecycle.Service` 的 Name、Start、Stop。按注册顺序启动、按 LIFO 停止。
- **配置**：通过 `config.Get()` 获取热重载快照；新增配置同步维护示例。

## 安全与验证

工作区路径与符号链接检查、危险命令检测、工具权限元数据和 HITL 审批不因迁移而绕过。没有审批回调、没有回答或后台无交互的情况都不能转成授权。运行结束或取消后清理该运行的检查点。

`internal/architecture` 和适配器边界测试约束依赖方向。运行回归测试覆盖流式工具关联、成员并行问答和恢复、工具 schema 传递、会话隔离、取消、事件接收器失败、默认拒绝、检查点清理、圆桌顺序及 Deep 工作区逃逸。

跨包或运行行为变更运行 `make check`；并发变更另运行相关包的 `go test -race`。生成的 `web/dist` 与 `release` 不提交。
