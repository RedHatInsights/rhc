package prefcache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// PreferenceCache manages feature preferences in memory with lazy file persistence.
type PreferenceCache struct {
	filePath string
	prefs    map[string]bool
	// dirty is true when in-memory prefs differ from their expected file state
	dirty bool
}

// defaultPrefs returns the default preference values.
func defaultPrefs() map[string]bool {
	return map[string]bool{
		"content":           true,
		"analytics":         true,
		"remote-management": true,
	}
}

// validatePath ensures the path is absolute and clean.
func validatePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	if !filepath.IsAbs(filePath) {
		return fmt.Errorf("file path must be absolute: %q", filePath)
	}
	cleanPath := filepath.Clean(filePath)
	if cleanPath != filePath {
		return fmt.Errorf("file path contains invalid elements: %q", filePath)
	}
	return nil
}

// isDefault returns true if the current preferences match the default ones.
func (c *PreferenceCache) isDefault() bool {
	for k, v := range defaultPrefs() {
		if value, exists := c.prefs[k]; !exists || value != v {
			return false
		}
	}
	return true
}

// NewDefaultCache returns a PreferenceCache instance with default values.
func NewDefaultCache(filePath string) (*PreferenceCache, error) {
	if err := validatePath(filePath); err != nil {
		return nil, err
	}
	return &PreferenceCache{
		filePath: filePath,
		prefs:    defaultPrefs(),
		dirty:    false,
	}, nil
}

// LoadCache loads feature preferences from the file into memory.
// Returns a default cache if the file does not exist.
func LoadCache(filePath string) (*PreferenceCache, error) {
	cache, err := NewDefaultCache(filePath)
	if err != nil {
		return nil, err
	}

	rawContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("using default set of features")
			return cache, nil
		}
		return nil, fmt.Errorf("failed to read preferences file: %w", err)
	}
	var content map[string]bool
	if err = json.Unmarshal(rawContent, &content); err != nil {
		return nil, fmt.Errorf("failed to parse preferences file: %w", err)
	}

	// In case the filesystem cache is dirty or so old that we have deprecated
	// some values, start with defaults and overlay the loaded ones.
	for k, v := range content {
		if _, exists := cache.prefs[k]; exists {
			cache.prefs[k] = v
		} else {
			slog.Debug("ignoring unknown preference key from file", "key", k, "value", v)
		}
	}

	slog.Debug("loaded feature preferences cache", "path", filePath)
	return cache, nil
}

// Get returns the preference value for a feature by name.
// Returns an error if the feature preference is not set.
func (c *PreferenceCache) Get(featureName string) (bool, error) {
	value, exists := c.prefs[featureName]
	if !exists {
		return false, fmt.Errorf("no such feature: %q", featureName)
	}
	return value, nil
}

// Set updates the preference value for a feature by name and marks the cache as dirty.
// Returns an error if the feature name does not exist in the cache.
func (c *PreferenceCache) Set(featureName string, enabled bool) error {
	currentValue, exists := c.prefs[featureName]
	if !exists {
		return fmt.Errorf("no such feature: %q", featureName)
	}
	if currentValue != enabled {
		c.prefs[featureName] = enabled
		c.dirty = true
	}
	return nil
}

// Save writes the cache to the disk if it has been modified.
func (c *PreferenceCache) Save() error {
	if !c.dirty {
		return nil
	}

	dirPath := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	if c.isDefault() {
		if err := os.Remove(c.filePath); err != nil {
			if os.IsNotExist(err) {
				slog.Debug("preferences cache contains default values, not saving it", "path", c.filePath)
			} else {
				return fmt.Errorf("failed to delete preferences cache: %w", err)
			}
		} else {
			slog.Debug("preferences cache contains default values, cache file was removed", "path", c.filePath)
		}
		c.dirty = false
		return nil
	}

	content, err := json.MarshalIndent(c.prefs, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	if err = os.WriteFile(c.filePath, content, 0640); err != nil {
		return fmt.Errorf("failed to write preferences file: %w", err)
	}

	slog.Debug("saved feature preferences cache", "path", c.filePath)
	c.dirty = false
	return nil
}

// Delete removes the preference file from the disk and resets to default preferences.
func (c *PreferenceCache) Delete() error {
	if err := os.Remove(c.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete preferences file: %w", err)
	}

	c.prefs = defaultPrefs()
	c.dirty = false
	slog.Debug("deleted feature preferences cache", "path", c.filePath)
	return nil
}
