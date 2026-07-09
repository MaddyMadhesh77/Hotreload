package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCLIConfigGoShorthand(t *testing.T) {
	root, build, exec, cleanup, err := resolveCLIConfig("", "./testserver", "", "", nil)
	if err != nil {
		t.Fatalf("resolveCLIConfig returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function for shorthand mode")
	}
	defer cleanup()

	if root != "." {
		t.Fatalf("expected default root '.', got %q", root)
	}
	if !strings.HasPrefix(build, "go build -o ") {
		t.Fatalf("expected go build command, got %q", build)
	}
	if !strings.Contains(build, "./testserver") {
		t.Fatalf("expected build command to target ./testserver, got %q", build)
	}
	if !strings.Contains(exec, "hotreload-server") {
		t.Fatalf("expected temp binary name in exec command, got %q", exec)
	}
}

func TestResolveCLIConfigGoShorthandWithArgs(t *testing.T) {
	_, build, exec, cleanup, err := resolveCLIConfig("./app", "./cmd/server", "", "", []string{"--port", "8080"})
	if err != nil {
		t.Fatalf("resolveCLIConfig returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function for shorthand mode")
	}
	defer cleanup()

	if !strings.Contains(build, "go build -o ") {
		t.Fatalf("expected build command, got %q", build)
	}
	if !strings.Contains(exec, "--port") || !strings.Contains(exec, "8080") {
		t.Fatalf("expected runtime args in exec command, got %q", exec)
	}
}

func TestResolveCLIConfigPositionalShorthandWithArgs(t *testing.T) {
	_, _, exec, cleanup, err := resolveCLIConfig("", "", "", "", []string{"./cmd/server", "--", "--port", "8080"})
	if err != nil {
		t.Fatalf("resolveCLIConfig returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function for positional shorthand")
	}
	defer cleanup()

	if !strings.Contains(exec, "--port") || !strings.Contains(exec, "8080") {
		t.Fatalf("expected runtime args in exec command, got %q", exec)
	}
}

func TestResolveCLIConfigExplicitMode(t *testing.T) {
	root, build, exec, cleanup, err := resolveCLIConfig("./app", "", "go build -o ./bin/app ./cmd/app", "./bin/app", nil)
	if err != nil {
		t.Fatalf("resolveCLIConfig returned error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("did not expect cleanup function for explicit mode")
	}
	if root != "./app" {
		t.Fatalf("unexpected root: %q", root)
	}
	if build != "go build -o ./bin/app ./cmd/app" {
		t.Fatalf("unexpected build command: %q", build)
	}
	if exec != "./bin/app" {
		t.Fatalf("unexpected exec command: %q", exec)
	}
}

func TestResolveCLIConfigRejectsMixedModes(t *testing.T) {
	_, _, _, _, err := resolveCLIConfig(".", "./testserver", "go build", "./bin/server", nil)
	if err == nil {
		t.Fatal("expected error when mixing shorthand and explicit flags")
	}
}

func TestResolveCLIConfigAutoDetectsSingleMainPackage(t *testing.T) {
	root := writeTempGoModule(t, "example.com/app", map[string]string{
		"cmd/server/main.go": "package main\n\nfunc main() {}\n",
		"internal/lib/lib.go": "package lib\n",
	})

	resolvedRoot, build, exec, cleanup, err := resolveCLIConfig(root, "", "", "", nil)
	if err != nil {
		t.Fatalf("resolveCLIConfig returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function for inferred mode")
	}
	defer cleanup()

	if resolvedRoot != root {
		t.Fatalf("unexpected root: %q", resolvedRoot)
	}
	if !strings.Contains(build, "example.com/app/cmd/server") {
		t.Fatalf("expected inferred build target, got %q", build)
	}
	if !strings.Contains(exec, "hotreload-server") {
		t.Fatalf("expected temp binary in exec command, got %q", exec)
	}
}

func TestResolveCLIConfigRejectsAmbiguousMainPackages(t *testing.T) {
	root := writeTempGoModule(t, "example.com/app", map[string]string{
		"cmd/server/main.go": "package main\n\nfunc main() {}\n",
		"cmd/worker/main.go": "package main\n\nfunc main() {}\n",
	})

	_, _, _, _, err := resolveCLIConfig(root, "", "", "", nil)
	if err == nil {
		t.Fatal("expected error for multiple main packages")
	}
}

func writeTempGoModule(t *testing.T, modulePath string, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.26.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	for relativePath, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relativePath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relativePath, err)
		}
	}

	return root
}
