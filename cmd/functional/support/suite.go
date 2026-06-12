package support

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// contextKey is an unexported type for context keys used in this package.
type contextKey int

const (
	resultKey   contextKey = iota // *CommandResult
	scenarioKey                   // *SystemState
	avcKey                        // *AVCChecker
	workdirKey                    // string — path of per-scenario temporary directory
)

// TestTarget is the resolved value of TARGET, set once at suite init.
var TestTarget string

// TestConfig is the resolved value of CONFIG, set once at suite init.
var TestConfig string

// InitializeTestSuite is called once before any scenario runs.
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		// 1. Validate and record the test target.
		target, err := ValidateTarget()
		if err != nil {
			panic(err)
		}
		TestTarget = target

		// 2. Validate and record the config directory.
		config, err := ValidateConfig()
		if err != nil {
			panic(err)
		}
		TestConfig = config
		slog.Info("functional test suite starting", "target", target, "config", config)

		// 3. Reset the process environment to a known-good state.
		ResetEnvironment()
		slog.Info("environment reset to clean state")
	})
}

// NewScenarioInitializer returns a godog ScenarioInitializer that wires step
// definitions and AVC skip patterns into every scenario.
//
// stepRegistrar is called once per scenario to register Gherkin step
// definitions.  avcSkipRegistrar, when non-nil, is called once per scenario
// immediately after the AVCChecker is created so that known-good denial
// patterns can be pre-registered via AVCChecker.SkipPattern.
func NewScenarioInitializer(
	stepRegistrar func(*godog.ScenarioContext),
	avcSkipRegistrar func(*AVCChecker),
) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		initializeScenario(sc, stepRegistrar, avcSkipRegistrar)
	}
}

// initializeScenario is the shared implementation used by NewScenarioInitializer.
func initializeScenario(sc *godog.ScenarioContext, stepRegistrar func(*godog.ScenarioContext), avcSkipRegistrar func(*AVCChecker)) {
	// Register step definitions.
	if stepRegistrar != nil {
		stepRegistrar(sc)
	}

	// Skip scenarios whose tags do not match the current test target.
	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		if err := checkTargetTags(s); err != nil {
			return ctx, err
		}
		return ctx, nil
	})

	// Before each scenario: start AVC monitoring and set up the per-scenario
	// filesystem state (snapshot managed paths, install fixture config files).
	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		avc := NewAVCChecker()
		if avcSkipRegistrar != nil {
			avcSkipRegistrar(avc)
		}
		ctx = context.WithValue(ctx, avcKey, avc)

		state, err := SetupScenario()
		if err != nil {
			return ctx, fmt.Errorf("scenario setup failed: %w", err)
		}
		ctx = context.WithValue(ctx, scenarioKey, state)

		// Ensure the feature preferences file is absent so every scenario
		// starts from default preferences.  /var/lib/rhc is already
		// snapshotted above and will be restored by Cleanup, so this
		// deletion does not leak between scenarios.
		const connectFeaturesPrefsFile = "/var/lib/rhc/rhc-connect-features-prefs.json"
		if err := os.Remove(connectFeaturesPrefsFile); err != nil && !os.IsNotExist(err) {
			return ctx, fmt.Errorf("removing feature preferences file: %w", err)
		}

		slog.Debug("scenario setup complete", "scenario", s.Name)
		return ctx, nil
	})

	// After each scenario: collect artifacts, then restore the filesystem to
	// its pre-scenario state.
	sc.After(func(ctx context.Context, s *godog.Scenario, scenarioErr error) (context.Context, error) {
		var cleanupErrs []error

		avc, _ := ctx.Value(avcKey).(*AVCChecker)
		state, _ := ctx.Value(scenarioKey).(*SystemState)

		// Close the AVC monitoring window before collecting artifacts so the
		// time range is bounded.  CollectArtifacts requires Stop to have been
		// called first.
		if avc != nil {
			avc.Stop()
		}

		// Collect artifacts before restoring so we still have access to any
		// files written during the scenario.
		//
		// Name artifacts as {directory}-{file}-{test} so that the artifact
		// directory identifies both the feature file and the scenario within
		// it, rather than just the scenario title.  s.Uri is a path such as
		// "tests-functional/features/configure-feature/disconnected.feature".
		featureDir := filepath.Base(filepath.Dir(s.Uri))
		featureFile := strings.TrimSuffix(filepath.Base(s.Uri), ".feature")
		artifactName := featureDir + "-" + featureFile + "-" + s.Name
		if err := CollectArtifacts(artifactName, avc); err != nil {
			slog.Warn("artifact collection failed", "scenario", s.Name, "err", err)
			cleanupErrs = append(cleanupErrs, err)
		}

		// Disable any rhc-collector timers that may have been enabled during
		// the scenario.  systemctl enable creates symlinks under
		// /etc/systemd/system/timers.target.wants/ which the file-based
		// snapshot system cannot restore correctly; use systemctl directly.
		if err := disableCollectorTimers(); err != nil {
			slog.Warn("failed to disable collector timers", "err", err)
		}

		// Restore all managed paths to their pre-scenario state.
		if state != nil {
			if err := state.Cleanup(); err != nil {
				slog.Error("scenario cleanup failed", "scenario", s.Name, "err", err)
				cleanupErrs = append(cleanupErrs, err)
			}
		}

		// Remove the per-scenario temporary directory when one was created.
		if dir, ok := ctx.Value(workdirKey).(string); ok && dir != "" {
			if err := os.RemoveAll(dir); err != nil {
				slog.Warn("failed to remove temporary directory", "dir", dir, "err", err)
			}
		}

		// Fail the scenario if unexpected AVC denials were detected.
		// Known-good denials are pre-registered via AddKnownAVCSkips.
		if avc != nil {
			if has, err := avc.HasUnexpectedDenials(); err != nil {
				slog.Warn("failed to check AVC denials", "err", err)
			} else if has {
				cleanupErrs = append(cleanupErrs,
					fmt.Errorf("SELinux AVC denials detected during scenario %q; "+
						"see selinux/avc-denials.log in the artifact directory", s.Name),
				)
			}
		}

		if len(cleanupErrs) > 0 {
			return ctx, fmt.Errorf("post-scenario cleanup: %v", cleanupErrs)
		}
		return ctx, nil
	})
}

