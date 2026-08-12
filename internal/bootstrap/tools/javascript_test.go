package tools

import (
	"context"
	"testing"

	javascripttool "fkteams/internal/adapters/tools/javascript"
	apptools "fkteams/internal/app/tools"
	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/runtime/hooks"
)

type nestedSearchInput struct {
	Query string `json:"query"`
}

func TestJavaScriptHostCallsRegisteredToolThroughHooks(t *testing.T) {
	var received string
	target, err := runtimeport.NewTool(runtimeport.ToolInfo{Name: "search", Desc: "test search"}, func(_ context.Context, input *nestedSearchInput) (map[string]string, error) {
		received = input.Query
		return map[string]string{"query": input.Query}, nil
	})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	registry := apptools.NewToolGroupRegistry()
	if err := registry.Register(apptools.ToolGroupRegistration{
		Info: apptools.ToolGroupInfo{
			Name:        "test-search",
			DisplayName: "测试搜索",
			Description: "测试嵌套工具调用",
			Category:    "测试",
		},
		Factory: func(apptools.ToolResolveContext) ([]runtimeport.Tool, error) {
			return []runtimeport.Tool{target}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	host := &javascriptHost{registry: registry}
	bus := hooks.NewBus()
	afterCalled := false
	bus.RegisterFunc("rewrite", []hooks.HookPoint{hooks.HookBeforeToolCall}, func(_ hooks.Context, invocation hooks.Invocation) (hooks.Result, error) {
		payload := invocation.Payload.(hooks.BeforeToolCallPayload)
		payload.Args = `{"query":"rewritten"}`
		return hooks.Result{Payload: payload}, nil
	}, hooks.Options{})
	bus.RegisterFunc("observe", []hooks.HookPoint{hooks.HookAfterToolCall}, func(_ hooks.Context, invocation hooks.Invocation) (hooks.Result, error) {
		payload := invocation.Payload.(hooks.AfterToolCallPayload)
		afterCalled = payload.Result == `{"query":"rewritten"}` && payload.Meta["script_id"] == "workflow"
		return hooks.Result{}, nil
	}, hooks.Options{})

	content, err := host.Call(hooks.WithBus(context.Background(), bus), javascripttool.ToolCallRequest{
		ScriptID: "workflow",
		Group:    "test-search",
		Tool:     "search",
		CallID:   "nested-1",
		Args:     map[string]any{"query": "original"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if received != "rewritten" || content != `{"query":"rewritten"}` || !afterCalled {
		t.Fatalf("Call() = received %q, content %q, after %v", received, content, afterCalled)
	}
}
