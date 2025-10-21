package k6env

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
)

// ErrNotFound is returned when the k6 executable cannot be located on PATH.
var ErrNotFound = errors.New("k6 executable not found on PATH")

// Info captures metadata about a discovered k6 installation.
type Info struct {
	// Path is the absolute path to the discovered k6 executable.
	Path string
}

// Locate searches for the k6 executable on PATH and returns its location.
// The provided context is reserved for future expansion (e.g., version lookups)
// and may be nil.
func Locate(_ context.Context) (Info, error) {
	path, err := exec.LookPath("k6")
	if err != nil {
		return Info{}, fmt.Errorf("%w: %s", ErrNotFound, normalizeExecError(err))
	}

	return Info{Path: path}, nil
}

// normalizeExecError converts platform-specific errors from exec.LookPath into a
// compact string for display to end-users.
func normalizeExecError(err error) string {
	var execErr *exec.Error
	if errors.As(err, &execErr) && execErr.Err != nil {
		return execErr.Err.Error()
	}

	var pathErr *fs.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err.Error()
	}

	return strings.TrimSpace(err.Error())
}
