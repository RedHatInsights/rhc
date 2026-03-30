package prefcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a temporary directory for tests.
// Returns the temporary directory path and a closure to clean it up.
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "prefcache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}
	return tmpDir, cleanup
}

// assertCacheDirty validates the dirty flag of PreferenceCache
func assertCacheDirty(t *testing.T, c *PreferenceCache, stage string, dirty bool) {
	t.Helper()

	if c.dirty != dirty {
		t.Errorf("%s: dirty was expected %v, got %v", stage, dirty, c.dirty)
	}
}

// assertCacheFeatures validates the values in PreferenceCache
func assertCacheFeatures(t *testing.T, c *PreferenceCache, stage string, content, analytics, mgmt bool) {
	t.Helper()

	{
		v, e := c.Get("content")
		if e != nil {
			t.Errorf("%s: 'content' must exist, got %v", stage, e)
		}
		if v != content {
			t.Errorf("%s: content: expected %v, got %v", stage, content, v)
		}
	}
	{
		v, e := c.Get("analytics")
		if e != nil {
			t.Errorf("%s: 'analytics' must exist, got %v", stage, e)
		}
		if v != analytics {
			t.Errorf("%s: analytics: expected %v, got %v", stage, analytics, v)
		}
	}
	{
		v, e := c.Get("remote-management")
		if e != nil {
			t.Errorf("%s: 'remote-management' must exist, got %v", stage, e)
		}
		if v != mgmt {
			t.Errorf("%s: remote-management: expected %v, got %v", stage, mgmt, v)
		}
	}
}

// assertFileAbsent validates a path does not exist
func assertFileAbsent(t *testing.T, stage, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Errorf("%s: expected file %s to not exist, but it does", stage, path)
	} else if !os.IsNotExist(err) {
		t.Errorf("%s: failed to stat file %s: %v", stage, path, err)
	}
}

// assertFileContent reads the cache file and verifies the map values
func assertFileContent(t *testing.T, stage, path string, content, analytics, mgmt bool) {
	t.Helper()

	stat, err := os.Stat(path)
	if err != nil {
		t.Errorf("%s: failed to stat file %s: %v", stage, path, err)
		return
	}
	if stat.Mode().Perm() != os.FileMode(0640) {
		t.Errorf("%s: expected file mode %o, got %o", stage, os.FileMode(0640), stat.Mode().Perm())
	}

	var prefs map[string]bool
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: failed to read file %s: %v", stage, path, err)
		return
	}
	if err = json.Unmarshal(raw, &prefs); err != nil {
		t.Errorf("%s: failed to parse file content: %v", stage, err)
		return
	}
	if prefs["content"] != content {
		t.Errorf("%s: content was expected %v, got %v", stage, content, prefs["content"])
	}
	if prefs["analytics"] != analytics {
		t.Errorf("%s: analytics was expected %v, got %v", stage, analytics, prefs["analytics"])
	}
	if prefs["remote-management"] != mgmt {
		t.Errorf("%s: remote-management was expected %v, got %v", stage, mgmt, prefs["remote-management"])
	}
}

