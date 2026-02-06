// Package helpers provides utility functions.
package helpers

import "strings"

// GetPathType returns a safe representation of file paths for logging.
func GetPathType(path string) string {
	switch {
	case strings.Contains(path, "temp"), strings.Contains(path, "tmp"):
		return "temporary"
	case strings.HasSuffix(path, ".js"):
		return "javascript"
	case strings.HasSuffix(path, ".ts"):
		return "typescript"
	}
	return "other"
}
