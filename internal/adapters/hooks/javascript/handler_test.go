package javascript

import (
	"context"
	"strings"
	"testing"

	"fkteams/internal/app/config"
	"fkteams/internal/domain/message"
	hookport "fkteams/internal/ports/hooks"
)

func TestHandlerRewritesToolArguments(t *testing.T) {
	handler := NewHandler(func() []config.JavaScriptHook {
		return []config.JavaScriptHook{{
			ID:         "rewrite",
			Enabled:    true,
			HookPoints: []string{"before_tool_call"},
			Source: `function handle(hook) {
  hook.payload.args = JSON.stringify({value: 42});
  return {payload: hook.payload};
}`,
		}}
	})
	result, err := handler.Handle(context.Background(), hookport.Invocation{
		HookPoint: hookport.HookBeforeToolCall,
		Payload: hookport.BeforeToolCallPayload{
			ToolName: "sample",
			Args:     `{"value":1}`,
		},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	payload := result.Payload.(hookport.BeforeToolCallPayload)
	if payload.Args != `{"value":42}` || payload.ToolName != "sample" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestHandlerCanRejectToolCall(t *testing.T) {
	handler := NewHandler(func() []config.JavaScriptHook {
		return []config.JavaScriptHook{{
			ID:         "guard",
			Enabled:    true,
			HookPoints: []string{"before_tool_call"},
			Source:     `function handle() { return {action: "reject", message: "blocked"}; }`,
		}}
	})
	result, err := handler.Handle(context.Background(), hookport.Invocation{
		HookPoint: hookport.HookBeforeToolCall,
		Payload:   hookport.BeforeToolCallPayload{ToolName: "execute"},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.Action != hookport.ActionReject || result.Message != "blocked" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHandlerRewritesBeforeRunInput(t *testing.T) {
	handler := NewHandler(func() []config.JavaScriptHook {
		return []config.JavaScriptHook{{
			ID:         "prefix",
			Enabled:    true,
			HookPoints: []string{"before_run"},
			Source: `function handle(hook) {
  hook.payload.input.message.content = "prefix: " + hook.payload.input.message.content;
  return {payload: hook.payload};
}`,
		}}
	})
	result, err := handler.Handle(context.Background(), hookport.Invocation{
		HookPoint: hookport.HookBeforeRun,
		Payload: hookport.BeforeRunPayload{Input: message.TurnInput{
			Message: message.Message{Role: message.RoleUser, Content: "hello"},
		}},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	payload := result.Payload.(hookport.BeforeRunPayload)
	if payload.Input.Message.Content != "prefix: hello" {
		t.Fatalf("content = %q", payload.Input.Message.Content)
	}
}

func TestValidateDefinitionsRejectsMissingHandle(t *testing.T) {
	err := ValidateDefinitions([]config.JavaScriptHook{{
		ID:      "invalid",
		Enabled: true,
		Source:  `function other() {}`,
	}})
	if err == nil || !strings.Contains(err.Error(), "must define function handle") {
		t.Fatalf("ValidateDefinitions() error = %v", err)
	}
}
