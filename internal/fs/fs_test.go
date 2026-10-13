package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rhc.conf")
	const contents = "[compatibility]\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	data, err := Filesystem{}.Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(data) != contents {
		t.Errorf("Read() = %q, want %q", data, contents)
	}
}

func TestReadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.conf")

	_, err := Filesystem{}.Read(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read() error = %v, want os.ErrNotExist", err)
	}
}

func TestReadDirectory(t *testing.T) {
	_, err := Filesystem{}.Read(t.TempDir())
	if err == nil {
		t.Fatal("Read() error = nil, want read error")
	}
}
