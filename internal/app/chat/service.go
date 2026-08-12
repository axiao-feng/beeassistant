package chat

import (
	"context"
	"fmt"

	"fkteams/internal/app/tools/ask"
	"fkteams/internal/domain/event"
	"fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/runtime/approval"
	"fkteams/internal/runtime/hooks"
	"fkteams/internal/runtime/turn"
)

// EventHandler 处理一次对话运行期间产生的领域事件。
type EventHandler func(event.Event) error

// ContextHook 在运行前补充上下文能力，例如转向输入和请求级元数据。
type ContextHook func(context.Context) context.Context

// TurnRequest 描述一次用户输入到运行时执行的最小请求。
type TurnRequest struct {
	SessionID        string
	RunID            string
	Runner           runtimeport.Runner
	Input            message.TurnInput
	EventSink        EventHandler
	Summary          turn.SummarySink
	InterruptHandler runtimeport.InterruptHandler
	NonInteractive   bool
	ApprovalRegistry *approval.Registry
	SteeringSource   runtimeport.SteeringSource
	AskHandler       ask.RuntimeHandler
	HookBus          *hooks.Bus
	ContextHooks     []ContextHook
	OnFinish         func(ctx context.Context, result *runtimeport.RunResult, err error)
}

// Service 是所有入口共享的聊天用例服务。
type Service struct{}

// NewService 创建聊天用例服务。
func NewService() *Service {
	return &Service{}
}

// RunTurn 执行一次对话回合，并将入口层能力转换为运行时选项。
func (s *Service) RunTurn(ctx context.Context, req TurnRequest) (*runtimeport.RunResult, error) {
	if req.Runner == nil {
		return nil, fmt.Errorf("chat turn runner is nil")
	}
	if req.SessionID == "" {
		return nil, fmt.Errorf("chat turn session ID is empty")
	}
	if req.HookBus == nil {
		req.HookBus = hooks.FromContext(ctx)
	}

	contextHooks := append([]ContextHook(nil), req.ContextHooks...)
	if req.ApprovalRegistry != nil {
		contextHooks = append(contextHooks, func(ctx context.Context) context.Context {
			return approval.WithRegistry(ctx, req.ApprovalRegistry)
		})
	}
	if req.SteeringSource != nil {
		contextHooks = append(contextHooks, func(ctx context.Context) context.Context {
			return runtimeport.WithSteeringSource(ctx, req.SteeringSource)
		})
	}
	if req.AskHandler != nil {
		contextHooks = append(contextHooks, func(ctx context.Context) context.Context {
			return ask.WithRuntimeHandler(ctx, req.AskHandler)
		})
	}

	turnHooks := make([]turn.ContextHook, 0, len(contextHooks))
	for _, hook := range contextHooks {
		if hook != nil {
			turnHooks = append(turnHooks, turn.ContextHook(hook))
		}
	}

	return turn.NewExecutor().Run(ctx, turn.Request{
		Runner:         req.Runner,
		SessionID:      req.SessionID,
		RunID:          req.RunID,
		Input:          req.Input,
		EventSink:      req.EventSink,
		Summary:        req.Summary,
		OnInterrupt:    turn.InterruptHandler(req.InterruptHandler),
		NonInteractive: req.NonInteractive,
		ContextHooks:   turnHooks,
		HookBus:        req.HookBus,
		OnFinish:       req.OnFinish,
	})
}
