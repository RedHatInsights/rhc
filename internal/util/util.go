package util

import (
	"io"
	"os"
	"strings"
)

// MustReadFile returns whitespace-trimmed content of a file.
// Returns an empty string in case an error of any kind occurs.
func MustReadFile(file *os.File) string {
	raw, err := io.ReadAll(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
