package main

import (
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
		t.Cleanup(func() { _ = os.RemoveAll(rhcTmpDir) })
		tmpDir, err := createTmpDir()
		if err != nil {
			t.Errorf("createTmpDir() unexpected error: %v", err)
			return
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Logf("Failed to clean up temp dir: %v", err)
			}
		})
		if tmpDir == "" {
			t.Error("createTmpDir() returned empty string")
			return
		}
		if !strings.HasPrefix(tmpDir, rhcTmpDir) {
			t.Errorf("createTmpDir() = %q, want prefix '%q'", tmpDir, rhcTmpDir)
		}
		parentInfo, err := os.Stat(rhcTmpDir)
		if err != nil {
			t.Errorf("parent directory does not exist: %v", err)
			return
		}
		if perms := parentInfo.Mode().Perm(); perms != 0700 {
			t.Errorf("parent directory permissions = %o, want %o", perms, os.FileMode(0700))
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

func TestCreateTmpDirCreatesMissingParent(t *testing.T) {
	if err := os.RemoveAll(rhcTmpDir); err != nil {
		t.Fatalf("failed to remove existing rhcTmpDir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(rhcTmpDir); err != nil {
			t.Logf("Failed to clean up rhcTmpDir: %v", err)
		}
	})

	tmpDir, err := createTmpDir()
	if err != nil {
		t.Fatalf("createTmpDir() unexpected error: %v", err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to clean up temp dir: %v", err)
		}
	}()

	info, err := os.Stat(rhcTmpDir)
	if err != nil {
		t.Fatalf("parent directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("rhcTmpDir is not a directory")
	}

	if perms := info.Mode().Perm(); perms != 0700 {
		t.Errorf("permissions = %o, want %o", perms, os.FileMode(0700))
	}
}

// TestGetConfig verifies that getConfig correctly validates the collector ID
// before attempting to read from the filesystem.
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

// TestExecuteCollector verifies that executeCollector runs the collector binary
// with the correct arguments.
func TestExecuteCollector(t *testing.T) {
	t.Run("nonexistent collector binary", func(t *testing.T) {
		tmpDir := t.TempDir()
		collectorId := "com.redhat.nonexistent"
		err := executeCollector(collectorId, tmpDir)
		if err == nil {
			t.Error("executeCollector() expected error for nonexistent collector binary")
		}
		if !strings.Contains(err.Error(), "failed to execute collector") {
			t.Errorf("executeCollector() error = %v, want error containing 'failed to execute collector'", err)
		}
	})

	t.Run("collector binary with invalid ID characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		collectorId := "../../../etc/passwd"
		err := executeCollector(collectorId, tmpDir)
		if err == nil {
			t.Error("executeCollector() expected error for path traversal attempt")
		}
	})
}

// TestGetArchive verifies that collector.GetArchive correctly creates a
// .tar.xz archive from a given source directory.
func TestGetArchive(t *testing.T) {
	t.Run("directory with files", func(t *testing.T) {
		srcDir := t.TempDir()
		outDir := t.TempDir()
		testFile := filepath.Join(srcDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		archivePath, err := collector.GetArchive(srcDir, outDir)
		if err != nil {
			t.Errorf("GetArchive() unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(archivePath) })
		if archivePath == "" {
			t.Error("GetArchive() returned empty string")
		}
		if !strings.HasSuffix(archivePath, ".tar.xz") {
			t.Errorf("GetArchive() = %q, want file ending with '.tar.xz'", archivePath)
		}
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("GetArchive() archive does not exist: %s", archivePath)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		srcDir := t.TempDir()
		outDir := t.TempDir()
		archivePath, err := collector.GetArchive(srcDir, outDir)
		if err != nil {
			t.Errorf("GetArchive() unexpected error for empty directory: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(archivePath) })
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("GetArchive() archive does not exist for empty directory: %s", archivePath)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		outDir := t.TempDir()
		nonexistentDir := "/nonexistent/path"
		_, err := collector.GetArchive(nonexistentDir, outDir)
		if err == nil {
			t.Error("GetArchive() expected error for nonexistent directory")
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

// TestUploadArchive verifies the behavior of uploadArchive.
func TestUploadArchive(t *testing.T) {
	testConfig := collector.Config{
		ID:          "test.collector",
		Name:        "Test Collector",
		ContentType: "application/vnd.redhat.advisor.collection",
	}

	// FIXME What is this testing, without any asserts?
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

// TestCleanup verifies that the cleanup function removes a file or directory
// when called.
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
