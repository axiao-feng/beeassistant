# 架构设计

本文档只描述 fkteams 当前主线架构和边界约束。目标是让核心用例稳定、底层结构清晰、实现可替换，并支持 CLI、Web、纯 API、消息通道和后台任务复用同一套应用能力。

## 架构目标

- 核心用例收敛：入口层只做协议转换、用户交互和生命周期连接，不直接拼装底层运行细节。
- 依赖方向明确：`domain`、`ports`、`app`、`runtime` 不反向依赖 `adapters`。
- Runtime 可替换：核心只依赖 `internal/ports/runtime`，具体运行适配器由组合根注入。
- Agent 可轻量运行：后台小任务可以只用显式 model、instruction、tools 创建独立 agent，不加载完整交互结构。
- 状态事实单一：会话、事件、历史、checkpoint、memory、task result 由明确 store/service 管理。
- 事件协议稳定：运行事实事件、持久化事件和展示 DTO 分层转换，不在入口层重复拼装核心状态。
- 组合显式：runtime、model provider、tool registry、scheduler、channel factory 等都由组合根创建和注入，不使用隐式全局默认实例。

## 依赖方向

```text
cmd
  -> internal/bootstrap
  -> internal/adapters
  -> internal/app
  -> internal/runtime
  -> internal/ports
  -> internal/domain
```

实际代码依赖遵循更严格的单向边界：

```text
domain  <- ports
domain  <- runtime
domain  <- app
ports   <- runtime
ports   <- app
domain/ports/app/runtime <- adapters
domain/ports/app/runtime/adapters <- bootstrap
```

禁止方向：

- `domain` 依赖 `ports`、`app`、`runtime`、`adapters`。
- `ports` 依赖 `app`、`runtime`、`adapters`。
- `app` 依赖 `adapters` 或具体传输、存储、运行实现。
- `runtime` 依赖 `app`、`adapters` 或入口层。
- 入口层绕过 `app` 直接驱动核心运行流程。

## 目录职责

```text
cmd/fkteams/
  main.go                    # 命令入口，只连接组合根

internal/domain/             # 领域模型和值对象
  event/                     # 运行事实事件
  history/                   # transcript、消息投影 DTO
  memory/                    # 长期记忆领域模型
  message/                   # 模型消息、turn input
  schedule/                  # 调度任务领域模型
  session/                   # 会话标识与 context 绑定

internal/ports/              # 外部能力端口和核心契约
  hooks/
  memory/
  runtime/
  scheduler/
  storage/
  tools/

internal/app/                # 应用用例层
  agent/                     # agent 定义、解析、组装和 runner 创建
    catalog/                 # 内置 agent 定义
    standalone/              # 轻量独立 agent 门面
  chat/                      # 对话回合用例中轴
    taskstream/              # 运行中事件流、任务队列、steering
  config/                    # 配置加载、保存、热重载和示例生成
  memory/                    # 长期记忆检索、注入、提取
  schedule/                  # 调度用例服务
  skill/                     # 技能 provider、安装、移除、搜索
  tools/                     # 工具组注册表、工具解析策略
  lifecycle/                 # 应用服务生命周期编排
  appdata/                   # 应用数据目录
  appstate/                  # 应用运行态聚合
  version/                   # 版本元数据

internal/runtime/            # 运行时无关基础能力
  checkpoint/                # checkpoint 存储实现
  events/                    # 事件构造、校验、分发
  hooks/                     # hook bus 实现
  model/                     # 模型工厂注册表
  resources/                 # 资源清理器
  retry/                     # 重试和迭代限制策略
  turn/                      # turn 执行内核
  atomicfile/
  env/
  log/
  mdiff/
  pathguard/
  registry/
  typeutil/

internal/adapters/           # 具体技术实现和协议转换
  model/                     # 模型 provider 实现
  runtime/                   # runtime adapter 实现
  scheduler/                 # 调度器实现
  storage/                   # 存储实现
  tools/                     # 工具 adapter 实现
  transport/                 # CLI、HTTP、消息通道传输层

internal/bootstrap/          # 组合根
  composition.go             # 入口共享的执行依赖创建与 context 装配
  channels/
  environment/
  runtimes/
  services/
  tools/

web/                         # Web 前端工程和嵌入资源
docs/                        # 项目文档
```

## 核心执行链路

所有交互入口统一走应用用例：

```text
transport
  -> app service
  -> runtime/turn.Executor
  -> ports/runtime.Runner
  -> runtime/events
  -> app lifecycle / history / taskstream
  -> transport DTO
```

入口层职责：

- CLI：输入、终端展示、快捷键、当前会话实例生命周期。
- Web/API：请求 DTO、SSE/WebSocket 事件转换、会话路由。
- Channel：平台消息转换、回复发送、当前 Bridge 生命周期。
- Scheduler：时间触发、任务执行上下文、结果归档。

入口层不得：

- 直接创建或调用具体 runtime adapter。
- 直接写 transcript、metadata、memory 或 task result。
- 手动拼装 turn 执行上下文。
- 绕过 `app/chat.Service` 或对应应用服务执行核心流程。

