package httpapi

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const (
	// unknownOS is returned when OS information cannot be determined.
	unknownOS = "unknown"
)

var (
	// osReleasePath is the path to the os-release file.
	// Can be overridden in tests to use a mock file.
	osReleasePath = "/etc/os-release"

	// cachedOSVersion stores the OS version to avoid repeated file I/O.
	cachedOSVersion string
	// cacheOnce ensures OS version is loaded only once.
	cacheOnce sync.Once
)

// GetUserAgent constructs a User-Agent header string according to ADR-009.
// Format: {component}/{version} (triggered-by: {trigger-id}) {os-id}/{os-version}
// Example: rhc-collector/1.0.0 (triggered-by: com.redhat.advisor) rhel/9.3
//
// Parameters:
//   - component: hardcoded component identifier (e.g., "rhc-collector", "rhc-server")
//   - version: build-time version string injected via ldflags
//   - triggeredBy: validated collector ID or Varlink method name
//
// All inputs must be pre-validated. Collector IDs are validated by ValidateID(),
// Varlink method names follow protocol conventions, and component names are hardcoded literals.
func GetUserAgent(component, version, triggeredBy string) string {
	osVersion := getOSIdentifier()
	return fmt.Sprintf("%s/%s (triggered-by: %s) %s",
		component,
		version,
		triggeredBy,
		osVersion,
	)
}

// getOSIdentifier reads the OS version from /etc/os-release.
// Returns ID/VERSION_ID (e.g., "rhel/9.3", "fedora/39", "centos/9").
// The result is cached to avoid repeated file I/O.
func getOSIdentifier() string {
	cacheOnce.Do(func() {
		env, err := loadEnv(osReleasePath)
		if err != nil {
			slog.Debug("Failed to load os-release", "path", osReleasePath, "error", err)
			cachedOSVersion = unknownOS
			return
		}

		id := env["ID"]
		version := env["VERSION_ID"]

		if id == "" || version == "" {
			slog.Debug("Missing required fields in os-release", "path", osReleasePath, "ID", id, "VERSION_ID", version)
			cachedOSVersion = unknownOS
			return
		}

		cachedOSVersion = fmt.Sprintf("%s/%s", id, version)
	})

	return cachedOSVersion
}

// loadEnv parses a simple environment-style file and returns a map of key-value pairs.
// Handles both quoted (KEY="value") and unquoted (KEY=value) values.
// Skips empty lines and comments (lines starting with #).
// Returns an error if the file cannot be read.
//
// This is suitable for files with simple KEY=VALUE format like os-release,
// but does not handle complex shell constructs like multiline strings.
func loadEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	env := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], `"'`)

		env[key] = value
	}

	return env, nil
}
