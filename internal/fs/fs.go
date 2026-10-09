package fs

import (
	"fmt"
	"io"
	"os"
)

type FS interface {
	Read(path string) ([]byte, error)
}

type Filesystem struct{}

// Read reads the contents of the file at path.
func (fs Filesystem) Read(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	defer func() { _ = file.Close() }()

	return data, nil
}
