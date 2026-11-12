package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceVersion is a cheap cache invalidation token, not a content identity.
// Actual device identity is checked against the bytes read by the reader.
func (l *Library) SourceVersion(folder, file, ext string) (string, error) {
	name := file + "." + ext
	if !pathInside(l.root, folder) || strings.ContainsAny(name, `/\`) {
		return "", ErrNoFile
	}
	path := filepath.Join(l.root, filepath.FromSlash(folder))
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		info, err = os.Stat(filepath.Join(path, name))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%d:%d", filepath.Join(path, name), info.Size(), info.ModTime().UnixNano()), nil
	}
	var parts []string
	for _, candidate := range archiveCandidates(path) {
		if info, err := os.Stat(candidate); err == nil {
			parts = append(parts, fmt.Sprintf("%s:%d:%d", candidate, info.Size(), info.ModTime().UnixNano()))
		}
	}
	if len(parts) == 0 {
		return "", ErrNoFile
	}
	return strings.Join(parts, "|"), nil
}
