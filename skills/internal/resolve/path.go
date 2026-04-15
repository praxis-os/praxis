// SPDX-License-Identifier: Apache-2.0

// Package resolve validates and resolves bundle paths for skill loading.
package resolve

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkillFileName is the required file name for skill bundles.
const SkillFileName = "SKILL.md"

// ValidateFSRoot checks that root does not escape the fs.FS boundary.
// Returns an error if root contains ".." or is an absolute path.
func ValidateFSRoot(root string) error {
	if filepath.IsAbs(root) {
		return &EscapeError{Path: root, Reason: "absolute path not allowed in fs.FS root"}
	}
	for _, part := range strings.Split(filepath.ToSlash(root), "/") {
		if part == ".." {
			return &EscapeError{Path: root, Reason: "\"..\" component not allowed"}
		}
	}
	return nil
}

// ResolvePath resolves a host filesystem path to (dirPath, filePath).
// If path points to a file named SKILL.md, returns (dir, path).
// If path points to a directory, returns (path, path/SKILL.md).
// Returns an error if the path does not exist.
func ResolvePath(path string) (dir string, file string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}

	if info.IsDir() {
		return abs, filepath.Join(abs, SkillFileName), nil
	}
	return filepath.Dir(abs), abs, nil
}

// OpenFile opens SKILL.md within the given fs.FS at the given root.
func OpenFile(fsys fs.FS, root string) ([]byte, error) {
	path := root
	if path == "." || path == "" {
		path = SkillFileName
	} else {
		path = path + "/" + SkillFileName
	}
	return fs.ReadFile(fsys, path)
}

// EscapeError indicates a path traversal attempt.
type EscapeError struct {
	Path   string
	Reason string
}

func (e *EscapeError) Error() string {
	return "path escape: " + e.Path + ": " + e.Reason
}
