package main

import (
	"context"
	"os"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
)

func TestIsEnvironmentVariableEnabled(t *testing.T) {
	const environmentVariable = "RHC_TEST_BOOLEAN"
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "one", value: "1", want: true},
		{name: "lowercase t", value: "t", want: true},
		{name: "uppercase T", value: "T", want: true},
		{name: "lowercase true", value: "true", want: true},
		{name: "title case true", value: "True", want: true},
		{name: "uppercase true", value: "TRUE", want: true},
		{name: "zero", value: "0", want: false},
		{name: "false", value: "false", want: false},
		{name: "two", value: "2", want: false},
		{name: "three", value: "3", want: false},
		{name: "invalid", value: "invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(environmentVariable, tt.value)

			if got := isEnvironmentVariableEnabled(environmentVariable); got != tt.want {
				t.Errorf("isEnvironmentVariableEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsForceColorEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "two", value: "2", want: true},
		{name: "three", value: "3", want: true},
		{name: "larger positive integer", value: "42", want: true},
		{name: "zero", value: "0", want: false},
		{name: "negative integer", value: "-1", want: false},
		{name: "false", value: "false", want: false},
		{name: "non-integer", value: "1.5", want: false},
		{name: "invalid", value: "invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envForceColor, tt.value)

			if got := isForceColorEnabled(); got != tt.want {
				t.Errorf("isForceColorEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestConfigureUIForNonInteractiveOutput(t *testing.T) {
	originalStdout := os.Stdout
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = pipeWriter
	t.Cleanup(func() {
		os.Stdout = originalStdout
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		ui.ConfigureOutput(true, true, false)
	})

	if ui.IsInteractive() {
		t.Fatal("ui.IsInteractive() = true for pipe output, want false")
	}

	tests := []struct {
		name       string
		forceColor string
		wantIcon   string
	}{
		{
			name:     "colors disabled by default",
			wantIcon: "✓",
		},
		{
			name:       "colors forced with positive integer",
			forceColor: "2",
			wantIcon:   "\u001B[32m✓\u001B[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envForceColor, tt.forceColor)
			t.Setenv(envNoColor, "false")

			cmd := &cli.Command{
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "no-color", Sources: cli.EnvVars(envNoColor)},
					&cli.StringFlag{Name: "format"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					configureUI(cmd)
					return nil
				},
			}
			if err := cmd.Run(context.Background(), []string{"rhc"}); err != nil {
				t.Fatalf("Command.Run() error = %v", err)
			}

			if ui.AreAnimationsEnabled() {
				t.Error("AreAnimationsEnabled() = true, want false")
			}
			if got := ui.Icons.Ok; got != tt.wantIcon {
				t.Errorf("Icons.Ok = %q, want %q", got, tt.wantIcon)
			}
			if ui.IsOutputMachineReadable() {
				t.Error("IsOutputMachineReadable() = true, want false")
			}
		})
	}
}

func TestBeforeActionRejectsColorConflicts(t *testing.T) {
	tests := []struct {
		name               string
		arguments          []string
		noColorEnvironment string
	}{
		{
			name:               "NO_COLOR",
			noColorEnvironment: "1",
		},
		{
			name:               "--no-color",
			arguments:          []string{"--no-color"},
			noColorEnvironment: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envForceColor, "2")
			t.Setenv(envNoColor, tt.noColorEnvironment)

			cmd := &cli.Command{
				ExitErrHandler: func(context.Context, *cli.Command, error) {},
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "no-color", Sources: cli.EnvVars(envNoColor)},
				},
				Before: beforeAction,
				Action: func(context.Context, *cli.Command) error {
					t.Fatal("Action() called after conflicting color options")
					return nil
				},
			}
			err := cmd.Run(
				context.Background(),
				append([]string{"rhc"}, tt.arguments...),
			)
			if err == nil {
				t.Fatal("Command.Run() error = nil, want color conflict")
			}
			const wantError = "FORCE_COLOR cannot be used together with NO_COLOR or --no-color"
			if err.Error() != wantError {
				t.Errorf("Command.Run() error = %q, want %q", err, wantError)
			}
			exitErr, ok := err.(cli.ExitCoder)
			if !ok {
				t.Fatalf("Command.Run() error type = %T, want cli.ExitCoder", err)
			}
			if got := exitErr.ExitCode(); got != exitcode.Usage {
				t.Errorf("Command.Run() exit code = %d, want %d", got, exitcode.Usage)
			}
		})
	}
}
