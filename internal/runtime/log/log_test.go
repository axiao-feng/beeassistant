package log

import (
	"fkteams/internal/runtime/env"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestSprintf(t *testing.T) {
	if got := Sprintf("hello %s", "world"); got != "hello world" {
		t.Fatalf("Sprintf() = %q, want hello world", got)
	}
}

func TestReadLevelDefaultsToDebug(t *testing.T) {
	if got := readLevel(); got != zapcore.DebugLevel {
		t.Fatalf("readLevel() = %v, want debug", got)
	}
}

func TestAppDirUsesEnvThenHomeFallback(t *testing.T) {
	t.Setenv(env.AppDir, "/tmp/fkteams-log")
	if got := appDir(); got != "/tmp/fkteams-log" {
		t.Fatalf("appDir() = %q, want env value", got)
	}

	t.Setenv(env.AppDir, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got := appDir(); got != filepath.Join(home, ".fkteams") {
		t.Fatalf("appDir() = %q, want home fallback", got)
	}
}

func TestLoggerWritesToAppLogFile(t *testing.T) {
	resetLogger(t)
	appDir := t.TempDir()
	t.Setenv(env.AppDir, appDir)
	defer func() { _ = Close() }()

	Info("hello log")
	Printf("formatted %s", "message")
	Println("line log")
	Print("plain log")
	Debug("debug log")
	Warn("warn log")
	Error("error log")
	Debugf("debug %s", "format")
	Infof("info %s", "format")
	Warnf("warn %s", "format")
	Errorf("error %s", "format")
	if sugar != nil {
		_ = sugar.Sync()
	}

	data, err := os.ReadFile(filepath.Join(appDir, "log", "fkteams.log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	for _, want := range []string{"hello log", "formatted message", "line log", "plain log", "debug log", "warn log", "error log"} {
		if !strings.Contains(content, want) {
			t.Fatalf("log content = %q, missing %q", content, want)
		}
	}
}

func TestLoggerIsSingleton(t *testing.T) {
	resetLogger(t)
	t.Setenv(env.AppDir, t.TempDir())

	first := logger()
	second := logger()
	if first == nil || second == nil {
		t.Fatal("logger returned nil")
	}
	if first != second {
		t.Fatal("logger should return singleton instance")
	}
}

func resetLogger(t *testing.T) {
	t.Helper()

	_ = Close()
	t.Cleanup(func() {
		_ = Close()
	})
}
