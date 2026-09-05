package runtime

import (
	"context"
	"fkteams/internal/domain/event"
	"fkteams/internal/domain/message"
	storageport "fkteams/internal/ports/storage"
	"fmt"
	"time"
)

// Agent 是 runtime 无关的智能体实例。
type Agent interface {
	Name() string
	Description() string
}

// AgentToolNameFunc 为成员智能体生成工具名。
type AgentToolNameFunc func(displayName string, index int) string

// AgentToolDisplayFunc 记录工具名和成员展示名的映射。
type AgentToolDisplayFunc func(toolName, displayName string)

// AgentToolConfig 描述把子智能体暴露为工具所需的命名策略。
type AgentToolConfig struct {
	ToolName        AgentToolNameFunc
	RegisterDisplay AgentToolDisplayFunc
}

// UnknownToolHandler 处理模型调用未知工具时的兜底响应。
type UnknownToolHandler func(ctx context.Context, name, arguments string) (string, error)

// RetryContext 描述一次模型重试判断的上下文。
type RetryContext struct {
	Err error
}

// RetryDecision 描述模型请求失败后的重试决策。
type RetryDecision struct {
	Retry        bool
	RejectReason string
}

// ModelRetryConfig 定义模型请求重试策略。
type ModelRetryConfig struct {
	MaxRetries  int
	ShouldRetry func(ctx context.Context, retryCtx *RetryContext) *RetryDecision
}

// DeepPlanningConfig 描述深度智能体的计划能力。
type DeepPlanningConfig struct {
	Enabled bool
}

// DeepWorkspaceConfig 描述深度智能体的工作区文件能力。
type DeepWorkspaceConfig struct {
	Dir     string
	Enabled bool
}

// DeepShellConfig 描述深度智能体的命令执行能力。
type DeepShellConfig struct {
	Enabled   bool
	Streaming bool
	Timeout   time.Duration
}

// DeepDelegationConfig 描述深度智能体的子任务委派能力。
type DeepDelegationConfig struct {
	GeneralAgent        bool
	TaskToolDescription string
}

// DeepOutputConfig 描述深度智能体的输出写入能力。
type DeepOutputConfig struct {
	Key string
}

// ChatAgentConfig 是创建单模型工具智能体的 runtime 无关配置。
type ChatAgentConfig struct {
	Name               string
	Description        string
	Instruction        string
	Model              ChatModel
	Tools              []Tool
	ToolMiddlewares    []ToolMiddleware
	UnknownToolHandler UnknownToolHandler
	Middlewares        []AgentMiddleware
	ModelRetryConfig   *ModelRetryConfig
	MaxIterations      int
}

// Validate 校验 ChatAgentConfig 的最低契约。
func (cfg *ChatAgentConfig) Validate() error {
	if cfg == nil {
		return fmt.Errorf("chat agent config is nil")
	}
	if cfg.Name == "" {
		return fmt.Errorf("chat agent name is required")
	}
	if cfg.Model == nil {
		return fmt.Errorf("chat agent model is required")
	}
	return nil
}

// LoopAgentConfig 是创建循环协作智能体的 runtime 无关配置。
type LoopAgentConfig struct {
	Name          string
	Description   string
	SubAgents     []Agent
	MaxIterations int
}

// Validate 校验 LoopAgentConfig 的最低契约。
func (cfg *LoopAgentConfig) Validate() error {
	if cfg == nil {
		return fmt.Errorf("loop agent config is nil")
	}
	if cfg.Name == "" {
		return fmt.Errorf("loop agent name is required")
	}
	if len(cfg.SubAgents) == 0 {
		return fmt.Errorf("loop agent sub agents are required")
	}
	return nil
}

// DeepAgentConfig 是创建深度协作智能体的 runtime 无关配置。
type DeepAgentConfig struct {
	Name             string
	Description      string
	Instruction      string
	Model            ChatModel
	Tools            []Tool
	ToolMiddlewares  []ToolMiddleware
	SubAgents        []Agent
	Middlewares      []AgentMiddleware
	ModelRetryConfig *ModelRetryConfig
	MaxIterations    int
	Planning         DeepPlanningConfig
	Workspace        DeepWorkspaceConfig
	Shell            DeepShellConfig
	Delegation       DeepDelegationConfig
	Output           DeepOutputConfig
}

// Validate 校验 DeepAgentConfig 的最低契约。
func (cfg *DeepAgentConfig) Validate() error {
	if cfg == nil {
		return fmt.Errorf("deep agent config is nil")
	}
	if cfg.Name == "" {
		return fmt.Errorf("deep agent name is required")
	}
	if cfg.Model == nil {
		return fmt.Errorf("deep agent model is required")
	}
	return nil
}

// RunnerConfig 描述创建 Runner 所需的最小配置。
type RunnerConfig struct {
	Agent           Agent
	EnableStreaming bool
	CheckpointStore storageport.CheckpointStore
}

