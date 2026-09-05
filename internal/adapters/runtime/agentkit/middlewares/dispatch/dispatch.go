// Package dispatch 提供子任务并行分发中间件，向父智能体注入 dispatch_tasks 工具，
// 将独立子任务下发到子智能体并行执行。
package dispatch

import (
	"context"
	"fmt"
	"time"

	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	"fkteams/internal/adapters/runtime/agentkit/middlewares/agentsmd"
	runtimeport "fkteams/internal/ports/runtime"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	defaultMaxConcurrency = 3
	defaultTaskTimeout    = 30 * time.Minute
)

// Config 分发中间件配置。未指定工具时子智能体自动继承父智能体的工具。
type Config struct {
	Model          runtimeport.ChatModel // 子任务模型（由 agent definition resolver 自动填充）
	Tools          []runtimeport.Tool    // 子智能体工具实例；为空时继承父智能体工具
	MaxConcurrency int                   // 最大并发数（默认 3）
	TaskTimeout    time.Duration         // 单任务超时（默认 30min）
}

func (c *Config) defaults() {
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = defaultMaxConcurrency
	}
	if c.TaskTimeout <= 0 {
		c.TaskTimeout = defaultTaskTimeout
	}
}

// New 创建分发中间件
func New(ctx context.Context, cfg *Config) (runtimeport.AgentMiddleware, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("dispatch: Model is required")
	}
	cfg.defaults()
	chatModel, err := kitruntime.AdaptChatModelForRunner(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("dispatch: adapt model: %w", err)
	}

	runnerTools, err := kitruntime.AdaptToolsForRunner(ctx, cfg.Tools)
	if err != nil {
		return nil, fmt.Errorf("dispatch: adapt tools: %w", err)
	}
	agentsMDMiddleware, err := agentsmd.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("dispatch: init agents.md middleware: %w", err)
	}
	agentsMDHandler, err := kitruntime.AdaptAgentMiddlewareForRunner(agentsMDMiddleware)
	if err != nil {
		return nil, fmt.Errorf("dispatch: adapt agents.md middleware: %w", err)
	}

	return kitruntime.WrapAgentMiddleware("dispatch", &middleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		chatModel:                    chatModel,
		tools:                        runnerTools,
		handlers:                     []adk.ChatModelAgentMiddleware{agentsMDHandler},
		maxConcurrency:               int64(cfg.MaxConcurrency),
		taskTimeout:                  cfg.TaskTimeout,
	}), nil
}

type middleware struct {
	*adk.BaseChatModelAgentMiddleware
	chatModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	handlers       []adk.ChatModelAgentMiddleware
	maxConcurrency int64
	taskTimeout    time.Duration
}

// BeforeAgent 注入 dispatch_tasks 工具和提示词，未配置工具时继承父智能体的工具
func (m *middleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	local := *m
	if len(local.tools) == 0 {
		local.tools = append([]tool.BaseTool(nil), runCtx.Tools...)
	}

	t, err := utils.InferTool("dispatch_tasks", toolDesc, local.executeTasks)
	if err != nil {
		return ctx, runCtx, fmt.Errorf("dispatch: create tool: %w", err)
	}
	runCtx.Tools = append(runCtx.Tools, t)
	runCtx.Instruction += dispatchPrompt
	return ctx, runCtx, nil
}
