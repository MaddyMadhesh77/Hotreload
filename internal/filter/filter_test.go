package filter_test

import (
	"testing"

	"hotreload/internal/filter"
)

func TestShouldIgnorePath_IgnoredDirectories(t *testing.T) {
	f := &filter.Filter{}

	cases := []struct {
		path   string
		ignore bool
	}{
		// Ignored directory segments anywhere in path
		{"/home/user/project/.git/config", true},
		{"/home/user/project/node_modules/express/index.js", true},
		{"/home/user/project/vendor/pkg/lib.go", true},
		{"C:/projects/app/bin/server", true},
		{"/app/dist/bundle.js", true},
		{"/app/build/output", true},
		{".cache/go/build", true},

		// Valid source files — must NOT be ignored
		{"/home/user/project/main.go", false},
		{"/home/user/project/internal/handler/handler.go", false},
		{"/home/user/project/pkg/util/strings.go", false},
		{"cmd/server/main.go", false},
		{"/app/testserver/main.go", false},
	}

	for _, tc := range cases {
		got := f.ShouldIgnorePath(tc.path)
		if got != tc.ignore {
			t.Errorf("ShouldIgnorePath(%q) = %v, want %v", tc.path, got, tc.ignore)
		}
	}
}

func TestShouldIgnorePath_IgnoredExtensions(t *testing.T) {
	f := &filter.Filter{}

	cases := []struct {
		path   string
		ignore bool
	}{
		{"/project/main.go.swp", true}, // vim swap
		{"/project/notes.tmp", true},   // temp
		{"/project/file.bak", true},    // backup
		{"#autosave#", true},           // emacs autosave
		{"/project/file.go~", true},    // tilde backup
		{"/project/main.go", false},    // normal Go file
		{"/project/handler_test.go", false},
		{"/project/README.md", false},
		{"/project/config.yaml", false},
	}

	for _, tc := range cases {
		got := f.ShouldIgnorePath(tc.path)
		if got != tc.ignore {
			t.Errorf("ShouldIgnorePath(%q) = %v, want %v", tc.path, got, tc.ignore)
		}
	}
}

func TestShouldIgnoreDir(t *testing.T) {
	f := &filter.Filter{}

	cases := []struct {
		name   string
		ignore bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{"bin", true},
		{"dist", true},
		{"build", true},
		{"internal", false},
		{"cmd", false},
		{"pkg", false},
		{"testserver", false},
	}

	for _, tc := range cases {
		got := f.ShouldIgnoreDir(tc.name)
		if got != tc.ignore {
			t.Errorf("ShouldIgnoreDir(%q) = %v, want %v", tc.name, got, tc.ignore)
		}
	}
}

func TestExtraDirs(t *testing.T) {
	f := &filter.Filter{ExtraDirs: []string{"myartifacts", "tmp"}}

	if !f.ShouldIgnoreDir("myartifacts") {
		t.Error("expected myartifacts to be ignored (extra dir)")
	}
	if !f.ShouldIgnorePath("/project/tmp/debug.log") {
		t.Error("expected tmp/debug.log to be ignored (extra dir)")
	}
	if f.ShouldIgnoreDir("src") {
		t.Error("src should not be ignored")
	}
}
