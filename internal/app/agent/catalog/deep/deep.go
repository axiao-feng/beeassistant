package deep

import (
	"context"
	"fkteams/internal/app/agent/catalog/common"
	"fkteams/internal/app/config"
	"fkteams/internal/app/tools"
	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/runtime/env"
	retry "fkteams/internal/runtime/retry"
	"fkteams/internal/runtime/toolpolicy"
	"fmt"
	"strconv"
	"time"
)

func NewAgent(ctx context.Context, subAgents []runtimeport.Agent) (runtimeport.Agent, error) {
	deepCfg := config.Get().Deep.WithDefaults()

	toolList, err := tools.GetBuiltinCapabilityTools(ctx)
	if err != nil {
		return nil, err
	}
	for _, toolName := range uniqueToolNames(deepCfg.ExtraTools) {
		baseTools, err := tools.GetToolsByName(ctx, toolName)
		if err != nil {
			return nil, fmt.Errorf("init tool %s: %w", toolName, err)
		}
		if err := toolpolicy.MarkPolicyRequired(baseTools); err != nil {
			return nil, fmt.Errorf("mark tool policy %s: %w", toolName, err)
		}
		toolList = append(toolList, baseTools...)
	}
	if err := toolpolicy.ClassifyTools(toolList); err != nil {
		return nil, fmt.Errorf("classify tools: %w", err)
	}
	chatModel, err := common.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	agentRuntime, err := runtimeport.RequireAgentRuntime(ctx)
	if err != nil {
		return nil, err
	}
	pipelineRuntime, ok := runtimeport.PipelineRuntimeFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("runtime does not support deep agent middlewares")
	}

	middlewares := []runtimeport.AgentMiddleware{
		pipelineRuntime.NewSteeringMiddleware(),
	}
	if deepCfg.Context.Summary {
		maxTokens := runtimeport.DefaultMaxTokensBeforeSummary
		if v := env.Get(env.MaxTokensBeforeSummary); v != "" {
			if n, _ := strconv.Atoi(v); n > 0 {
				maxTokens = n
			}
		}
		summaryMiddleware, err := pipelineRuntime.NewSummaryMiddleware(ctx, &runtimeport.SummaryConfig{
			Model:                  chatModel,
			MaxTokensBeforeSummary: maxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("init summary middleware: %w", err)
		}
		middlewares = append(middlewares, summaryMiddleware)
	}
	if deepCfg.Context.AgentsMD {
		agentsMDMiddleware, err := pipelineRuntime.NewAgentsMDMiddleware(ctx)
		if err != nil {
			return nil, fmt.Errorf("init agents.md middleware: %w", err)
		}
		middlewares = append(middlewares, agentsMDMiddleware)
	}
	shellTimeout := 30 * time.Second
	if deepCfg.Shell.Timeout != "" {
		parsed, err := time.ParseDuration(deepCfg.Shell.Timeout)
		if err != nil {
			return nil, fmt.Errorf("parse deep shell timeout: %w", err)
		}
		if parsed > 0 {
			shellTimeout = parsed
		}
	}
	return agentRuntime.NewDeepAgent(ctx, &runtimeport.DeepAgentConfig{
		Name:             "deep_researcher",
		Description:      "深度研究智能体，负责深入分析问题并协调多个成员解决复杂任务。",
		Instruction:      deepCfg.Instruction,
		Model:            chatModel,
		ModelRetryConfig: retry.NewModelRetryConfig(),
		SubAgents:        subAgents,
		Tools:            toolList,
		MaxIterations:    deepMaxIterations(deepCfg.MaxIterations),
		Middlewares:      middlewares,
		ToolMiddlewares:  pipelineRuntime.DefaultToolMiddlewares(),
		Planning: runtimeport.DeepPlanningConfig{
			Enabled: deepCfg.Planning.Enabled,
		},
		Workspace: runtimeport.DeepWorkspaceConfig{
			Dir:     common.WorkspaceDir(),
			Enabled: deepCfg.Workspace.Enabled,
		},
		Shell: runtimeport.DeepShellConfig{
			Enabled:   deepCfg.Shell.Enabled,
			Streaming: deepCfg.Shell.Streaming,
			Timeout:   shellTimeout,
		},
		Delegation: runtimeport.DeepDelegationConfig{
			GeneralAgent:        deepCfg.Delegation.GeneralAgent,
			TaskToolDescription: deepCfg.Delegation.TaskToolDescription,
		},
		Output: runtimeport.DeepOutputConfig{
			Key: deepCfg.Output.Key,
		},
	})
}

func deepMaxIterations(configured int) int {
	if configured > 0 {
		return configured
	}
	return retry.MaxIterations()
}

func uniqueToolNames(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
