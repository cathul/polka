// Package converter runs the separately packaged FB2 converter. It never
// modifies the library file and does not download executables at runtime.
package converter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var ErrFailed = errors.New("FB2 conversion failed")

func EPUB3(ctx context.Context, binary, source string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output := filepath.Join(filepath.Dir(source), "converted.epub")
	cmd := exec.CommandContext(ctx, binary, "convert", "--to", "epub3", "--output-file", output, source)
	cmd.Dir = filepath.Dir(source)
	// Do not expose paths, book contents, or converter diagnostics to clients/logs.
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrFailed
	}
	info, err := os.Stat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return "", ErrFailed
	}
	return output, nil
}
