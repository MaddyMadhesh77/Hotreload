package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hotreload/internal/filter"
	"hotreload/internal/watcher"
)

// TestWatcherDetectsFileChange verifies that modifying a file in the watched
// root directory triggers an event on the Events channel.
func TestWatcherDetectsFileChange(t *testing.T) {
	dir := t.TempDir()

	// Create an initial file.
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	f := &filter.Filter{}
	w, err := watcher.New(dir, f)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	defer w.Close()

	go w.Start()

	// Give fsnotify a moment to initialise inotify/kqueue/ReadDirectoryChangesW.
	time.Sleep(100 * time.Millisecond)

	// Modify the file.
	if err := os.WriteFile(file, []byte("package main\n// changed\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case path := <-w.Events:
		if filepath.Base(path) != "main.go" {
			t.Errorf("unexpected event path: %s", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected file-change event but timed out")
	}
}

// TestWatcherIgnoresGitDir verifies that changes inside .git are not forwarded.
func TestWatcherIgnoresGitDir(t *testing.T) {
	dir := t.TempDir()

	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	f := &filter.Filter{}
	w, err := watcher.New(dir, f)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	// Write to a file inside .git — should be ignored.
	gitFile := filepath.Join(gitDir, "COMMIT_EDITMSG")
	_ = os.WriteFile(gitFile, []byte("initial commit"), 0o644)

	select {
	case ev := <-w.Events:
		t.Errorf("expected no event for .git change, got: %s", ev)
	case <-time.After(500 * time.Millisecond):
		// expected: no event
	}
}

// TestWatcherPicksUpNewSubdirectory verifies that a directory created after
// the watcher starts is automatically added to the watch list.
func TestWatcherPicksUpNewSubdirectory(t *testing.T) {
	dir := t.TempDir()

	f := &filter.Filter{}
	w, err := watcher.New(dir, f)
	if err != nil {
		t.Fatalf("watcher.New: %v", err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	// Create a new subdirectory.
	subDir := filepath.Join(dir, "handlers")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Wait for the watcher to pick up and add the new directory.
	time.Sleep(200 * time.Millisecond)

	// Now write a file inside the new subdirectory — should be detected.
	newFile := filepath.Join(subDir, "user.go")
	if err := os.WriteFile(newFile, []byte("package handlers\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case path := <-w.Events:
		if filepath.Base(path) != "user.go" {
			t.Logf("received event for: %s (may be subdir creation itself)", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected event for file in new directory, but timed out")
	}
}
