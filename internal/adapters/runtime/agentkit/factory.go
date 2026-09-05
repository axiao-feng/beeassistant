package agentkit

import (
	"context"
	runtimeport "fkteams/internal/ports/runtime"
	"fmt"
	"github.com/cloudwego/eino/adk"
	kit "github.com/wsshow/agentkit"
)

// NewChatModelAgent 解析模型、工具和中间件，创建 AgentKit 智能体声明。
func NewChatModelAgent(ctx context.Context, cfg *runtimeport.ChatAgentConfig) (runtimeport.Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	model, err := AdaptChatModelForRunner(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("adapt chat model: %w", err)
	}
	handlers, err := AdaptAgentMiddlewaresForRunner(cfg.Middlewares)
	if err != nil {
		return nil, fmt.Errorf("adapt middleware: %w", err)
	}
	policy := &kit.ToolPolicy{UnknownTool: cfg.UnknownToolHandler, MaxResultChars: -1}
	for _, middleware := range cfg.ToolMiddlewares {
		if middleware == nil {
			continue
		}
		adapted, err := AdaptToolMiddlewareForRunner(middleware)
		if err != nil {
			return nil, fmt.Errorf("adapt tool middleware: %w", err)
		}
		policy.Middlewares = append(policy.Middlewares, adapted)
	}
	a := &agent{config: kit.Config{
		Name: cfg.Name, Description: cfg.Description, SystemPrompt: cfg.Instruction,
		Model: model, Handlers: handlers, ToolPolicy: policy,
		ModelRetryConfig: AdaptModelRetryConfigForRunner(cfg.ModelRetryConfig), MaxIterations: cfg.MaxIterations,
	}, members: make(map[string]string)}
	var tools []runtimeport.Tool
	for _, tool := range cfg.Tools {
		if member, ok := tool.(*agentTool); ok {
			child := member.agent.config
			if len(child.SubAgents) > 0 || len(member.agent.loop) > 0 {
				return nil, fmt.Errorf("nested team %q is not supported", child.Name)
			}
			a.config.SubAgents = append(a.config.SubAgents, kit.SubAgentConfig{
				Name: member.name, Description: member.description(), SystemPrompt: child.SystemPrompt,
				Model: child.Model, Tools: child.Tools, ToolPolicy: child.ToolPolicy, Handlers: child.Handlers,
				ModelRetryConfig: child.ModelRetryConfig, MaxIterations: child.MaxIterations,
			})
			a.members[member.name] = child.Name
		} else {
			tools = append(tools, tool)
		}
	}
	a.config.Tools, err = AdaptToolsForRunner(ctx, tools)
	if err != nil {
		return nil, fmt.Errorf("adapt tools: %w", err)
	}
	return a, nil
}

// NewLoopAgent 创建按成员顺序共享讨论上下文的循环。
func NewLoopAgent(_ context.Context, cfg *runtimeport.LoopAgentConfig) (runtimeport.Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.MaxIterations < 0 {
		return nil, fmt.Errorf("loop max iterations must not be negative")
	}
	result := &agent{config: kit.Config{Name: cfg.Name, Description: cfg.Description}, iterations: cfg.MaxIterations}
	for _, value := range cfg.SubAgents {
		member, err := requireAgent(value)
		if err != nil {
			return nil, err
		}
		if len(member.loop) > 0 {
			return nil, fmt.Errorf("nested discussion loops are not supported")
		}
		result.loop = append(result.loop, member)
	}
	return result, nil
}

// NewRunnerFromConfig 创建执行器；会话资源的生命周期由每次 Run 持有。
func NewRunnerFromConfig(_ context.Context, cfg runtimeport.RunnerConfig) (runtimeport.Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	a, err := requireAgent(cfg.Agent)
	if err != nil {
		return nil, err
	}
	return &Runner{agent: a, streaming: cfg.EnableStreaming, store: cfg.CheckpointStore}, nil
}

// AdaptModelRetryConfigForRunner 将应用重试策略转换为 AgentKit 配置。
func AdaptModelRetryConfigForRunner(cfg *runtimeport.ModelRetryConfig) *kit.ModelRetryConfig {
	if cfg == nil {
		return nil
	}
	result := &kit.ModelRetryConfig{MaxRetries: cfg.MaxRetries}
	if cfg.ShouldRetry != nil {
		result.ShouldRetry = func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
			var input *runtimeport.RetryContext
			if retryCtx != nil {
				input = &runtimeport.RetryContext{Err: retryCtx.Err}
			}
			decision := cfg.ShouldRetry(ctx, input)
			if decision == nil {
				return nil
			}
			return &adk.RetryDecision{Retry: decision.Retry, RejectReason: decision.RejectReason}
		}
	}
	return result
}
