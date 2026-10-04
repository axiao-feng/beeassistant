//go:build windows

package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartBackgroundProcessHandlesSpecialWorkingDirectory(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "special & $ path")
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatalf("create working directory: %v", err)
	}

	result, err := startBackgroundProcess("echo beeassistant", workDir)
	if err != nil {
		t.Fatalf("start background process: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(result.StdoutFile)
		if readErr == nil && strings.TrimSpace(string(data)) == "beeassistant" {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	data, _ := os.ReadFile(result.StdoutFile)
	t.Fatalf("unexpected background output: %q", data)
}
