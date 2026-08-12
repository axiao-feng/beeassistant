package handler

import (
	"context"
	"testing"

	mcpadapter "fkteams/internal/adapters/tools/mcp"
	"fkteams/internal/app/config"
	bootstraptools "fkteams/internal/bootstrap/tools"
)

func TestJavaScriptToolTestRunsDraftAndCollectsNotices(t *testing.T) {
	registry, err := bootstraptools.RegisterDefaults(mcpadapter.NewProvider())
	if err != nil {
		t.Fatalf("RegisterDefaults() error = %v", err)
	}
	resp, err := testJavaScriptTool(context.Background(), registry, javaScriptToolTestRequest{
		Tool: config.JavaScriptTool{
			ID:          "preview",
			Name:        "预览",
			Description: "试运行测试",
			Permissions: []string{config.JavaScriptPermissionEventNotice},
			Source: `function execute(input, context) {
  context.events.notice("preview completed", "info");
  return {value: input.value + 1};
}`,
		},
		Input: map[string]any{"value": 1},
	})
	if err != nil {
		t.Fatalf("testJavaScriptTool() error = %v", err)
	}
	if resp.Raw != `{"value":2}` {
		t.Fatalf("raw result = %q", resp.Raw)
	}
	if len(resp.Notices) != 1 || resp.Notices[0].Message != "preview completed" {
		t.Fatalf("notices = %#v", resp.Notices)
	}
}

func TestJavaScriptToolTestRejectsInvalidDefinition(t *testing.T) {
	_, err := testJavaScriptTool(context.Background(), nil, javaScriptToolTestRequest{
		Tool: config.JavaScriptTool{ID: "invalid id"},
	})
	if err == nil {
		t.Fatal("testJavaScriptTool() error = nil, want validation error")
	}
}