// Validate 校验 RunnerConfig 的最低契约。
func (cfg RunnerConfig) Validate() error {
	if cfg.Agent == nil {
		return fmt.Errorf("runner agent is required")
	}
	return nil
}

// SummaryPersistCallback 保存摘要文本。
type SummaryPersistCallback func(summaryText string)

const DefaultMaxTokensBeforeSummary = 800 * 1024

type summaryPersistCallbackKey struct{}

func WithSummaryPersistCallback(ctx context.Context, cb SummaryPersistCallback) context.Context {
	return context.WithValue(ctx, summaryPersistCallbackKey{}, cb)
}

func SummaryPersistCallbackFromContext(ctx context.Context) (SummaryPersistCallback, bool) {
	cb, ok := ctx.Value(summaryPersistCallbackKey{}).(SummaryPersistCallback)
	return cb, ok
}

// SummaryConfig 描述摘要中间件配置。
type SummaryConfig struct {
	MaxTokensBeforeSummary int
	Model                  ChatModel
}

// DispatchConfig 描述子任务分发中间件配置。
type DispatchConfig struct {
	Model          ChatModel
	ToolNames      []string
	Tools          []Tool
	MaxConcurrency int
	TaskTimeout    time.Duration
}

// AgentRuntime 创建普通、循环和深度智能体。
type AgentRuntime interface {
	NewChatModelAgent(ctx context.Context, cfg *ChatAgentConfig) (Agent, error)
	NewLoopAgent(ctx context.Context, cfg *LoopAgentConfig) (Agent, error)
	NewDeepAgent(ctx context.Context, cfg *DeepAgentConfig) (Agent, error)
}

// RunnerRuntime 创建可执行 Runner。
type RunnerRuntime interface {
	NewRunner(ctx context.Context, cfg RunnerConfig) (Runner, error)
}

// AgentToolRuntime 将子智能体包装为可调用工具。
type AgentToolRuntime interface {
	NewAgentTools(ctx context.Context, subAgents []Agent, cfg AgentToolConfig) ([]Tool, error)
}

// Runtime 是组合根注入的最小可执行 adapter 能力集合。
// 消费方应优先依赖 AgentRuntime、RunnerRuntime、AgentToolRuntime 等小接口。
type Runtime interface {
	AgentRuntime
	RunnerRuntime
	AgentToolRuntime
}

// RuntimeHealth 描述 runtime adapter 的运行可用性。
type RuntimeHealth struct {
	Name    string
	Ready   bool
	Message string
}

// RuntimeInspector 暴露 runtime adapter 的健康检查。
type RuntimeInspector interface {
	CheckHealth(ctx context.Context) RuntimeHealth
}

// ModelDecorator 为模型附加 runtime 级增强能力。
type ModelDecorator interface {
	DecorateChatModel(ctx context.Context, model ChatModel) (ChatModel, error)
}

// PipelineRuntime 创建 runtime 默认中间件与可选能力中间件。
type PipelineRuntime interface {
	DefaultAgentMiddlewares(ctx context.Context) ([]AgentMiddleware, error)
	NewSteeringMiddleware() AgentMiddleware
	NewSummaryMiddleware(ctx context.Context, cfg *SummaryConfig) (AgentMiddleware, error)
	NewSkillsMiddleware(ctx context.Context) (AgentMiddleware, error)
	NewDispatchMiddleware(ctx context.Context, cfg *DispatchConfig) (AgentMiddleware, error)
	NewAgentsMDMiddleware(ctx context.Context) (AgentMiddleware, error)
	DefaultToolMiddlewares() []ToolMiddleware
}

// Interrupt 描述一次 HITL 中断请求。
type Interrupt struct {
	ID             string
	IsRootCause    bool
	Info           any
	MemberCallID   string
	MemberToolName string
	MemberName     string
	MemberOrder    *int
}

// InterruptDecisions 描述一次 HITL 恢复的目标决策映射，key 为 interrupt ID。
type InterruptDecisions map[string]any

// InterruptHandler 决定如何恢复一组运行中断。
type InterruptHandler func(ctx context.Context, interrupts []Interrupt) (InterruptDecisions, error)

// EventSink 接收 runtime 产生的领域事件。
type EventSink func(event.Event) error

// RunOptions 描述一次 Runner 执行的可选能力。
type RunOptions struct {
	RunID            string
	Sink             EventSink
	InterruptHandler InterruptHandler
}

// WithDefaults 填充 RunOptions 的安全默认值。
func (opts RunOptions) WithDefaults(defaultRunID string) RunOptions {
	if opts.RunID == "" {
		opts.RunID = defaultRunID
	}
	if opts.Sink == nil {
		opts.Sink = NoopEventSink
	}
	return opts
}

// NoopEventSink 丢弃事件。
func NoopEventSink(event.Event) error {
	return nil
}

// RunResult 描述一次 Runner 执行结果。
type RunResult struct {
	LastEvent event.Event
}

// Runner 执行一次对话输入。
type Runner interface {
	Run(ctx context.Context, input message.TurnInput, opts RunOptions) (*RunResult, error)
}
