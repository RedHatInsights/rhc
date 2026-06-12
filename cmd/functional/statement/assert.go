package statement

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/redhatinsights/rhc/cmd/functional/support"
)

// Connected verifies the system is fully connected: an RHSM consumer
// certificate is present and yggdrasil.service is active.
func Connected(ctx context.Context) error {
	if err := consumerCertExists(); err != nil {
		return err
	}
	r, err := support.RunCommand("systemctl is-active yggdrasil.service")
	if err != nil {
		return fmt.Errorf("checking yggdrasil.service: %w", err)
	}
	if r.ExitCode != 0 {
		return fmt.Errorf(
			"yggdrasil.service is not active (status: %s)",
			strings.TrimSpace(r.Stdout),
		)
	}
	return nil
}

// HasIdentity verifies the system has a Red Hat Insights machine identity,
// represented by /etc/insights-client/machine-id.
func HasIdentity(ctx context.Context) error {
	const machineID = "/etc/insights-client/machine-id"
	if _, err := os.Stat(machineID); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("system has no Insights identity: %s does not exist", machineID)
		}
		return fmt.Errorf("stat %s: %w", machineID, err)
	}
	return nil
}

// HasContent verifies that insights-client is registered for content delivery,
// indicated by the presence of /etc/insights-client/.registered.
func HasContent(ctx context.Context) error {
	const registeredFile = "/etc/insights-client/.registered"
	if _, err := os.Stat(registeredFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"system does not have content: %s does not exist",
				registeredFile,
			)
		}
		return fmt.Errorf("stat %s: %w", registeredFile, err)
	}
	return nil
}

// AllDataCollectorsDisabled verifies that no rhc-collector timer is currently
// scheduled, using the `rhc collector list --format json` CLI command.
//
// A collector is considered disabled when its next_run field is absent from
// the JSON output, meaning the timer is not scheduled for a future run.
func AllDataCollectorsDisabled(ctx context.Context) error {
	r, err := support.RunCommand("rhc collector list --format json")
	if err != nil {
		return fmt.Errorf("running rhc collector list: %w", err)
	}
	if r.ExitCode != 0 {
		return fmt.Errorf(
			"rhc collector list failed (exit %d)\n--- stderr ---\n%s",
			r.ExitCode, r.Stderr,
		)
	}

	output := strings.TrimSpace(r.Stdout)

	// No collectors installed: the command outputs "{}" (an empty object).
	if output == "{}" {
		return nil
	}

	var collectors []struct {
		ID      string `json:"id"`
		NextRun *int   `json:"next_run"`
	}
	if err := json.Unmarshal([]byte(output), &collectors); err != nil {
		return fmt.Errorf(
			"parsing collector list output: %w\n--- stdout ---\n%s",
			err, r.Stdout,
		)
	}

	for _, c := range collectors {
		if c.NextRun != nil {
			return fmt.Errorf(
				"collector %q is not disabled: timer is scheduled (next_run=%d)",
				c.ID, *c.NextRun,
			)
		}
	}
	return nil
}

// SystemdUnitIsEnabled verifies that the named systemd unit is enabled.
func SystemdUnitIsEnabled(ctx context.Context, unit string) error {
	r, err := support.RunCommand("systemctl is-enabled " + unit)
	if err != nil {
		return fmt.Errorf("checking %s: %w", unit, err)
	}
	if r.ExitCode != 0 {
		return fmt.Errorf(
			"systemd unit %q is not enabled (status: %s)",
			unit, strings.TrimSpace(r.Stdout),
		)
	}
	return nil
}

// JournalContains verifies that the systemd journal for the given unit
// contains a substring.
func JournalContains(ctx context.Context, unit, substr string) error {
	r, err := support.RunCommand(
		fmt.Sprintf("journalctl --unit=%s --no-pager --output=cat", unit),
	)
	if err != nil {
		return fmt.Errorf("reading journal for %s: %w", unit, err)
	}
	if !strings.Contains(r.Stdout, substr) {
		return fmt.Errorf(
			"journal for unit %q does not contain %q\n--- journal ---\n%s",
			unit, substr, r.Stdout,
		)
	}
	return nil
}

