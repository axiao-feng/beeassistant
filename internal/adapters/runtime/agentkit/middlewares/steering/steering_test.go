package steering

import (
	"context"
	domainmessage "fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestBeforeModelRewriteStateAppendsSteeringMessages(t *testing.T) {
	h := &handler{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
	ctx := runtimeport.WithSteeringSource(context.Background(), func(context.Context) ([]domainmessage.Message, error) {
		return []domainmessage.Message{{Role: domainmessage.RoleUser, Content: "stop and do this"}}, nil
	})
	state := &adk.ChatModelAgentState{
		Messages: []*schema.Message{{Role: schema.User, Content: "start"}},
	}

	_, next, err := h.BeforeModelRewriteState(ctx, state, nil)
	if err != nil {
		t.Fatalf("expected steering middleware to succeed: %v", err)
	}

	if len(next.Messages) != 2 {
		t.Fatalf("expected steering message to be appended, got %d messages", len(next.Messages))
	}
	if next.Messages[1].Role != schema.User || next.Messages[1].Content != "stop and do this" {
		t.Fatalf("unexpected steering message: %#v", next.Messages[1])
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected original state to remain unchanged, got %d messages", len(state.Messages))
	}
}
