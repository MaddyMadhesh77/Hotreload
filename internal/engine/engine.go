// Package engine ties together the watcher, debouncer, builder, and runner into
// a coherent hot-reload lifecycle:
//
//  1. Trigger an initial build immediately on startup.
//  2. On build success, start the server.
//  3. Watch for file changes; debounce rapid events.
//  4. On each debounced signal: cancel any in-flight build, stop the server,
//     run a fresh build, and restart the server if the build passes.
//
// The engine shuts down cleanly on SIGINT / SIGTERM.
package engine

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hotreload/internal/builder"
	"hotreload/internal/debouncer"
	"hotreload/internal/filter"
	"hotreload/internal/runner"
	"hotreload/internal/watcher"
)

// Config holds the user-supplied configuration for the hot-reload engine.
type Config struct {
	Root         string
	BuildCommand string
	ExecCommand  string
	// DebounceDelay is how long to wait after the last file event before
	// triggering a rebuild. Defaults to 200ms if zero.
	DebounceDelay time.Duration
}

// Run starts the hot-reload engine and blocks until the process is signalled
// to stop. It returns any fatal startup error.
func Run(cfg Config) error {
	if cfg.DebounceDelay == 0 {
		cfg.DebounceDelay = 200 * time.Millisecond
	}

	slog.Info("hotreload: starting",
		"root", cfg.Root,
		"build", cfg.BuildCommand,
		"exec", cfg.ExecCommand,
		"debounce", cfg.DebounceDelay,
	)

	// --- Components ---
	f := &filter.Filter{}
	b := builder.New(cfg.BuildCommand)
	r := runner.New(cfg.ExecCommand)
	db := debouncer.New(cfg.DebounceDelay)

	// Set up the file watcher.
	w, err := watcher.New(cfg.Root, f)
	if err != nil {
		return err
	}
	defer w.Close()

	// --- Signal handling ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			slog.Info("hotreload: received signal, shutting down", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	// --- Watcher goroutine ---
	go w.Start()

	// Forward watcher events into the debouncer.
	go func() {
		for {
			select {
			case path, ok := <-w.Events:
				if !ok {
					return
				}
				slog.Debug("hotreload: file changed", "path", path)
				db.Trigger()
			case <-ctx.Done():
				return
			}
		}
	}()

	// --- Build context management ---
	// buildCtx / buildCancel track the in-flight build so we can abort it
	// when a new event arrives.
	buildCtx, buildCancel := context.WithCancel(ctx)
	defer buildCancel()

	// Trigger the first build immediately without waiting for a file event.
	triggerCh := make(chan struct{}, 1)
	triggerCh <- struct{}{}

	// Bridge debouncer output into triggerCh.
	go func() {
		for {
			select {
			case <-db.C():
				select {
				case triggerCh <- struct{}{}:
				default:
					// Already a pending trigger; skip.
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// --- Main rebuild loop ---
	for {
		select {
		case <-ctx.Done():
			slog.Info("hotreload: stopping server and exiting")
			r.Stop()
			db.Stop()
			return nil

		case <-triggerCh:
			// Cancel any in-flight build so we don't have two competing builds.
			buildCancel()

			// Stop the running server before starting a new build.
			r.Stop()

			// Fresh build context.
			buildCtx, buildCancel = context.WithCancel(ctx)

			slog.Info("hotreload: change detected, rebuilding...")
			result := b.Run(buildCtx)

			if result.Err != nil && buildCtx.Err() != nil {
				// Build was cancelled because a newer change arrived; wait for
				// the next trigger.
				slog.Info("hotreload: build superseded by newer change")
				continue
			}

			if !result.Success {
				slog.Error("hotreload: build failed — fix errors and save to retry")
				continue
			}

			slog.Info("hotreload: build succeeded, starting server")
			r.Start()
		}
	}
}
