package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redhatinsights/rhc/internal/collector"
)

func TestCreateTmpDir(t *testing.T) {
	tmpDir, err := createTmpDir()
	if err != nil {
		t.Fatalf("createTmpDir() unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to clean up temp dir: %v", err)
		}
	})

	if filepath.Dir(tmpDir) != collectorTmpParent {
		t.Errorf("createTmpDir() = %q, want parent %q", tmpDir, collectorTmpParent)
	}
	if !strings.HasPrefix(filepath.Base(tmpDir), "rhc-") {
		t.Errorf("createTmpDir() = %q, want name beginning with %q", tmpDir, "rhc-")
	}
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("created directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
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
		config := collector.Config{
			ID:    "com.redhat.nonexistent",
			User:  "root",
			Group: "root",
		}
		err := executeCollector(config, tmpDir)
		if err == nil {
			t.Error("executeCollector() expected error for nonexistent collector binary")
		}
		if !strings.Contains(err.Error(), "failed to execute collector") {
			t.Errorf("executeCollector() error = %v, want error containing 'failed to execute collector'", err)
		}
	})

	t.Run("collector binary with invalid ID characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := collector.Config{
			ID:    "../../../etc/passwd",
			User:  "root",
			Group: "root",
		}
		err := executeCollector(config, tmpDir)
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

	t.Run("empty output directory", func(t *testing.T) {
		_, err := collector.GetArchive(t.TempDir(), "")
		if err == nil {
			t.Error("GetArchive() expected error for empty output directory")
		}
	})

	t.Run("workspace archive excludes itself", func(t *testing.T) {
		tmpDir := t.TempDir()
		workDir := filepath.Join(tmpDir, "workdir")
		if err := os.Mkdir(workDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir, "test.txt"), []byte("test content"), 0600); err != nil {
			t.Fatal(err)
		}
		archivePath, err := getArchivePath(workDir, tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(archivePath) != tmpDir {
			t.Errorf("archive directory = %q, want %q", filepath.Dir(archivePath), tmpDir)
		}
		contents, err := exec.Command("tar", "--list", "--xz", "--file", archivePath).Output()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "./test.txt\n") || strings.Contains(string(contents), filepath.Base(archivePath)) {
			t.Errorf("archive contents = %q, want test.txt without the archive itself", contents)
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
