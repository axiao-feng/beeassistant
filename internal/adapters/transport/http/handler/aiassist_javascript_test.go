package handler

import (
	"testing"

	appaiassist "fkteams/internal/app/aiassist"
	"fkteams/internal/app/config"
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
	enrichJavaScriptDraftRequest(req, &config.Config{JavaScript: config.JavaScriptSettings{Tools: []config.JavaScriptTool{
		{ID: "current"},
		{ID: "other"},
	}}})
	if len(req.ExistingIDs) != 1 || req.ExistingIDs[0] != "other" {
		t.Fatalf("existing ids = %#v", req.ExistingIDs)
	}
}
