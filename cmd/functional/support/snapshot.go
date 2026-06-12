package support

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// managedPath describes a filesystem path managed by the test suite.
//
// All entries are collected as test artifacts after each scenario.
// Entries with a non-empty snapshotPath are also snapshotted before the
// scenario and restored after; a matching fixture from $CONF is installed
// if one exists (missing fixtures are silently skipped at debug level).
//
// artifactPaths, when non-empty, overrides the paths used for artifact
// collection — use this when a directory is snapshotted whole but only
// specific files within it are worth preserving as artifacts.  An entry
// may have artifactPaths without a snapshotPath (artifact-only).
//
// Add a new entry to managedPaths whenever a scenario touches a new path
// that should be snapshotted, injected, or collected.
type managedPath struct {
	snapshotPath  string
	artifactPaths []string
}

// managedPaths is the single source of truth for every filesystem path the
// test suite touches.  Update this list — and nowhere else — when adding
// coverage for a new path.
var managedPaths = []managedPath{
	// Files captured before each scenario and restored after.
	// A matching fixture from $CONF is installed when present.
	{snapshotPath: "etc/rhsm/rhsm.conf"},
	{snapshotPath: "etc/insights-client/insights-client.conf"},
	{snapshotPath: "etc/yggdrasil/config.toml"},
	{snapshotPath: "etc/yum.repos.d", artifactPaths: []string{
		"etc/yum.repos.d/redhat.repo",
	}},
	{snapshotPath: "var/lib/rhc"},
	{snapshotPath: "var/lib/rhsm"},
	{snapshotPath: "etc/pki/consumer", artifactPaths: []string{
		"etc/pki/consumer/cert.pem",
		"etc/pki/consumer/key.pem",
	}},
	// Collector configuration and state: snapshotted so that TOML configs or
	// timer-cache JSON files written during a scenario do not persist into the
	// next one.  A matching fixture is installed when present in $CONF.
	{snapshotPath: "usr/lib/rhc/collectors"},
	{snapshotPath: "var/cache/rhc/collectors"},
	// Artifact-only: collected but not snapshotted or restored.
	{artifactPaths: []string{"etc/rhc"}},
	{artifactPaths: []string{
		"var/log/rhsm/rhsm.log",
		"var/log/rhsm/rhsmcertd.log",
	}},
}

// collectPaths returns the paths used for artifact collection.
// Falls back to snapshotPath when artifactPaths is not set.
func (mp managedPath) collectPaths() []string {
	if len(mp.artifactPaths) > 0 {
		return mp.artifactPaths
	}
	if mp.snapshotPath != "" {
		return []string{mp.snapshotPath}
	}
	return nil
}

// fileEntry holds the saved content of one regular file.
type fileEntry struct {
	path string
	mode fs.FileMode
	data []byte
}

// SystemState is the per-scenario handle returned by SetupScenario.
// It holds the pre-scenario filesystem snapshots and restores them on Cleanup.
type SystemState struct {
	files []fileEntry
}

// SetupScenario captures the current state of all managed paths, then installs
// fixture config files from CONF.  Call Cleanup() when the scenario is done
// to restore everything to its pre-scenario state.
func SetupScenario() (*SystemState, error) {
	s := &SystemState{}

	for _, mp := range managedPaths {
		if mp.snapshotPath == "" {
			continue
		}
		abs := filepath.Join("/", mp.snapshotPath)

		// 1. Snapshot every path that a scenario might touch.
		entries, err := collectFiles(abs)
		if err != nil {
			_ = s.Cleanup()
			return nil, fmt.Errorf("snapshot %s: %w", abs, err)
		}
		s.files = append(s.files, entries...)
		slog.Debug("snapshotted path", "path", abs, "files", len(entries))

		// 2. Install the matching fixture from $CONF if one exists.
		src := filepath.Join(TestConfig, mp.snapshotPath)
		if err := installFile(src, abs); err != nil {
			if os.IsNotExist(err) {
				slog.Debug("fixture file not present, skipping", "path", mp.snapshotPath)
				continue
			}
			_ = s.Cleanup()
			return nil, fmt.Errorf("install fixture %s: %w", mp.snapshotPath, err)
		}
		slog.Debug("installed fixture file", "dst", abs)
	}

	return s, nil
}

// Cleanup restores every backed-up path to its pre-scenario state. All
// operations are attempted; errors are collected and returned together.
func (s *SystemState) Cleanup() error {
	var errs []string

	// 1. Clear the current state of every snapshotted path (in reverse order)
	// so that files created during the scenario do not bleed into the next one.
	for i := len(managedPaths) - 1; i >= 0; i-- {
		if managedPaths[i].snapshotPath == "" {
			continue
		}
		abs := filepath.Join("/", managedPaths[i].snapshotPath)
		if err := clearPath(abs); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// 2. Write back every file that existed before the scenario.
	for _, entry := range s.files {
		if err := os.MkdirAll(filepath.Dir(entry.path), 0755); err != nil {
			errs = append(errs, fmt.Sprintf("mkdir %s: %v", filepath.Dir(entry.path), err))
			continue
		}
		if err := os.WriteFile(entry.path, entry.data, entry.mode); err != nil {
			errs = append(errs, fmt.Sprintf("restore %s: %v", entry.path, err))
		}
	}

	slog.Debug("restored paths", "files", len(s.files))
	if len(errs) > 0 {
		return fmt.Errorf("scenario cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// =============================================================================
// snapshot helpers
// =============================================================================

// collectFiles returns a fileEntry for every regular file under path.
// If the path does not exist, an empty slice is returned without error.
func collectFiles(path string) ([]fileEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if !info.IsDir() {
		entry, err := readFileEntry(path)
		if err != nil {
			return nil, err
		}
		return []fileEntry{entry}, nil
	}

	var entries []fileEntry
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		entry, err := readFileEntry(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", path, err)
	}
	return entries, nil
}

func readFileEntry(path string) (fileEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileEntry{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fileEntry{}, err
	}
	return fileEntry{path: path, mode: info.Mode(), data: data}, nil
}

// =============================================================================
// restore helpers
// =============================================================================

// clearPath removes a regular file, or clears the contents of a directory
// without removing the directory itself (safe for mount-point directories
// where os.RemoveAll would return EBUSY).  It is a no-op if path does not exist.
func clearPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return clearDirContents(path)
	}
	return os.Remove(path)
}

// clearDirContents removes every entry inside dir without removing dir itself.
func clearDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// install helpers
// =============================================================================

// installFile copies src into dst, creating parent directories as needed.
// Returns an *os.PathError wrapping fs.ErrNotExist if src does not exist,
// which the caller can test with os.IsNotExist.
func installFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}
