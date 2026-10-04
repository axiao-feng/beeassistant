package agentkit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/testmodel"
)

func TestDeepAgentPlansAndWritesInsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	model := testmodel.New().
		EnqueueStream(toolMessage("plan", "write_todos", `{"todos":[{"content":"write report","status":"in_progress"}]}`)).
		EnqueueStream(toolMessage("write", "write_file", `{"file_path":"report.txt","content":"report"}`)).
		EnqueueStream(testmodel.AssistantMessage("done"))
	a, err := NewDeepAgent(context.Background(), &runtimeport.DeepAgentConfig{
		Name: "deep", Model: model, Planning: runtimeport.DeepPlanningConfig{Enabled: true},
		Workspace: runtimeport.DeepWorkspaceConfig{Enabled: true, Dir: dir},
	})
	if err != nil {
		t.Fatal(err)
	}
	runAgentForTest(t, context.Background(), a, true)
	data, err := os.ReadFile(filepath.Join(dir, "report.txt"))
	if err != nil || string(data) != "report" {
		t.Fatalf("workspace output = %q, %v", data, err)
	}
}

func TestDeepAgentRejectsWorkspaceEscapes(t *testing.T) {
	for _, mode := range []string{"parent", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			workspace := filepath.Join(root, "workspace")
			if err := os.Mkdir(workspace, 0755); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(root, "outside.txt")
			if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			path := "../outside.txt"
			if mode == "symlink" {
				path = "link.txt"
				if err := os.Symlink(outside, filepath.Join(workspace, path)); err != nil {
					t.Skipf("symlinks are unavailable in this Windows environment: %v", err)
				}
			}
			args, _ := json.Marshal(map[string]string{"file_path": path, "content": "changed"})
			model := testmodel.New().EnqueueStream(toolMessage("escape", "write_file", string(args)))
			a, err := NewDeepAgent(context.Background(), &runtimeport.DeepAgentConfig{Name: "deep", Model: model, Workspace: runtimeport.DeepWorkspaceConfig{Enabled: true, Dir: workspace}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runAgentForTestResult(t, context.Background(), a, true)
			if err == nil {
				t.Fatal("workspace escape was allowed")
			}
			data, err := os.ReadFile(outside)
			if err != nil || string(data) != "unchanged" {
				t.Fatalf("outside file changed: %q, %v", data, err)
			}
		})
	}
}

func TestDeepAgentRejectsDangerousCommand(t *testing.T) {
	model := testmodel.New().EnqueueStream(toolMessage("command", "execute", `{"command":"rm -rf /"}`)).EnqueueStream(testmodel.AssistantMessage("rejected"))
	a, err := NewDeepAgent(context.Background(), &runtimeport.DeepAgentConfig{
		Name: "deep", Model: model, Workspace: runtimeport.DeepWorkspaceConfig{Dir: t.TempDir()}, Shell: runtimeport.DeepShellConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	runAgentForTest(t, context.Background(), a, true)
	calls := model.StreamCalls()
	if len(calls) != 2 || !messagesContain(calls[1].Input, "命令被拒绝") {
		t.Fatalf("command rejection missing: %#v", calls)
	}
}

func TestDeepAgentRequiresExplicitWorkspace(t *testing.T) {
	_, err := NewDeepAgent(context.Background(), &runtimeport.DeepAgentConfig{Name: "deep", Model: testmodel.New(), Workspace: runtimeport.DeepWorkspaceConfig{Enabled: true}})
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("error = %v", err)
	}
}
