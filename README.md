# hotreload

A lightweight CLI tool that watches a Go project for source-code changes and automatically rebuilds and restarts the server. No frameworks — just `fsnotify` and idiomatic Go.

```
hotreload --root ./myproject --build "go build -o ./bin/server ./cmd/server" --exec "./bin/server"
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

```
hotreload [flags]

Flags:
  --root      <dir>       Directory to watch (default: .)
  --build     "<cmd>"     Build command (required)
  --exec      "<cmd>"     Run command (required)
  --debounce  <duration>  Quiet period before rebuild (default: 200ms)
  -v                      Verbose / debug logging
```

### Example

```bash
hotreload \
  --root ./myproject \
  --build "go build -o ./bin/server ./cmd/server" \
  --exec "./bin/server"
```

---

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
