// Package fkteamshelper 提供 fkteams 项目使用与配置答疑智能体。
package fkteamshelper

import (
	"context"

	"fkteams/internal/app/agent/catalog/common"
	runtimeport "fkteams/internal/ports/runtime"
)

// DefaultDefinition 返回蜜蜂助手帮助的默认定义。
func DefaultDefinition() common.Definition {
	return common.Definition{
		Name:        "fkteams_helper",
		Description: "fkteams 项目助手，解答安装、配置、使用、扩展、架构和故障排查问题。",
		Instruction: helperPrompt,
		Profile:     common.ProfileBare,
		ToolNames:   []string{"search", "fetch"},
	}
}

// NewAgent 创建蜜蜂助手帮助。
func NewAgent(ctx context.Context) (runtimeport.Agent, error) {
	return common.BuildAgent(ctx, DefaultDefinition())
}
