// Package filter provides path-based ignore rules for the hotreload watcher.
// It keeps the watcher focused on source code changes by excluding well-known
// noise sources: VCS directories, dependency caches, build artifacts, and
// ephemeral editor temp files.
package filter

import (
	"path/filepath"
	"strings"
)

// ignoredDirs is the set of directory names that should never be watched.
var ignoredDirs = map[string]bool{
	".git":         true,
	".svn":         true,
	".hg":          true,
	"node_modules": true,
	"vendor":       true,
	".cache":       true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
	"dist":         true,
	"build":        true,
	"bin":          true,
	"out":          true,
	".bin":         true,
}

// ignoredExtensions is the set of file extensions that should be ignored.
var ignoredExtensions = map[string]bool{
	".swp":  true,
	".swo":  true,
	".tmp":  true,
	".temp": true,
	".bak":  true,
	".orig": true,
	".pyc":  true,
	".o":    true,
	".a":    true,
	".test": true,
}

// Filter holds the configuration for path filtering.
// The zero value is ready to use with default rules.
type Filter struct {
	// ExtraDirs holds additional directory names to ignore beyond the defaults.
	ExtraDirs []string
}

// ShouldIgnorePath returns true if the given absolute path should be excluded
// from file-change processing. It checks every segment of the path against the
// ignore-dir set and checks the file extension against the ignore-extension set.
func (f *Filter) ShouldIgnorePath(path string) bool {
	// Normalize separators so the logic works on both Unix and Windows.
	path = filepath.ToSlash(path)

	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if ignoredDirs[seg] {
			return true
		}
		// honour extra dirs supplied by caller
		for _, extra := range f.ExtraDirs {
			if seg == extra {
				return true
			}
		}
		// Ignore dotfiles that start with a dot but aren't the root "./"
		if len(seg) > 1 && seg[0] == '.' {
			// Already caught .git etc.; catch any other hidden dirs/files
			// but be careful: we don't want to accidentally ignore "./" itself.
			if ignoredDirs[seg] {
				return true
			}
		}
	}

	// Check extension of the final element.
	base := segments[len(segments)-1]
	ext := strings.ToLower(filepath.Ext(base))
	if ignoredExtensions[ext] {
		return true
	}

	// Vim / Emacs backup patterns: files ending in ~ or starting with #
	if strings.HasSuffix(base, "~") || strings.HasPrefix(base, "#") {
		return true
	}

	return false
}

// ShouldIgnoreDir returns true if the given directory name (basename only)
// is in the ignore list. Used by the watcher to avoid adding dirs to fsnotify.
func (f *Filter) ShouldIgnoreDir(name string) bool {
	if ignoredDirs[name] {
		return true
	}
	for _, extra := range f.ExtraDirs {
		if name == extra {
			return true
		}
	}
	return false
}
