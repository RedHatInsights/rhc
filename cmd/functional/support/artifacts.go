package support

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)


// ArtifactsDir returns the base directory for all test artifacts.
// If $TMT_PLAN_DATA is set it is used as the root; otherwise the current
// working directory is used.
func ArtifactsDir() string {
	base := os.Getenv("TMT_PLAN_DATA")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "artifacts")
}

// CollectArtifacts gathers all relevant configuration files, log files, and
// SELinux AVC denials into a per-feature subdirectory under ArtifactsDir().
// It is always called (not only on failure) so that a complete picture of every
// test run is preserved.
//
// avc must already be stopped (Stop called) before CollectArtifacts is invoked;
// its time window is used to scope the journal and AVC queries.
//
// featureName is used as the directory name; it is sanitised to be
// filesystem-safe.
func CollectArtifacts(featureName string, avc *AVCChecker) error {
	dir := filepath.Join(ArtifactsDir(), sanitise(featureName))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact dir %s: %w", dir, err)
	}

	var errs []string

	// --- files and directories -----------------------------------------------
	// Derive sources from managedPaths so there is a single place to update
	// when coverage for a new path is added.  Each entry mirrors the original
	// path under the artifact directory (e.g. etc/rhsm/rhsm.conf →
	// <dir>/etc/rhsm/rhsm.conf).  Missing sources are silently skipped.
	for _, mp := range managedPaths {
		for _, rel := range mp.collectPaths() {
			src := filepath.Join("/", rel)
			dest := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				errs = append(errs, err.Error())
				continue
			}
			if err := copyPathTo(src, dest); err != nil {
				if !os.IsNotExist(err) {
					errs = append(errs, err.Error())
				}
			}
		}
	}

	// --- journal log for relevant units --------------------------------------
	// Scope the journal window to the scenario's start time so that log
	// entries from prior runs are not mixed in.
	var journalSince time.Time
	if avc != nil {
		journalSince = avc.StartTime()
	}
	if journalSince.IsZero() {
		journalSince = time.Now().Add(-24 * time.Hour)
	}

	journalUnits := []string{"rhcd", "yggdrasil", "rhsmcertd"}
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		errs = append(errs, err.Error())
	} else {
		for _, unit := range journalUnits {
			if err := collectJournal(unit, logDir, journalSince); err != nil {
				slog.Debug("failed to collect journal", "unit", unit, "err", err)
			}
		}
	}

	// --- SELinux AVC denials -------------------------------------------------
	if avc != nil {
		selinuxDir := filepath.Join(dir, "selinux")
		if err := os.MkdirAll(selinuxDir, 0755); err != nil {
			errs = append(errs, err.Error())
		} else {
			if err := collectAVCs(avc, selinuxDir); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("artifact collection errors: %s", strings.Join(errs, "; "))
	}

	slog.Info("artifacts collected", "dir", dir)
	return nil
}

// --- helpers -----------------------------------------------------------------

// sanitise replaces characters that are unsafe in directory names.
func sanitise(name string) string {
	r := strings.NewReplacer(
		"/", "-",
		" ", "-",
		":", "-",
	)
	return strings.ToLower(r.Replace(name))
}

// copyPathTo copies a file or directory tree from src to dest, where dest is
// the fully-qualified destination path (not a parent directory).
func copyPathTo(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest)
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPathTo(filepath.Join(src, e.Name()), filepath.Join(dest, e.Name())); err != nil && !os.IsNotExist(err) {
			slog.Debug("failed to copy artifact entry", "entry", e.Name(), "err", err)
		}
	}
	return nil
}

func copyFile(src, dest string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dest)
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

// collectJournal writes journalctl output for unit (scoped to entries since
// the given time) into destDir/<unit>-journal.log.
func collectJournal(unit, destDir string, since time.Time) error {
	out, err := exec.Command(
		"journalctl", "--unit", unit,
		"--since", since.Format("2006-01-02 15:04:05"),
		"--no-pager",
	).Output()
	if err != nil {
		// journalctl exits 1 when the unit has no entries - not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("journalctl for %s: %w", unit, err)
	}

	dest := filepath.Join(destDir, unit+"-journal.log")
	return os.WriteFile(dest, out, 0644)
}

func collectAVCs(avc *AVCChecker, destDir string) error {
	denials, err := avc.GetDenials()
	if err != nil {
		return fmt.Errorf("failed to query AVC denials: %w", err)
	}

	var lines []string
	for _, d := range denials {
		lines = append(lines, d.Raw)
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}

	dest := filepath.Join(destDir, "avc-denials.log")
	return os.WriteFile(dest, []byte(content), 0644)
}
