package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redhatinsights/rhc/internal/collector"
)

func TestCreateTmpDir(t *testing.T) {
	t.Run("successful temp directory creation", func(t *testing.T) {
		// Ensure the parent directory exists for testing
		if err := os.MkdirAll(rhcTmpDir, 0755); err != nil {
			t.Skipf("Cannot create parent directory %s: %v", rhcTmpDir, err)
		}
		defer func() {
			if os.RemoveAll(rhcTmpDir) != nil {
				return
			}
		}() // Clean up the parent directory after test
		tmpDir, err := createTmpDir()
		if err != nil {
			t.Errorf("createTmpDir() unexpected error: %v", err)
			return
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Logf("Failed to clean up temp dir: %v", err)
			}
		}()
		if tmpDir == "" {
			t.Error("createTmpDir() returned empty string")
			return
		}
		if !strings.HasPrefix(tmpDir, rhcTmpDir) {
			t.Errorf("createTmpDir() = %q, want prefix '%q'", tmpDir, rhcTmpDir)
		}
		info, err := os.Stat(tmpDir)
		if err != nil {
			t.Errorf("created directory does not exist: %v", err)
			return
		}
		if !info.IsDir() {
			t.Error("created path is not a directory")
		}
	})
}

func TestGetConfig(t *testing.T) {
	// Note: Can't test happy path without mocking filesystem or root permissions.
	// This wrapper around collector.GetConfig() reads from /usr/lib/rhc/collectors/.

	t.Run("invalid collector ID", func(t *testing.T) {
		invalidID := "invalid..id"
		if _, err := getConfig(invalidID); err == nil {
			t.Error("getConfig() expected error for invalid collector ID")
		}
	})
}

func TestExecuteCollector(t *testing.T) {
	t.Run("successful command execution", func(t *testing.T) {
		tmpDir := t.TempDir()
		command := "echo 'test output'"
		err := executeCollector(command, tmpDir)
		if err != nil {
			t.Errorf("executeCollector() unexpected error: %v", err)
		}
	})

	t.Run("command that creates files", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := "test-output.txt"
		command := fmt.Sprintf("echo 'test content' > %s", testFile)
		err := executeCollector(command, tmpDir)
		if err != nil {
			t.Errorf("executeCollector() unexpected error: %v", err)
		}
		createdFile := filepath.Join(tmpDir, testFile)
		if _, err := os.Stat(createdFile); os.IsNotExist(err) {
			t.Errorf("executeCollector() did not create expected file: %s", createdFile)
		}
	})

	t.Run("failing command", func(t *testing.T) {
		tmpDir := t.TempDir()
		command := "exit 1"
		err := executeCollector(command, tmpDir)
		if err == nil {
			t.Error("executeCollector() expected error for failing command")
		}
		if !strings.Contains(err.Error(), "failed to execute collector") {
			t.Errorf("executeCollector() error = %v, want error containing 'failed to execute collector'", err)
		}
	})

	t.Run("nonexistent command", func(t *testing.T) {
		tmpDir := t.TempDir()
		command := "nonexistentcommand123456"
		err := executeCollector(command, tmpDir)
		if err == nil {
			t.Error("executeCollector() expected error for nonexistent command")
		}
	})

	t.Run("command with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		command := "echo 'test with spaces and \"quotes\"'"
		err := executeCollector(command, tmpDir)
		if err != nil {
			t.Errorf("executeCollector() unexpected error with special characters: %v", err)
		}
	})
}

func TestGetArchivePath(t *testing.T) {
	t.Run("directory with files", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		archivePath, err := getArchivePath(tmpDir)
		if err != nil {
			t.Errorf("getArchivePath() unexpected error: %v", err)
		}
		defer func(name string) {
			if os.Remove(name) != nil {
				return
			}
		}(archivePath)
		if archivePath == "" {
			t.Error("getArchivePath() returned empty string")
		}
		if !strings.HasSuffix(archivePath, ".tar.xz") {
			t.Errorf("getArchivePath() = %q, want file ending with '.tar.xz'", archivePath)
		}
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("getArchivePath() archive does not exist: %s", archivePath)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath, err := getArchivePath(tmpDir)
		if err != nil {
			t.Errorf("getArchivePath() unexpected error for empty directory: %v", err)
		}
		defer func(name string) {
			if os.Remove(name) != nil {
				return
			}
		}(archivePath)
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("getArchivePath() archive does not exist for empty directory: %s", archivePath)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		nonexistentDir := "/nonexistent/path"
		_, err := getArchivePath(nonexistentDir)
		if err == nil {
			t.Error("getArchivePath() expected error for nonexistent directory")
		}
	})
}

// getArchivePathFromTmpDir is a test helper that compresses a sample text file
// into a .tar.xz archive and returns the archive path.
// The archive is removed automatically when the test completes via t.Cleanup.
func getArchivePathFromTmpDir(t *testing.T) string {
	t.Helper()
	srcDir := t.TempDir()
	outDir := t.TempDir()
	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	archivePath, err := collector.GetArchive(srcDir, outDir)
	if err != nil {
		t.Fatalf("Failed to create test archive: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(archivePath)
	})
	return archivePath
}

func TestUploadArchive(t *testing.T) {
	testConfig := collector.Config{
		ID:          "test.collector",
		Name:        "Test Collector",
		ContentType: "application/vnd.redhat.advisor.collection",
	}

	t.Run("upload with valid parameters", func(t *testing.T) {
		archivePath := getArchivePathFromTmpDir(t)
		err := uploadArchive(archivePath, testConfig)
		t.Logf("UploadArchive() result: %v", err)
	})

	t.Run("upload with nonexistent archive", func(t *testing.T) {
		nonexistentArchive := "/nonexistent/archive.tar.xz"
		err := uploadArchive(nonexistentArchive, testConfig)
		if err == nil {
			t.Error("uploadArchive() expected error for nonexistent archive")
		}
	})

	t.Run("upload with empty content type", func(t *testing.T) {
		archivePath := getArchivePathFromTmpDir(t)
		emptyContentTypeConfig := collector.Config{
			ID:          "test.collector",
			Name:        "Test Collector",
			ContentType: "",
		}
		err := uploadArchive(archivePath, emptyContentTypeConfig)
		if err == nil {
			t.Error("uploadArchive() expected error for empty content type")
		}
	})
}

func TestCleanup(t *testing.T) {
	t.Run("removes file successfully", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-cleanup-")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		filePath := tmpFile.Name()
		if tmpFile.Close() != nil {
			return
		}

		cleanup(filePath)

		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("File still exists after cleanup: %s", filePath)
		}
	})

	t.Run("handles error gracefully", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("this test does not work when run as root")
		}
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, "test")
		if err := os.Mkdir(testDir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		if err := os.Chmod(tmpDir, 0444); err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}
		defer func(name string, mode os.FileMode) {
			err := os.Chmod(name, mode)
			if err != nil {
				t.Fatalf("Failed to make directory read-only: %v", err)
			}
		}(tmpDir, 0755)

		cleanup(testDir)
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Error("Directory was unexpectedly removed")
		}
	})
}
