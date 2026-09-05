package autocontinue

import (
	"context"
	"testing"

	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	domainevent "fkteams/internal/domain/event"
	domainmessage "fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	kit "github.com/wsshow/agentkit"
)

func TestRunnerPublishesRepairedArgumentsAndContinues(t *testing.T) {
	ctx := context.Background()
	var received string
	tool, err := runtimeport.InferTool("record", "记录文本", func(_ context.Context, input *continueToolInput) (string, error) {
		received = input.Prompt
		return "recorded", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	model := kit.NewMockChatModel(kit.MockModelMessage(&schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{ID: "partial", Type: "function", Function: schema.FunctionCall{
			Name: "record", Arguments: `{"prompt":"hello`,
		}}},
		ResponseMeta: &schema.ResponseMeta{FinishReason: "length"},
	}), kit.MockModelText("continued"))
	agent, err := kitruntime.NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name: "continuation", Model: kitruntime.WrapChatModel(toolCallingMock{model}),
		Tools: []runtimeport.Tool{tool}, Middlewares: []runtimeport.AgentMiddleware{handler},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := kitruntime.NewRunnerFromConfig(ctx, runtimeport.RunnerConfig{Agent: agent, EnableStreaming: true})
	if err != nil {
		t.Fatal(err)
	}
	var output []domainevent.Event
	_, err = runner.Run(ctx, domainmessage.TurnInput{Message: domainmessage.Message{Role: domainmessage.RoleUser, Content: "go"}}, runtimeport.RunOptions{Sink: func(event domainevent.Event) error {
		output = append(output, event)
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if received != "hello" {
		t.Fatalf("tool received %q", received)
	}
	var started, completed, continued bool
	for _, event := range output {
		if event.Type == domainevent.TypeToolCallStarted && event.ToolName == "record" {
			started = true
			if event.ToolArgs != `{"prompt":"hello"}` || event.ToolCallRef != "tool_call:partial" {
				t.Fatalf("start differs from execution: %#v", event)
			}
		}
		if event.Type == domainevent.TypeAssistantCompleted {
			for _, call := range event.ToolCalls {
				if call.ID == "partial" {
					completed = true
					if call.Function.Arguments != `{"prompt":"hello"}` {
						t.Fatalf("history contains unrepaired call: %#v", call)
					}
				}
			}
			continued = continued || event.Content == "continued"
		}
	}
	if !started || !completed || !continued {
		t.Fatalf("missing repaired events or continuation: start=%v complete=%v continued=%v", started, completed, continued)
	}
}

type toolCallingMock struct{ *kit.MockChatModel }

func (m toolCallingMock) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
