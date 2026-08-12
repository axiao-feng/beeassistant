package tools

import (
	"context"
	"testing"

	mcpadapter "fkteams/internal/adapters/tools/mcp"
	"fkteams/internal/app/config"
	apptools "fkteams/internal/app/tools"
	runtimeport "fkteams/internal/ports/runtime"
)

func TestBootstrapRegistersSchedulerToolGroup(t *testing.T) {
	registry, err := RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults should be idempotent: %v", err)
	}
	ctx := apptools.WithRegistry(context.Background(), registry)
	if !contains(apptools.BuiltinToolNames(ctx), "scheduler") {
		t.Fatal("scheduler tool group is not registered")
	}
	resolved, err := apptools.GetToolsByName(ctx, "scheduler")
	if err != nil {
		t.Fatalf("GetToolsByName returned error: %v", err)
	}
	if len(resolved) == 0 {
		t.Fatal("expected scheduler tools")
	}
	info, err := resolved[0].Info(context.Background())
	if err != nil {
		t.Fatalf("tool info: %v", err)
	}
	if info.Name != "schedule_add" {
		t.Fatalf("first scheduler tool = %q, want schedule_add", info.Name)
	}
}

func TestBootstrapRegistersGitToolGroup(t *testing.T) {
	registry, err := RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults should be idempotent: %v", err)
	}
	ctx := apptools.WithRegistry(context.Background(), registry)
	if !contains(apptools.BuiltinToolNames(ctx), "git") {
		t.Fatal("git tool group is not registered")
	}
	resolved, err := apptools.GetToolsByName(ctx, "git")
	if err != nil {
		t.Fatalf("GetToolsByName returned error: %v", err)
	}
	if len(resolved) == 0 {
		t.Fatal("expected git tools")
	}
	info, err := resolved[0].Info(context.Background())
	if err != nil {
		t.Fatalf("tool info: %v", err)
	}
	if info.Name != "git_init" {
		t.Fatalf("first git tool = %q, want git_init", info.Name)
	}
}

func TestBootstrapRegistersSSHToolGroup(t *testing.T) {
	registry, err := RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults should be idempotent: %v", err)
	}
	ctx := apptools.WithRegistry(context.Background(), registry)
	infos := apptools.BuiltinToolInfos(ctx)
	for _, info := range infos {
		if info.Name != "ssh" {
			continue
		}
		if info.DisplayName == "" || info.Category == "" || len(info.IncludedTools) == 0 {
			t.Fatalf("ssh tool info is incomplete: %#v", info)
		}
		return
	}
	t.Fatal("ssh tool group is not registered")
}

func TestBootstrapRegistersJavaScriptToolGroup(t *testing.T) {
	registry, err := RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults() error = %v", err)
	}
	if _, ok, err := registry.Resolve(context.Background(), "javascript", nil); err != nil || !ok {
		t.Fatalf("Resolve(javascript) = ok %v, error %v", ok, err)
	}
}

func TestJavaScriptToolGroupUsesResolveContextConfig(t *testing.T) {
	registry, err := RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults() error = %v", err)
	}
	cfg := &config.Config{JavaScript: config.JavaScriptSettings{Tools: []config.JavaScriptTool{{
		ID: "preview", Name: "预览", Description: "测试配置覆盖", Enabled: true,
		Source: `function execute(input) { return input; }`,
	}}}}
	ctx := apptools.WithResolveContextPatch(context.Background(), apptools.ToolResolveContext{Config: cfg})
	resolved, err := registry.GetToolsByName(ctx, "javascript")
	if err != nil {
		t.Fatalf("GetToolsByName() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("tool count = %d, want 1", len(resolved))
	}
	result, err := resolved[0].Invoke(ctx, runtimeport.ToolInvocation{Name: "preview", Arguments: `{"value":1}`})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Content != `{"value":1}` {
		t.Fatalf("Invoke() content = %s", result.Content)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
