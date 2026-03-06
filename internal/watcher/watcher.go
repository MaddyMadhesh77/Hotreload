// Package watcher wraps fsnotify to provide recursive, dynamic directory
// watching. It handles:
//
//   - Walking the entire root tree on startup and watching every non-ignored directory.
//   - Detecting newly created subdirectories and immediately watching them.
//   - Removing watches when directories are deleted.
//   - Forwarding filtered file-change events to a channel for the engine.
package watcher

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"hotreload/internal/filter"

	"github.com/fsnotify/fsnotify"
)

// Watcher wraps fsnotify and provides recursive watching with dynamic updates.
type Watcher struct {
	fw     *fsnotify.Watcher
	filter *filter.Filter
	root   string
	Events chan string // emits the path of changed files
	Errors chan error
}

// New creates and returns a Watcher rooted at root.
// It immediately adds all non-ignored directories under root to the watch list.
func New(root string, f *filter.Filter) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify.NewWatcher: %w", err)
	}

	w := &Watcher{
		fw:     fw,
		filter: f,
		root:   root,
		Events: make(chan string, 64),
		Errors: make(chan error, 8),
	}

	if err := w.walkAndWatch(root); err != nil {
		fw.Close()
		return nil, err
	}

	return w, nil
}

// walkAndWatch recursively walks dir and adds every non-ignored subdirectory
// to the fsnotify watcher.
func (w *Watcher) walkAndWatch(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Tolerate permission errors on individual paths.
			slog.Warn("watcher: walk error", "path", path, "err", err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if w.filter.ShouldIgnoreDir(d.Name()) {
			return filepath.SkipDir
		}
		if addErr := w.fw.Add(path); addErr != nil {
			slog.Warn("watcher: could not add directory", "path", path, "err", addErr)
		} else {
			slog.Debug("watcher: watching", "dir", path)
		}
		return nil
	})
}

// Start begins the event-processing loop. It should be called in a goroutine.
// It runs until Close is called.
func (w *Watcher) Start() {
	for {
		select {
		case event, ok := <-w.fw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			slog.Error("watcher: fsnotify error", "err", err)
			select {
			case w.Errors <- err:
			default:
			}
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// Skip paths the filter says to ignore.
	if w.filter.ShouldIgnorePath(path) {
		return
	}

	op := event.Op

	// --- Directory created: start watching it recursively ---
	if op&fsnotify.Create != 0 {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			slog.Info("watcher: new directory detected, adding watch", "dir", path)
			if walkErr := w.walkAndWatch(path); walkErr != nil {
				slog.Warn("watcher: could not watch new dir", "dir", path, "err", walkErr)
			}
			// A new directory is usually a structural event; don't forward it
			// as a file-change event — no rebuild needed just for mkdir.
			return
		}
	}

	// --- Directory removed: fsnotify removes it automatically ---
	if op&fsnotify.Remove != 0 || op&fsnotify.Rename != 0 {
		// Attempt to remove — fsnotify will no-op gracefully if not watched.
		_ = w.fw.Remove(path)
	}

	// Ignore chmod-only events (editors sometimes just touch metadata).
	if op == fsnotify.Chmod {
		return
	}

	// Forward the event path to the engine.
	slog.Debug("watcher: file event", "op", op, "path", path)
	select {
	case w.Events <- path:
	default:
		// Channel full: the engine is busy; the debouncer will still catch it
		// on the next delivery since we buffer generously.
	}
}

// Close shuts down the watcher and releases OS file handles.
func (w *Watcher) Close() error {
	return w.fw.Close()
}
