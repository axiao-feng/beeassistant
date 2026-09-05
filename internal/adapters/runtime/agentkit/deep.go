package agentkit

import (
	"context"
	"encoding/json"
	"fmt"

	"fkteams/internal/adapters/runtime/agentkit/middlewares/fkfs"
	runtimeport "fkteams/internal/ports/runtime"
	"github.com/cloudwego/eino/adk"
	fsport "github.com/cloudwego/eino/adk/filesystem"
	filesystem "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/components/tool/utils"
	kit "github.com/wsshow/agentkit"
)

// NewDeepAgent 在同一 AgentKit 执行模型上组合规划、工作区和成员委派能力。
func NewDeepAgent(ctx context.Context, cfg *runtimeport.DeepAgentConfig) (runtimeport.Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if (cfg.Workspace.Enabled || cfg.Shell.Enabled) && cfg.Workspace.Dir == "" {
		return nil, fmt.Errorf("deep workspace directory is required")
	}
	memberTools, err := NewAgentTools(ctx, cfg.SubAgents, runtimeport.AgentToolConfig{})
	if err != nil {
		return nil, err
	}
	instruction := cfg.Instruction
	if instruction == "" {
		instruction = "你是深度研究智能体。先拆解问题并制定计划，按需委派独立子任务，验证证据并整合结论。"
	}
	instruction += "\n" + cfg.Delegation.TaskToolDescription
	value, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name: cfg.Name, Description: cfg.Description, Instruction: instruction, Model: cfg.Model,
		Tools:       append(append([]runtimeport.Tool(nil), cfg.Tools...), memberTools...),
		Middlewares: cfg.Middlewares, ToolMiddlewares: cfg.ToolMiddlewares,
		ModelRetryConfig: cfg.ModelRetryConfig, MaxIterations: cfg.MaxIterations,
	})
	if err != nil {
		return nil, err
	}
	a := value.(*agent)
	if cfg.Planning.Enabled {
		plan, err := utils.InferTool("write_todos", "更新任务计划，条目状态为 pending、in_progress 或 completed。", func(ctx context.Context, input *planInput) (string, error) {
			for _, item := range input.Todos {
				switch item.Status {
				case "pending", "in_progress", "completed":
				default:
					return `{"error":"invalid todo status"}`, nil
				}
			}
			data, err := json.Marshal(input.Todos)
			if err != nil {
				return "", err
			}
			kit.SetRunValue(ctx, "deep_todos", string(data))
			return string(data), nil
		})
		if err != nil {
			return nil, fmt.Errorf("create planning tool: %w", err)
		}
		a.config.Tools = append(a.config.Tools, plan)
	}
	if cfg.Workspace.Enabled {
		backend, err := fkfs.NewLocalBackend(cfg.Workspace.Dir)
		if err != nil {
			return nil, fmt.Errorf("init deep workspace backend: %w", err)
		}
		handler, err := filesystem.New(ctx, &filesystem.MiddlewareConfig{Backend: backend})
		if err != nil {
			return nil, fmt.Errorf("create workspace middleware: %w", err)
		}
		a.config.Handlers = append(a.config.Handlers, handler)
	}
	if cfg.Shell.Enabled {
		shell := fkfs.NewLocalShell(cfg.Workspace.Dir, cfg.Shell.Timeout)
		if cfg.Shell.Streaming {
			shell.OnOutput = kit.EmitToolUpdate
		}
		execute, err := utils.InferTool("execute", "在工作区执行命令，危险命令会被拒绝。", func(ctx context.Context, input *executeInput) (string, error) {
			result, err := shell.Execute(ctx, &fsport.ExecuteRequest{Command: input.Command})
			if err != nil {
				return "", err
			}
			output, err := json.Marshal(result)
			return string(output), err
		})
		if err != nil {
			return nil, fmt.Errorf("create shell tool: %w", err)
		}
		a.config.Tools = append(a.config.Tools, execute)
	}
	if cfg.Delegation.GeneralAgent {
		a.config.SubAgents = append(a.config.SubAgents, kit.SubAgentConfig{
			Name: "general_purpose", Description: "处理独立的通用研究子任务。", SystemPrompt: instruction,
			Model: a.config.Model, Tools: a.config.Tools, ToolPolicy: a.config.ToolPolicy,
			Handlers: append([]kit.ChatModelAgentMiddleware(nil), a.config.Handlers...), MaxIterations: cfg.MaxIterations,
		})
		a.members["general_purpose"] = "general_purpose"
	}
	if cfg.Output.Key != "" {
		a.config.Handlers = append(a.config.Handlers, &outputHandler{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}, key: cfg.Output.Key})
	}
	return a, nil
}

type executeInput struct {
	Command string `json:"command"`
}

type planInput struct {
	Todos []planItem `json:"todos"`
}
type planItem struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status" jsonschema:"enum=pending,enum=in_progress,enum=completed"`
}

type outputHandler struct {
	*adk.BaseChatModelAgentMiddleware
	key string
}

func (h *outputHandler) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if len(state.Messages) > 0 {
		kit.SetRunValue(ctx, h.key, state.Messages[len(state.Messages)-1].Content)
	}
	return ctx, state, nil
}
