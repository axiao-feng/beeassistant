package fkteamshelper

import (
	"slices"
	"strings"
	"testing"

	"fkteams/internal/app/agent/catalog/common"
)

func TestDefaultDefinitionProvidesFocusedProjectHelp(t *testing.T) {
	def := DefaultDefinition()
	if def.Name != "fkteams_helper" {
		t.Fatalf("name = %q, want fkteams_helper", def.Name)
	}
	if def.Profile != common.ProfileBare {
		t.Fatalf("profile = %q, want bare", def.Profile)
	}
	if !slices.Equal(def.ToolNames, []string{"search", "fetch"}) {
		t.Fatalf("tool names = %#v", def.ToolNames)
	}
	for _, want := range []string{"Skills", "MCP", "自定义智能体", "自定义工具", "流程 Hooks", "官方资料"} {
		if !strings.Contains(def.Instruction, want) {
			t.Fatalf("instruction does not contain %q", want)
		}
	}
}
