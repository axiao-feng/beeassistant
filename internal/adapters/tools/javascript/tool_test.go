package javascript

import (
	"context"
	"testing"

	"fkteams/internal/app/config"
	runtimeport "fkteams/internal/ports/runtime"
)

func TestToolInvokesJavaScriptWithArgumentsAndContext(t *testing.T) {
	tool, err := NewTool(config.JavaScriptTool{
		ID:          "greet",
		Name:        "问候",
		Description: "生成问候语",
		Source: `function execute(input, context) {
  return {message: "hello " + input.name, tool: context.tool_name};
}`,
	})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	result, err := tool.Invoke(context.Background(), runtimeport.ToolInvocation{
		Name:      "greet",
		Arguments: `{"name":"Ada"}`,
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Content != `{"message":"hello Ada","tool":"greet"}` {
		t.Fatalf("Invoke() content = %s", result.Content)
	}
}

func TestNewToolRejectsMissingExecuteFunction(t *testing.T) {
	_, err := NewTool(config.JavaScriptTool{ID: "invalid", Source: `function other() {}`})
	if err == nil {
		t.Fatal("NewTool() error = nil, want missing execute error")
	}
}
