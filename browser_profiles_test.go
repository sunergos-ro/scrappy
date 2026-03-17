package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCleanupStaleUserDataDirsRemovesOnlyInactiveExpiredDirs(t *testing.T) {
	root := t.TempDir()

	activeDir := filepath.Join(root, "active")
	recentDir := filepath.Join(root, "recent")
	staleDir := filepath.Join(root, "stale")
	plainFile := filepath.Join(root, "note.txt")

	for _, dir := range []string{activeDir, recentDir, staleDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(plainFile, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(activeDir, old, old); err != nil {
		t.Fatalf("chtimes active: %v", err)
	}
	if err := os.Chtimes(staleDir, old, old); err != nil {
		t.Fatalf("chtimes stale: %v", err)
	}

	removed, err := cleanupStaleUserDataDirs(root, map[string]struct{}{
		filepath.Clean(activeDir): {},
	}, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("cleanupStaleUserDataDirs: %v", err)
	}

	if !slices.Contains(removed, filepath.Clean(staleDir)) {
		t.Fatalf("expected stale dir to be removed, removed=%v", removed)
	}
	if slices.Contains(removed, filepath.Clean(activeDir)) {
		t.Fatalf("active dir should not be removed, removed=%v", removed)
	}
	if slices.Contains(removed, filepath.Clean(recentDir)) {
		t.Fatalf("recent dir should not be removed, removed=%v", removed)
	}

	if _, err := os.Stat(activeDir); err != nil {
		t.Fatalf("active dir should remain: %v", err)
	}
	if _, err := os.Stat(recentDir); err != nil {
		t.Fatalf("recent dir should remain: %v", err)
	}
	if _, err := os.Stat(plainFile); err != nil {
		t.Fatalf("plain file should remain: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatalf("stale dir should be removed, stat err=%v", err)
	}
}
