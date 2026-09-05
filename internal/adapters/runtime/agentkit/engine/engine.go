package engine

import (
	"context"

	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/agentsmd"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/autocontinue"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/dispatch"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/inject"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/skills"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/steering"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/summary"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/tools/destructiveguard"
	hooktools "fkteams/internal/adapters/runtime/agentkit/middlewares/tools/hooks"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/tools/trimresult"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/tools/warperror"
	runtimeport "fkteams/internal/ports/runtime"

	einoMCP "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/mark3labs/mcp-go/client"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) CheckHealth(ctx context.Context) runtimeport.RuntimeHealth {
	return runtimeport.RuntimeHealth{
		Name:  "agentkit",
		Ready: ctx.Err() == nil,
	}
}

func (e *Engine) NewChatModelAgent(ctx context.Context, cfg *runtimeport.ChatAgentConfig) (runtimeport.Agent, error) {
	return kitruntime.NewChatModelAgent(ctx, cfg)
}

func (e *Engine) NewLoopAgent(ctx context.Context, cfg *runtimeport.LoopAgentConfig) (runtimeport.Agent, error) {
	return kitruntime.NewLoopAgent(ctx, cfg)
}

func (e *Engine) NewDeepAgent(ctx context.Context, cfg *runtimeport.DeepAgentConfig) (runtimeport.Agent, error) {
	return kitruntime.NewDeepAgent(ctx, cfg)
}

func (e *Engine) NewRunner(ctx context.Context, cfg runtimeport.RunnerConfig) (runtimeport.Runner, error) {
	return kitruntime.NewRunnerFromConfig(ctx, cfg)
}

func (e *Engine) NewAgentTools(ctx context.Context, subAgents []runtimeport.Agent, cfg runtimeport.AgentToolConfig) ([]runtimeport.Tool, error) {
	return kitruntime.NewAgentTools(ctx, subAgents, cfg)
}

func (e *Engine) DecorateChatModel(ctx context.Context, chatModel runtimeport.ChatModel) (runtimeport.ChatModel, error) {
	return inject.NewForModel(chatModel)
}

func (e *Engine) DefaultAgentMiddlewares(ctx context.Context) ([]runtimeport.AgentMiddleware, error) {
	result := make([]runtimeport.AgentMiddleware, 0, 5)
	result = append(result, e.newToolErrorMiddleware())
	acMiddleware, err := e.newAutoContinueMiddleware()
	if err != nil {
		return nil, err
	}
	result = append(result, acMiddleware)
	result = append(result, e.newTrimResultMiddleware())
	result = append(result, e.NewSteeringMiddleware())
	return result, nil
}

func (e *Engine) DefaultToolMiddlewares() []runtimeport.ToolMiddleware {
	return []runtimeport.ToolMiddleware{
		e.newHookToolMiddleware(),
		e.newDestructiveGuardMiddleware(),
	}
}

func (e *Engine) newToolErrorMiddleware() runtimeport.AgentMiddleware {
	return warperror.NewHandler(nil)
}

func (e *Engine) newAutoContinueMiddleware() (runtimeport.AgentMiddleware, error) {
	return autocontinue.NewHandler()
}

func (e *Engine) newTrimResultMiddleware() runtimeport.AgentMiddleware {
	return trimresult.New(nil)
}

func (e *Engine) NewSteeringMiddleware() runtimeport.AgentMiddleware {
	return steering.New()
}

func (e *Engine) NewSummaryMiddleware(ctx context.Context, cfg *runtimeport.SummaryConfig) (runtimeport.AgentMiddleware, error) {
	if cfg == nil {
		return summary.New(ctx, nil)
	}
	return summary.New(ctx, &summary.Config{
		Model:                  cfg.Model,
		MaxTokensBeforeSummary: cfg.MaxTokensBeforeSummary,
	})
}

func (e *Engine) NewSkillsMiddleware(ctx context.Context) (runtimeport.AgentMiddleware, error) {
	return skills.New(ctx)
}

func (e *Engine) NewDispatchMiddleware(ctx context.Context, cfg *runtimeport.DispatchConfig) (runtimeport.AgentMiddleware, error) {
	if cfg == nil {
		return dispatch.New(ctx, &dispatch.Config{})
	}
	return dispatch.New(ctx, &dispatch.Config{
		Model:          cfg.Model,
		Tools:          cfg.Tools,
		MaxConcurrency: cfg.MaxConcurrency,
		TaskTimeout:    cfg.TaskTimeout,
	})
}

func (e *Engine) NewAgentsMDMiddleware(ctx context.Context) (runtimeport.AgentMiddleware, error) {
	return agentsmd.New(ctx)
}

func (e *Engine) newDestructiveGuardMiddleware() runtimeport.ToolMiddleware {
	return destructiveguard.New()
}

func (e *Engine) newHookToolMiddleware() runtimeport.ToolMiddleware {
	return hooktools.New()
}

func (e *Engine) MCPTools(ctx context.Context, cli *client.Client) ([]runtimeport.Tool, error) {
	tools, err := einoMCP.GetTools(ctx, &einoMCP.Config{Cli: cli})
	if err != nil {
		return nil, err
	}
	result := make([]runtimeport.Tool, 0, len(tools))
	for _, t := range tools {
		result = append(result, kitruntime.WrapTool(t))
	}
	return result, nil
}
