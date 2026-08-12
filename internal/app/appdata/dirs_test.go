package appdata

import (
	"path/filepath"
	"strings"
	"testing"

	"fkteams/internal/runtime/env"
)

func TestDirectoryHelpersUseAppDir(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv(env.AppDir, appDir)

	if Dir() != appDir {
		t.Fatalf("Dir = %q, want %q", Dir(), appDir)
	}
	for _, got := range []string{SessionsDir(), WorkspaceDir(), SchedulerDir(), ShareDir(), RuntimeDir(), JavaScriptDir(), SkillsDir()} {
		if !strings.HasPrefix(got, appDir+string(filepath.Separator)) {
			t.Fatalf("derived dir %q should be under app dir %q", got, appDir)
		}
	}
	if got := ConfigFile(); got != filepath.Join(appDir, "config", "config.toml") {
		t.Fatalf("ConfigFile = %q", got)
	}
}
