package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if strings.Contains(err.Error(), "read "+path) {
		t.Errorf("Read() error = %q, want the os.Open error", err)
	}
}

func TestReadDirectory(t *testing.T) {
	dir := t.TempDir()

	_, err := Filesystem{}.Read(dir)
	if err == nil {
		t.Fatal("Read() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "read "+dir) {
		t.Errorf("Read() error = %q, want read wrapper", err)
	}
}
