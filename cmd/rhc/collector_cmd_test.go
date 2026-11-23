package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/systemd"
	"github.com/redhatinsights/rhc/pkg/exitcode"
)

func TestCollectorTimerCommandsRejectInvalidID(t *testing.T) {
	if !systemd.IsSystemdAvailable() {
		t.Skip("collector timer commands require systemd")
	}

	id := "invalid-collector-id-" + rand.Text()
	wantError := fmt.Sprintf("collector %s: invalid collector ID %q", id, id)
	tests := []struct {
		name   string
		action cli.ActionFunc
	}{
		{name: "enable", action: collectorEnableAction},
		{name: "disable", action: collectorDisableAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cli.Command{
				Name:           tt.name,
				Action:         tt.action,
				ExitErrHandler: func(context.Context, *cli.Command, error) {},
			}
			err := cmd.Run(context.Background(), []string{tt.name, id})
			exitErr, ok := err.(cli.ExitCoder)
			if !ok {
				t.Fatalf("Command.Run() error type = %T, want cli.ExitCoder", err)
			}
			if got := exitErr.Error(); got != wantError {
				t.Errorf("Command.Run() error = %q, want %q", got, wantError)
			}
			if got := exitErr.ExitCode(); got != exitcode.Err {
				t.Errorf("Command.Run() exit code = %d, want %d", got, exitcode.Err)
			}
		})
	}
}
