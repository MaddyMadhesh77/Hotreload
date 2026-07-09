# hotreload

A Go-first hot-reload CLI that rebuilds and restarts your server when source files change. It is meant to feel like the Go equivalent of nodemon: one command, save a file, and your app comes back automatically.

## Quick Start

If your Go project has a single `main` package, you can usually just run:

```bash
hotreload
```

If your app needs runtime flags, pass them after `--`:

```bash
hotreload -- --port 8080
```

If your repository has multiple binaries, point at the one you want:

```bash
hotreload ./cmd/server
```

Common layouts:

```bash
# API server
hotreload ./cmd/api

# Web server
hotreload ./cmd/server

# Run with app flags
hotreload ./cmd/server -- --port 8080
```

If you want full control, keep using the explicit mode:

```bash
hotreload \
  --root . \
  --build "go build -o ./bin/server ./cmd/server" \
  --exec "./bin/server"
```

---

## Features

| Feature | Status |
|---------|--------|
| Initial build on startup (no wait) | ✅ |
| Recursive directory watching | ✅ |
| Dynamic new-subdir detection | ✅ |
| 200 ms debounce (handles editor double-saves) | ✅ |
| In-flight build cancellation | ✅ |
| Graceful stop → SIGKILL fallback | ✅ |
| Process-group kill (children included) | ✅ |
| Crash-loop detection + cooldown | ✅ |
| Real-time log streaming | ✅ |
| File filter (.git, node_modules, *.swp …) | ✅ |
| Windows + Linux + macOS | ✅ |

---

## Installation

```bash
go install github.com/yourname/hotreload/cmd/hotreload@latest
```

Or build from source:

```bash
git clone https://github.com/yourname/hotreload
cd hotreload
make build          # produces ./bin/hotreload
```

---

## Usage

```bash
hotreload
hotreload [-- app-args...]
hotreload <go-package-or-dir> [-- app-args...]
hotreload --run <go-package-or-dir> [-- app-args...]
hotreload --root <dir> --build "<cmd>" --exec "<cmd>"
```

Flags:

```text
  --root      <dir>       Directory to watch (default: .)
  --run       <pkg>       Go package or directory to build and run automatically
  --build     "<cmd>"     Build command (required in explicit mode)
  --exec      "<cmd>"     Run command (required in explicit mode)
  --debounce  <duration>  Quiet period before rebuild (default: 200ms)
  -v                      Verbose / debug logging
```

When no target is provided, hotreload scans the Go workspace for a single runnable `main` package and uses that automatically. If it finds more than one, it asks you to pass the target explicitly.

---

## Why Use It

- One command for typical Go projects: `hotreload ./cmd/server`
- No separate build output directory to manage in the common case
- Works with runtime arguments, so local dev feels natural
- Keeps the explicit build/exec mode for advanced projects and custom pipelines

## Quick Demo

```bash
# Terminal 1 — start the hot-reload demo
make demo

# Terminal 2 — verify the server is running
curl http://localhost:8080

# Now edit testserver/main.go (change VERSION), save.
# Within ~1 second you'll see the server restart with the new version.
curl http://localhost:8080
```

---

## Running Tests

```bash
make test
# or
go test -v -race ./...
```

Tests cover:

- `internal/debouncer` — burst collapse, timer reset, Stop cancellation
- `internal/filter` — ignored dirs, extensions, editor temp files, extra dirs
- `internal/watcher` — file change detection, .git ignore, dynamic new-dir pickup

---

## Architecture

```
hotreload
├── cmd/hotreload/main.go          CLI: flag parsing, slog setup
└── internal/
    ├── engine/engine.go           Orchestrates the rebuild loop
    ├── watcher/watcher.go         Recursive fsnotify watcher
    ├── debouncer/debouncer.go     Collapses burst events → single trigger
    ├── builder/builder.go         Runs build cmd (cancellable)
    ├── runner/runner.go           Manages server process
    └── filter/filter.go           Path ignore rules
```

**Rebuild lifecycle:**

```
File saved
  → watcher detects event
  → filter drops noise (*.swp, .git/…)
  → debouncer waits 200 ms quiet period
  → engine cancels any in-flight build
  → engine stops old server
  → builder runs build command
  → on success: runner starts new server
```

---

## Ground Rules (per assignment)

- ❌ No `air`, `realize`, or `reflex`
- ✅ `fsnotify` used as event source only; all other logic hand-rolled
- ✅ `log/slog` for all logging
- ✅ Commit history shows incremental evolution

---

## Project Structure

```
hotreload/
├── cmd/hotreload/          Main binary
├── internal/
│   ├── builder/            Build command runner + platform process setup
│   ├── debouncer/          Event debounce logic
│   ├── engine/             Core orchestrator
│   ├── filter/             Path filtering
│   ├── runner/             Server process manager + platform kill logic
│   └── watcher/            Recursive fsnotify watcher
├── testserver/             Demo HTTP server
├── Makefile
└── README.md
```
