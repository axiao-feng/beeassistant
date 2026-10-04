package coordinator

import (
	"context"
	"fkteams/internal/app/agent/catalog/common"
	runtimeport "fkteams/internal/ports/runtime"
)

func DefaultDefinition(agentTools ...runtimeport.Tool) common.Definition {
	return common.Definition{
		Name:        "coordinator",
		Description: "个人综合助手，直接处理日常任务，并在确有必要时调用专业成员。",
		Instruction: coordinatorPrompt,
		Profile:     common.ProfileTeam,
		TemplateVars: map[string]any{
			"workspace_dir": common.WorkspaceDir(),
		},
		ToolNames: []string{"todo", "file", "command", "scheduler", "ask", "javascript"},
		Tools:     agentTools,
	}
}

func NewAgent(ctx context.Context, agentTools ...runtimeport.Tool) (runtimeport.Agent, error) {
	return common.BuildAgent(ctx, DefaultDefinition(agentTools...))
}
