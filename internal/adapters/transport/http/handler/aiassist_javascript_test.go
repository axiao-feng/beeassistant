package handler

import (
	"context"
	"testing"

	appaiassist "fkteams/internal/app/aiassist"
	"fkteams/internal/app/config"
	apptools "fkteams/internal/app/tools"
	runtimeport "fkteams/internal/ports/runtime"
)

func TestValidateJavaScriptDraftCompilesTool(t *testing.T) {
	err := validateJavaScriptDraft(appaiassist.JavaScriptDraftResponse{
		Kind: "tool",
		Tool: &config.JavaScriptTool{
			ID:          "text_stats",
			Name:        "文本统计",
			Description: "统计文本",
			Parameters:  map[string]any{"type": "object"},
			Source:      `function execute(input) { return input; }`,
		},
	})
	if err != nil {
		t.Fatalf("validateJavaScriptDraft() error = %v", err)
	}
}

func TestValidateJavaScriptDraftRejectsInvalidEntry(t *testing.T) {
	err := validateJavaScriptDraft(appaiassist.JavaScriptDraftResponse{
		Kind: "hook",
		Hook: &config.JavaScriptHook{
			ID:         "guard",
			Name:       "保护规则",
			HookPoints: []string{"before_tool_call"},
			Source:     `function other() {}`,
		},
	})
	if err == nil {
		t.Fatal("validateJavaScriptDraft() error = nil, want missing handle error")
	}
}

func TestEnrichJavaScriptDraftRequestExcludesCurrentID(t *testing.T) {
	req := &appaiassist.JavaScriptDraftRequest{
		Kind:        "tool",
		CurrentTool: &config.JavaScriptTool{ID: "current"},
	}
	enrichJavaScriptDraftRequest(context.Background(), req, &config.Config{JavaScript: config.JavaScriptSettings{Tools: []config.JavaScriptTool{
		{ID: "current"},
		{ID: "other"},
	}}}, nil)
	if len(req.ExistingIDs) != 1 || req.ExistingIDs[0] != "other" {
		t.Fatalf("existing ids = %#v", req.ExistingIDs)
	}
}

func TestEnrichJavaScriptDraftRequestAddsToolCatalog(t *testing.T) {
	registry := apptools.NewToolGroupRegistry()
	err := registry.Register(apptools.ToolGroupRegistration{
		Info: apptools.ToolGroupInfo{
			Name: "file", DisplayName: "文件", Description: "文件能力", Category: "文件", IncludedTools: []string{"file_read"},
		},
		Factory: func(apptools.ToolResolveContext) ([]runtimeport.Tool, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req := &appaiassist.JavaScriptDraftRequest{Kind: "tool"}
	enrichJavaScriptDraftRequest(context.Background(), req, &config.Config{}, registry)
	if len(req.AvailableToolGroups) != 1 || req.AvailableToolGroups[0].Name != "file" {
		t.Fatalf("available tool groups = %#v", req.AvailableToolGroups)
	}
}