// disableCollectorTimers disables all currently-enabled rhc-collector systemd
// timers.  It is called unconditionally after every scenario so that timers
// enabled during a test do not leak into the next one.
func disableCollectorTimers() error {
	r, err := RunCommand("systemctl list-unit-files --type=timer --state=enabled --plain --no-legend")
	if err != nil {
		return fmt.Errorf("listing enabled timers: %w", err)
	}
	for _, line := range strings.Split(r.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		if !strings.HasPrefix(unit, "rhc-collector-") {
			continue
		}
		if _, err := RunCommand("systemctl disable --now " + unit); err != nil {
			slog.Warn("failed to disable collector timer", "unit", unit, "err", err)
		}
		slog.Debug("disabled collector timer", "unit", unit)
	}
	return nil
}

// GetResult extracts the most recent CommandResult from the scenario context.
func GetResult(ctx context.Context) (*CommandResult, error) {
	r, ok := ctx.Value(resultKey).(*CommandResult)
	if !ok || r == nil {
		return nil, fmt.Errorf("no command has been run yet in this scenario")
	}
	return r, nil
}

// WithResult stores a CommandResult in the context.
func WithResult(ctx context.Context, r *CommandResult) context.Context {
	return context.WithValue(ctx, resultKey, r)
}

// GetWorkdir returns the per-scenario temporary directory created by a
// "run in a temporary directory" step.  Returns an error when no such
// directory has been set up yet.
func GetWorkdir(ctx context.Context) (string, error) {
	dir, ok := ctx.Value(workdirKey).(string)
	if !ok || dir == "" {
		return "", fmt.Errorf("no temporary directory has been created in this scenario")
	}
	return dir, nil
}

// WithWorkdir stores the path of the per-scenario temporary directory in the
// context so that assertion steps can find it.
func WithWorkdir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, workdirKey, dir)
}

// checkTargetTags returns godog.ErrSkip when the scenario is tagged with a
// target constraint that does not match TARGET.
func checkTargetTags(s *godog.Scenario) error {
	for _, tag := range s.Tags {
		switch tag.Name {
		case "@only-hosted":
			if TestTarget != "hosted" {
				return godog.ErrSkip
			}
		case "@only-satellite":
			if TestTarget != "satellite" {
				return godog.ErrSkip
			}
		case "@only-local":
			if TestTarget != "local" {
				return godog.ErrSkip
			}
		}
	}
	return nil
}
