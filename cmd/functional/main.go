package main

import (
	"flag"
	"os"

	"github.com/cucumber/godog"
	"github.com/redhatinsights/rhc/cmd/functional/statement"
	"github.com/redhatinsights/rhc/cmd/functional/support"
)

var opts = godog.Options{
	Format: "pretty",
	Paths:  []string{"features"},
}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opts)
}

// registerSteps wires all step definitions into the scenario context.
func registerSteps(sc *godog.ScenarioContext) {
	// --- Given / When — setup and action steps -------------------------------

	// System lifecycle
	sc.Step(`^a connected system$`, statement.Connect)
	sc.Step(`^a disconnected system$`, statement.Disconnect)
	sc.Step(`^a system registered with subscription-manager$`, statement.RegisterRHSM)

	// Command execution
	sc.Step("^I run `([^`]*)`$", statement.RunCommand)
	sc.Step("^I run `([^`]*)` in a temporary directory$", statement.RunCommandInTemporaryDirectory)

	// Systemd
	sc.Step("^I start systemd unit `([^`]*)`$", statement.StartSystemdUnit)

	// --- Then — assertion steps ----------------------------------------------

	// System state
	sc.Step(`^the system is connected$`, statement.Connected)
	sc.Step(`^the system has an identity$`, statement.HasIdentity)
	sc.Step(`^the system has content$`, statement.HasContent)

	// Collector / systemd
	sc.Step(`^all data collectors are disabled$`, statement.AllDataCollectorsDisabled)
	sc.Step("^systemd unit `([^`]*)` is enabled$", statement.SystemdUnitIsEnabled)
	sc.Step("^journal for unit `([^`]*)` contains `([^`]*)`$", statement.JournalContains)

	// Command output
	sc.Step(`^exit code is (\d+)$`, statement.ExitCode)
	sc.Step(`^exit code is not (\d+)$`, statement.ExitCodeNot)
	sc.Step("^stdout contains `([^`]*)`$", statement.StdoutContains)
	sc.Step("^stderr contains `([^`]*)`$", statement.StderrContains)
	sc.Step(`^stdout is valid JSON$`, statement.StdoutIsJSON)
	sc.Step("^stdout JSON field `([^`]*)` is `([^`]*)`$", statement.StdoutJSONField)

	// Filesystem
	sc.Step("^file `([^`]*)` exists$", statement.FileExists)
	sc.Step("^file `([^`]*)` does not exist$", statement.FileNotExists)
	sc.Step("^file `([^`]*)` contains `([^`]*)`$", statement.FileContains)

	// Temporary directory
	sc.Step(`^the temporary directory is not empty$`, statement.TemporaryDirectoryNotEmpty)
}

// addKnownAVCSkips registers skip patterns for AVC denials that are known
// pre-existing issues or environment quirks unrelated to the code under test.
// Add entries here (with a link to the tracking issue) whenever a denial is
// confirmed to be a pre-existing bug rather than a regression introduced by
// rhc changes.
//
// Example:
//
//	c.SkipPattern(`rhsmcertd_t.*openat.*admin_home_t`) // TFT-4293
func addKnownAVCSkips(c *support.AVCChecker) {
	// No known-good patterns yet.
}

func main() {
	flag.Parse()

	suite := godog.TestSuite{
		Name:                 "rhc-functional",
		Options:              &opts,
		TestSuiteInitializer: support.InitializeTestSuite,
		ScenarioInitializer:  support.NewScenarioInitializer(registerSteps, addKnownAVCSkips),
	}

	os.Exit(suite.Run())
}
