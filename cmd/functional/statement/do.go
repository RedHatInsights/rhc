// Package statement contains the Godog step implementations for the rhc
// functional test suite, split by role: do.go (Given/When), assert.go (Then),
// util.go (unexported helpers).
package statement

import (
	"context"
	"fmt"
	"os"

	"github.com/redhatinsights/rhc/cmd/functional/support"
)

// Connect runs `rhc connect` with the credentials from $CONF/credentials.toml.
func Connect(ctx context.Context) (context.Context, error) {
	creds, err := support.LoadCredentials()
	if err != nil {
		return ctx, err
	}
	result, err := support.RunCommand(creds.RHCConnectCommand())
	if err != nil {
		return ctx, fmt.Errorf("rhc connect: %w", err)
	}
	return support.WithResult(ctx, result), nil
}

// Disconnect runs `rhc disconnect` to ensure the system is unregistered.
// A non-zero exit code is tolerated when the system was already disconnected.
func Disconnect(ctx context.Context) (context.Context, error) {
	result, err := support.RunCommand("rhc disconnect")
	if err != nil {
		return ctx, fmt.Errorf("rhc disconnect: %w", err)
	}
	// Verify the final state rather than the exit code: consumer cert must be absent.
	const consumerCert = "/etc/pki/consumer/cert.pem"
	if _, statErr := os.Stat(consumerCert); statErr == nil {
		return ctx, fmt.Errorf(
			"rhc disconnect did not remove %s (exit %d)\n--- stderr ---\n%s",
			consumerCert, result.ExitCode, result.Stderr,
		)
	}
	return support.WithResult(ctx, result), nil
}

// RegisterRHSM runs `subscription-manager register` with the credentials from
// $CONF/credentials.toml, without registering insights-client or enabling
// yggdrasil.  Use this to establish the "RHSM-only" baseline.
func RegisterRHSM(ctx context.Context) (context.Context, error) {
	creds, err := support.LoadCredentials()
	if err != nil {
		return ctx, err
	}
	result, err := support.RunCommand(creds.RHSMRegisterCommand())
	if err != nil {
		return ctx, fmt.Errorf("subscription-manager register: %w", err)
	}
	if result.ExitCode != 0 {
		return ctx, fmt.Errorf(
			"subscription-manager register failed (exit %d)\n--- stderr ---\n%s",
			result.ExitCode, result.Stderr,
		)
	}
	return support.WithResult(ctx, result), nil
}

// RunCommand executes a shell command and stores the result in the context.
func RunCommand(ctx context.Context, command string) (context.Context, error) {
	result, err := support.RunCommand(command)
	if err != nil {
		return ctx, err
	}
	return support.WithResult(ctx, result), nil
}

// RunCommandInTemporaryDirectory creates a temporary directory, runs the given
// command inside it, and stores both the result and directory path in the
// context.  The directory is removed in the scenario After hook.
func RunCommandInTemporaryDirectory(ctx context.Context, command string) (context.Context, error) {
	dir, err := os.MkdirTemp("", "rhc-collector-*")
	if err != nil {
		return ctx, fmt.Errorf("creating temporary directory: %w", err)
	}
	ctx = support.WithWorkdir(ctx, dir)

	result, err := support.RunCommandInDir(command, dir)
	if err != nil {
		return ctx, err
	}
	return support.WithResult(ctx, result), nil
}

// StartSystemdUnit starts the named systemd unit and waits for it to finish.
// For Type=oneshot units, systemctl start blocks until the unit exits, so a
// non-zero exit code means the unit itself failed.
func StartSystemdUnit(ctx context.Context, unit string) (context.Context, error) {
	result, err := support.RunCommand("systemctl start " + unit)
	if err != nil {
		return ctx, fmt.Errorf("starting %s: %w", unit, err)
	}
	if result.ExitCode != 0 {
		return ctx, fmt.Errorf(
			"systemctl start %s failed (exit %d)\n--- stderr ---\n%s",
			unit, result.ExitCode, result.Stderr,
		)
	}
	return support.WithResult(ctx, result), nil
}