// TestFunctional walks through the lifecycle of a preference cache and
// validates the state of the cache object and its file after each step.
func TestFunctional(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	filePath := filepath.Join(tmpDir, "prefs.json")

	cache, err := NewDefaultCache(filePath)
	if err != nil {
		t.Fatalf("DefaultCache() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "DefaultCache()", false)
	assertCacheFeatures(t, cache, "DefaultCache()", true, true, true)
	assertFileAbsent(t, "DefaultCache()", filePath)

	// Enabling already enabled feature is noop; keep 'dirty' false
	if err := cache.Set("remote-management", true); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Set() [noop]", false)
	assertCacheFeatures(t, cache, "Set() [noop]", true, true, true)
	assertFileAbsent(t, "Set() [noop]", filePath)

	// Disabling enabled feature sets 'dirty' and the feature bool
	if err := cache.Set("remote-management", false); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Set() [dirty]", true)
	assertCacheFeatures(t, cache, "Set() [dirty]", true, true, false)
	assertFileAbsent(t, "Set() [dirty]", filePath)

	// Saving dirty cache sets 'dirty' to false
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Save()", false)
	assertCacheFeatures(t, cache, "Save()", true, true, false)
	assertFileContent(t, "Save()", filePath, true, true, false)

	// Reload from disk to verify state restore
	cache, err = LoadCache(filePath)
	if err != nil {
		t.Fatalf("LoadCache() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Reload", false)
	assertCacheFeatures(t, cache, "Reload", true, true, false)
	assertFileContent(t, "Reload", filePath, true, true, false)

	// Disabling already disabled feature is noop; keep 'dirty' false
	if err = cache.Set("remote-management", false); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Set() [noop,file]", false)
	assertCacheFeatures(t, cache, "Set() [noop,file]", true, true, false)
	assertFileContent(t, "Set() [noop,file]", filePath, true, true, false)

	// Disabling enabled feature sets 'dirty' to true
	if err = cache.Set("remote-management", true); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Set() [dirty,file]", true)
	assertCacheFeatures(t, cache, "Set() [dirty,file]", true, true, true)
	assertFileContent(t, "Set() [dirty,file]", filePath, true, true, false)

	// Deleting a cache clears 'dirty' flag, resets the features, and deletes the file
	if err = cache.Delete(); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Delete()", false)
	assertCacheFeatures(t, cache, "Delete()", true, true, true)
	assertFileAbsent(t, "Delete()", filePath)
}

// TestFunctionalFSBacked validates some special behavior of a PreferenceCache:
// calling Save on a default-like cache causes the file to be removed.
func TestFunctionalFSBacked(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	filePath := filepath.Join(tmpDir, "prefs.json")

	// Write down some possible state
	prefsJSON := []byte(`{"content": true, "analytics": true, "remote-management": false}`)
	if err := os.WriteFile(filePath, prefsJSON, 0640); err != nil {
		t.Fatalf("failed to write initial prefs file: %v", err)
	}

	// Initialize the cache from filesystem
	cache, err := LoadCache(filePath)
	if err != nil {
		t.Fatalf("LoadCache() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "LoadCache()", false)
	assertCacheFeatures(t, cache, "LoadCache()", true, true, false)
	assertFileContent(t, "LoadCache()", filePath, true, true, false)

	// Enable remote management, getting us to the default state
	if err = cache.Set("remote-management", true); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Set()", true)
	assertCacheFeatures(t, cache, "Set()", true, true, true)
	assertFileContent(t, "Set()", filePath, true, true, false)

	// Delete the file by saving it
	if err = cache.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	assertCacheDirty(t, cache, "Save()", false)
	assertCacheFeatures(t, cache, "Save()", true, true, true)
	assertFileAbsent(t, "Save()", filePath)
}

func TestPathValidation(t *testing.T) {
	invalidPaths := []string{
		"",
		"relative/prefs.json",
		"/tmp/../etc/passwd",
		"/tmp//prefs.json",
		"/trailing/slash/",
	}
	for _, path := range invalidPaths {
		t.Run(path, func(t *testing.T) {
			if err := validatePath(path); err == nil {
				t.Errorf("validatePath('%q') expected an error, got nil", path)
			}
		})
	}
}

// TestInvalidInput validates Get() and Set() return errors on invalid feature names.
func TestInvalidInput(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	filePath := filepath.Join(tmpDir, "prefs.json")

	cache, err := NewDefaultCache(filePath)
	if err != nil {
		t.Fatalf("NewDefaultCache() returned error: %v", err)
	}

	// Test Get() with invalid feature name
	_, err = cache.Get("nonexistent-feature")
	if err == nil {
		t.Errorf("Get('nonexistent-feature') expected error, got nil")
	}

	// Test Set() with invalid feature name
	err = cache.Set("invalid-feature", true)
	if err == nil {
		t.Errorf("Set('invalid-feature', true) expected error, got nil")
	}
}
