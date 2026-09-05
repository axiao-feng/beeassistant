package agentkit

import (
	"context"
	runtimeport "fkteams/internal/ports/runtime"
	"fmt"
)

// agentTool 是成员声明，构建父智能体时转换成 AgentKit SubAgentConfig。
// 不维护第二套成员执行器、检查点或事件转发协程。
type agentTool struct {
	agent *agent
	name  string
}

// NewAgentTools 为团队成员生成工具声明和展示名称。
func NewAgentTools(_ context.Context, agents []runtimeport.Agent, cfg runtimeport.AgentToolConfig) ([]runtimeport.Tool, error) {
	result := make([]runtimeport.Tool, 0, len(agents))
	for i, value := range agents {
		a, err := requireAgent(value)
		if err != nil {
			return nil, err
		}
		name := a.Name()
		if cfg.ToolName != nil {
			name = cfg.ToolName(name, i)
		}
		if cfg.RegisterDisplay != nil {
			cfg.RegisterDisplay(name, a.Name())
		}
		result = append(result, &agentTool{agent: a, name: name})
	}
	return result, nil
}

func (t *agentTool) description() string {
	return fmt.Sprintf("指派给 %s 处理独立子任务。request 应包含目标、必要上下文和完成标准；等待结果后再整合。能力：%s", t.agent.Name(), t.agent.Description())
}

func (t *agentTool) Info(context.Context) (*runtimeport.ToolInfo, error) {
	return &runtimeport.ToolInfo{Name: t.name, Desc: t.description()}, nil
}

func (t *agentTool) Invoke(context.Context, runtimeport.ToolInvocation) (*runtimeport.ToolResult, error) {
	return nil, fmt.Errorf("member tool %q must be bound to an agent", t.name)
}