// ExitCode verifies that the last command exited with the expected code.
func ExitCode(ctx context.Context, expected int) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	if r.ExitCode != expected {
		return fmt.Errorf(
			"expected exit code %d, got %d\n--- stdout ---\n%s--- stderr ---\n%s",
			expected, r.ExitCode, r.Stdout, r.Stderr,
		)
	}
	return nil
}

// ExitCodeNot verifies that the last command did not exit with the given code.
func ExitCodeNot(ctx context.Context, unexpected int) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	if r.ExitCode == unexpected {
		return fmt.Errorf(
			"expected exit code to differ from %d\n--- stdout ---\n%s--- stderr ---\n%s",
			unexpected, r.Stdout, r.Stderr,
		)
	}
	return nil
}

// StdoutContains verifies that the last command's stdout includes a substring.
func StdoutContains(ctx context.Context, substr string) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(r.Stdout, substr) {
		return fmt.Errorf(
			"stdout does not contain %q\n--- stdout ---\n%s",
			substr, r.Stdout,
		)
	}
	return nil
}

// StderrContains verifies that the last command's stderr includes a substring.
func StderrContains(ctx context.Context, substr string) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(r.Stderr, substr) {
		return fmt.Errorf(
			"stderr does not contain %q\n--- stderr ---\n%s",
			substr, r.Stderr,
		)
	}
	return nil
}

// StdoutIsJSON verifies that the last command's stdout is valid JSON.
func StdoutIsJSON(ctx context.Context) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(r.Stdout)) {
		return fmt.Errorf("stdout is not valid JSON\n--- stdout ---\n%s", r.Stdout)
	}
	return nil
}

// StdoutJSONField verifies that a dot-separated field in the stdout JSON equals
// the expected canonical JSON value.  The expected argument must be a JSON
// literal: `false`, `42`, `"hello"` (with quotes), etc.
//
// Examples:
//
//	stdout JSON field `.connected` is `false`
//	stdout JSON field `.name` is `"content"`
func StdoutJSONField(ctx context.Context, field, expected string) error {
	r, err := support.GetResult(ctx)
	if err != nil {
		return err
	}
	var doc interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
		return fmt.Errorf("stdout is not valid JSON: %w", err)
	}
	value, err := jsonLookup(doc, field)
	if err != nil {
		return fmt.Errorf("JSON field %q: %w", field, err)
	}
	actualBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("JSON field %q: could not marshal: %w", field, err)
	}
	if actual := string(actualBytes); actual != expected {
		return fmt.Errorf("JSON field %q: expected %q, got %q", field, expected, actual)
	}
	return nil
}

// FileExists verifies that a file (or directory) exists at the given path.
func FileExists(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file %s does not exist", path)
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return nil
}

// FileNotExists verifies that nothing exists at the given path.
func FileNotExists(ctx context.Context, path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("file %s exists but should not", path)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("stat %s: %w", path, err)
}

// FileContains verifies that the file at the given path contains a substring.
func FileContains(ctx context.Context, path, substr string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if !strings.Contains(string(content), substr) {
		return fmt.Errorf(
			"file %s does not contain %q\n--- content ---\n%s",
			path, substr, content,
		)
	}
	return nil
}

// TemporaryDirectoryNotEmpty verifies that the temporary directory created by
// a preceding "run in a temporary directory" step is non-empty.
func TemporaryDirectoryNotEmpty(ctx context.Context) error {
	dir, err := support.GetWorkdir(ctx)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading temporary directory %s: %w", dir, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf(
			"temporary directory %s is empty; expected collector output files",
			dir,
		)
	}
	return nil
}