## Chat 中轴

`internal/app/chat.Service` 是对话回合统一入口：

```go
type Service interface {
    RunTurn(ctx context.Context, req TurnRequest) (*runtime.RunResult, error)
}
```

`chat.Service` 负责：

- 接收入口传入的 `TurnRequest`。
- 显式装配 approval、ask、steering、event sink、summary sink、hooks、scheduler service 和 context hooks。
- 调用 `internal/runtime/turn.Executor`。
- 通过 `SessionLifecycle` 保存历史、更新 metadata、处理取消/错误和长期记忆收尾。
- 通过 `taskstream` 管理运行中事件、follow-up 队列、steering 队列、interrupt/ask 响应和断线续接。

`runtime/turn.Executor` 只负责执行一个 turn。它不关心 HTTP、CLI、channel、scheduler，也不直接依赖文件历史、WebSocket 或展示 DTO。

## Agent 构建

Agent 构建分三层：

- `Definition`：声明 name、description、instruction、model、tools、profile 和可选 middleware 配置。
- `Resolver`：解析 model、tools、策略、中间件和运行依赖。
- `Assembler`：调用 runtime 端口创建 agent。

Agent profile：

- `Bare`：只加载显式 model、tools、instruction；用于独立后台 agent。
- `Workspace`：加载工作区基础能力。
- `Full`：加载完整交互能力。
- `Team`：加载团队协作和 sub-agent 工具能力。

内置 agent catalog 只声明 agent 定义。配置、工具注册表、runtime、workspace、session 等外部依赖由 resolver 或组合根显式注入。

## Standalone Agent

轻量后台任务使用 `internal/app/agent/standalone.Service`，不要在业务代码里手动拼 `Definition`、runner、checkpoint 或事件文本聚合。

适用场景：

- 会话标题生成。
- 任务分类。
- 检索 query 生成。
- 简短摘要。
- 后台策略判断。

调用侧只需要提供：

- `Name`
- `Instruction`
- `Model`
- `Input` 或 `Message`
- 可选显式 `Tools`

非流式入口：

```go
text, err := svc.RunText(ctx, standalone.Request{
    Name:        "session_title",
    Instruction: titlePrompt,
    Model:       titleModel,
    Input:       userInput,
})
```

流式入口：

```go
text, err := svc.StreamText(ctx, req, func(delta string) error {
    return nil
})
```

该门面默认使用 `ProfileBare`，只依赖 `AgentRuntime` 和 `RunnerRuntime`，不加载 history、summary、skills、agents.md、默认 workspace middleware 或默认工具。

## Runtime 端口

Runtime 端口按消费者侧拆小接口：

```go
type AgentRuntime interface {
    NewChatModelAgent(ctx context.Context, cfg *ChatAgentConfig) (Agent, error)
    NewLoopAgent(ctx context.Context, cfg *LoopAgentConfig) (Agent, error)
    NewDeepAgent(ctx context.Context, cfg *DeepAgentConfig) (Agent, error)
}

type RunnerRuntime interface {
    NewRunner(ctx context.Context, cfg RunnerConfig) (Runner, error)
}

type AgentToolRuntime interface {
    NewAgentTools(ctx context.Context, subAgents []Agent, cfg AgentToolConfig) ([]Tool, error)
}

type PipelineRuntime interface {
    DefaultAgentMiddlewares(ctx context.Context) ([]AgentMiddleware, error)
    NewSteeringMiddleware() AgentMiddleware
    NewSummaryMiddleware(ctx context.Context, cfg *SummaryConfig) (AgentMiddleware, error)
    NewSkillsMiddleware(ctx context.Context) (AgentMiddleware, error)
    NewDispatchMiddleware(ctx context.Context, cfg *DispatchConfig) (AgentMiddleware, error)
    NewAgentsMDMiddleware(ctx context.Context) (AgentMiddleware, error)
    DefaultToolMiddlewares() []ToolMiddleware
}
```

使用规则：

- 普通业务优先依赖 `AgentRuntime`、`RunnerRuntime`、`AgentToolRuntime`、`PipelineRuntime` 等小接口。
- 组合根可以持有完整 runtime adapter，但应用用例不依赖大聚合类型。
- 运行适配器实现放在 `internal/adapters/runtime`。
- runtime 端口不得暴露具体实现 SDK 类型。
- 测试必须能通过 fake runtime 覆盖 app 层核心路径。

## Tools

工具系统由 `internal/app/tools.ToolGroupRegistry` 管理，工具解析依赖通过 `ToolResolveContext` 显式传入：

- workspace dir
- sessions dir
- runtime dir
- cleaner
- scheduler service
- history reader
- config snapshot

工具边界：

