// Package agentkit 将 AgentKit 的执行能力适配为应用运行端口。
package agentkit

import (
	runtimeport "fkteams/internal/ports/runtime"
	"fmt"
	kit "github.com/wsshow/agentkit"
)

// agent 保存可复用的声明；每次 Run 创建独立的 AgentKit 会话。
// 历史由应用持有，避免缓存的 Runner 在不同会话间共享对话状态。
type agent struct {
	config     kit.Config
	members    map[string]string
	loop       []*agent
	iterations int
}

func (a *agent) Name() string        { return a.config.Name }
func (a *agent) Description() string { return a.config.Description }

func requireAgent(value runtimeport.Agent) (*agent, error) {
	a, ok := value.(*agent)
	if !ok || a == nil {
		return nil, fmt.Errorf("unsupported agent: %T", value)
	}
	return a, nil
}
