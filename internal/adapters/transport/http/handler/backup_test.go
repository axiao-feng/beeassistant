package handler

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupArchiveRoundTrip(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "profile"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config", "config.toml"), []byte("[memory]\nenabled = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "profile", "personal.json"), []byte(`{"name":"小明"}`), 0644); err != nil {
		t.Fatal(err)
	}

	archiveFile, err := os.CreateTemp(t.TempDir(), "backup-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	archivePath := archiveFile.Name()
	if err := writeBackupArchive(archiveFile, source); err != nil {
		_ = archiveFile.Close()
		t.Fatalf("write backup: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	restored, err := restoreBackupArchive(archivePath, destination)
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	if restored != 2 {
		t.Fatalf("restored = %d, want 2", restored)
	}
	data, err := os.ReadFile(filepath.Join(destination, "profile", "personal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"小明"}` {
		t.Fatalf("profile = %s", data)
	}
}

func TestRestoreBackupRejectsTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("unsafe"))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := restoreBackupArchive(archivePath, t.TempDir()); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}
