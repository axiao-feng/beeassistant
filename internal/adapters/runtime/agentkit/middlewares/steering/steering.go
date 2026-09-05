package steering

import (
	"context"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	runtimeport "fkteams/internal/ports/runtime"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func New() runtimeport.AgentMiddleware {
	return kitruntime.WrapAgentMiddleware("steering", &handler{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
	})
}

type handler struct {
	*adk.BaseChatModelAgentMiddleware
}

func (h *handler) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	source, ok := runtimeport.SteeringSourceFromContext(ctx)
	if !ok {
		return ctx, state, nil
	}
	messages, err := source(ctx)
	if err != nil {
		return ctx, nil, fmt.Errorf("consume steering: %w", err)
	}
	if len(messages) == 0 {
		return ctx, state, nil
	}

	next := *state
	next.Messages = append(append([]*schema.Message(nil), state.Messages...), kitruntime.AdaptMessages(messages)...)
	return ctx, &next, nil
}
