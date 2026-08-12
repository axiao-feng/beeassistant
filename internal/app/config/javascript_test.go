package config

import "testing"

func TestValidateJavaScriptAcceptsTool(t *testing.T) {
	cfg := &Config{JavaScript: JavaScriptSettings{Tools: []JavaScriptTool{{
		ID:          "text_stats",
		Name:        "文本统计",
		Description: "统计文本",
		Parameters:  map[string]any{"type": "object"},
		Source:      `function execute(input) { return input; }`,
	}}}}
	if err := cfg.ValidateJavaScript(); err != nil {
		t.Fatalf("ValidateJavaScript() error = %v", err)
	}
}

func TestValidateJavaScriptAllowsLongerToolTimeoutThanHook(t *testing.T) {
	cfg := &Config{JavaScript: JavaScriptSettings{Tools: []JavaScriptTool{{
		ID: "workflow", Name: "工作流", Description: "长任务", TimeoutMS: 30_000,
	}}}}
	if err := cfg.ValidateJavaScript(); err != nil {
		t.Fatalf("ValidateJavaScript(tool) error = %v", err)
	}
	cfg.JavaScript = JavaScriptSettings{Hooks: []JavaScriptHook{{
		ID: "slow_hook", Name: "慢 Hook", HookPoints: []string{"before_run"}, TimeoutMS: 30_000,
	}}}
	if err := cfg.ValidateJavaScript(); err == nil {
		t.Fatal("ValidateJavaScript(hook) error = nil, want timeout validation error")
	}
}

func TestValidateJavaScriptRejectsInvalidToolSchema(t *testing.T) {
	cfg := &Config{JavaScript: JavaScriptSettings{Tools: []JavaScriptTool{{
		ID:          "bad tool",
		Name:        "无效工具",
		Description: "无效",
		Parameters:  map[string]any{"type": "string"},
	}}}}
	if err := cfg.ValidateJavaScript(); err == nil {
		t.Fatal("ValidateJavaScript() error = nil, want validation error")
	}
}

func TestValidateJavaScriptAcceptsHook(t *testing.T) {
	cfg := &Config{JavaScript: JavaScriptSettings{Hooks: []JavaScriptHook{{
		ID:          "guard",
		Name:        "保护规则",
		HookPoints:  []string{"before_tool_call"},
		ErrorPolicy: "fail",
		Source:      `function handle(hook) { return {action: "continue"}; }`,
	}}}}
	if err := cfg.ValidateJavaScript(); err != nil {
		t.Fatalf("ValidateJavaScript() error = %v", err)
	}
}

func TestValidateJavaScriptRejectsUnknownHookPoint(t *testing.T) {
	cfg := &Config{JavaScript: JavaScriptSettings{Hooks: []JavaScriptHook{{
		ID:         "guard",
		Name:       "保护规则",
		HookPoints: []string{"unknown"},
	}}}}
	if err := cfg.ValidateJavaScript(); err == nil {
		t.Fatal("ValidateJavaScript() error = nil, want invalid hook point error")
	}
}
