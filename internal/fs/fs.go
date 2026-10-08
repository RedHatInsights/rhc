package fs

import (
	"os"
)

type FS interface {
	Read(path string) ([]byte, error)
}

type Filesystem struct{}

// Read reads the contents of the file at path.
func (fs Filesystem) Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}