- `internal/app/tools` 只保留注册表、目录查询、工具策略和运行时无关能力。
- 依赖外部 IO、存储、协议、进程、网络、文件格式或生命周期的工具实现放在 `internal/adapters/tools`。
- 默认工具组由 `internal/bootstrap/tools` 注册并注入依赖。
- agent assembler 不手动标记工具策略；策略分类由工具解析流程统一处理。
- 禁止工具实现隐藏读取全局 appdata/config 或通过包级 setter 注入历史 reader。

工具调用事件必须通过稳定 `tool_call_ref` 关联：

- assistant tool args delta
- assistant completed tool calls
- tool start/update/end

## History 与事件

事实来源分层：

- `domain/event.Event`：运行时和用例事实事件。
- `domain/history.TranscriptEvent`：持久化事实来源。
- `AgentMessage`、`MessageEvent`：投影 DTO，不作为主状态模型。

历史记录职责：

- 接收 domain event。
- 追加写 transcript。
- 维护内存 projection cache。
- 为 turn input 输出 `[]message.Message` 投影。

`BuildTurnInput` 依赖 `HistoryProjector`，不直接耦合具体文件历史 recorder。

事件出口分三层：

1. `internal/domain/event`：事实事件。
2. app event pipeline：补齐 session、turn、sequence、history metadata。
3. transport DTO：CLI、Web/API、channel 展示格式。

`internal/runtime/events` 负责事件构造、协议校验、分发和错误归一化。入口层不得构造核心事件，只能转换展示 DTO。

## 状态与存储

状态能力按用途拆分：

- Session metadata：标题、状态、收藏、当前 agent、时间戳。
- History transcript：会话事件流和投影。
- Checkpoint：runtime 执行恢复状态。
- Memory：长期记忆。
- Schedule task：任务、执行历史和结果。
- Task stream：运行中事件流、队列快照、interrupt/ask 状态。

约束：

- HTTP、CLI、channel、scheduler 各自持有入口实例运行态。
- 跨入口共享状态必须通过 store/service，不通过包级变量。
- 核心用例不直接拼文件路径。
- 文件系统、网络、进程、数据库等实现属于 adapter。
- `internal/runtime/checkpoint` 只提供运行时无关 checkpoint 能力。

## Hooks

Hooks 是用例和运行内核之间的稳定扩展边界。

Hook point 包括：

- before/after turn
- before/after model request
- before/after tool call
- on event
- before/after memory injection
- before/after schedule execution

约束：

- hook payload 必须在 `internal/ports/hooks` 定义为明确结构体。
- payload 必须实现 `hooks.Payload`。
- `Invocation` 和 `Result` 不能把裸 `any` 当作契约。
- HookBus 由用例或组合根显式传入。
- 未传入 HookBus 时不执行 hook，也不提供可注册的全局默认实例。

## 配置与组合根

配置由 `internal/app/config` 管理，并通过快照或明确依赖传给用例、工具和 adapter。

组合根职责：

- 通过实例级 `ExecutionDependencies` 统一装配入口共享的执行依赖。
- 创建 runtime adapter。
- 创建模型注册表和 provider。
- 创建工具注册表并注入 `ToolResolveContext`。
- 创建 scheduler、memory、history、channel、skill 等服务实例。
- 将服务实例注入 HTTP runtime、CLI session、channel bridge 和 scheduler executor。
- 注册生命周期服务并按 LIFO 顺序停止。

组合根不得：

- 通过空白 import 或隐式初始化完成装配。
- 在注册失败时 panic。
- 暴露可变进程级默认注册表。
- 让 adapter 反向调用 app 入口层。

## 传输层

传输层只做协议转换：

- HTTP/Web/API：请求响应、流式事件、WebSocket、静态资源。
- CLI：终端输入、展示、交互控制。
- Channel：外部消息平台的消息收发和身份映射。

传输层必须调用 app service，不得绕过用例层直接调用 runtime、history、memory、scheduler 或 tool adapter。

## 调度

调度用例由 `internal/app/schedule` 提供，调度器实现由 adapter 提供。

约束：

- schedule 工具只委托 app schedule service。
- 后台 tasker 使用轻量 profile，不隐式加载完整交互结构。
- 调度执行结果通过 collector 收集事件并归档。
- 调度服务由组合根显式注入，不提供进程级默认实例。

## 架构门禁

必须持续维护边界测试：

- `internal/app` 不得 import `internal/adapters` 或具体传输、展示、运行实现。
- `internal/ports` 不得 import `internal/app`、`internal/runtime` 或 `internal/adapters`。
- `internal/ports/runtime` 不暴露具体 runtime SDK 类型。
- `internal/runtime` 不依赖入口层或 adapter。
- 入口层不得绕过 app service 直接调用 runtime adapter。
- `app/chat` 不直接依赖具体 storage adapter。
- `app/agent/catalog` 不直接依赖入口层状态。
- standalone agent 不需要完整 app runtime、history、summary、skills 或默认工具。
- 工具不得反向调用 `app/chat` 或传输展示层。

验证命令：

```bash
go test ./...
go build ./...
go vet ./...
git diff --check
```

提交粒度按模块切分，提交信息使用 Conventional Commits 中文说明。
