package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/redhatinsights/rhc/internal/features"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// readOrganization displays the available organizations in a tabular format
// and prompts the user to select one. It returns the organization name entered
// by the user.
func readOrganization(availableOrgs []string) string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Available Organizations:")
	writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for i, org := range availableOrgs {
		_, _ = fmt.Fprintf(writer, "%v\t", org)
		if (i+1)%4 == 0 {
			_, _ = fmt.Fprint(writer, "\n")
		}
	}
	_ = writer.Flush()
	fmt.Print("\nOrganization: ")
	_ = scanner.Scan()
	organization := strings.TrimSpace(scanner.Text())
	fmt.Printf("\n")
	return organization
}

// registerRHSM controls the actual registration flow for connecting the system
// to Red Hat Subscription Management. It handles both username/password and
// activation key registration methods. If credentials are not provided via flags,
// it prompts the user interactively. When the user belongs to multiple organizations
// and no organization is specified, it prompts the user to select one from the
// available list.
func registerRHSM(ctx *cli.Context) error {
	username := ctx.String("username")
	password := ctx.String("password")
	organization := ctx.String("organization")
	activationKeys := ctx.StringSlice("activation-key")
	contentTemplates := ctx.StringSlice("content-template")
	enableContent := features.ContentFeature.Enabled

	if len(activationKeys) == 0 {
		if username == "" {
			password = ""
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Username: ")
			_ = scanner.Scan()
			username = strings.TrimSpace(scanner.Text())
		}
		if password == "" {
			fmt.Print("Password: ")
			data, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return fmt.Errorf("unable to read password: %w", err)
			}
			password = string(data)
			fmt.Printf("\n\n")
		}
	}

	var s *spinner.Spinner
	if ui.IsOutputRich() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = ui.Indent.Small + "["
		s.Suffix = "] Connecting to Red Hat Subscription Management..."
		s.Start()
		defer s.Stop()
	}

	registerServer, err := rhsm.NewRegisterServer()
	if err != nil {
		return err
	}

	defer func() {
		if err = registerServer.Close(); err != nil {
			slog.Debug(err.Error())
		}
	}()

	if len(activationKeys) > 0 {
		return registerServer.RegisterWithActivationKeys(
			organization,
			activationKeys,
			contentTemplates,
			enableContent,
		)
	}

	err = registerServer.RegisterWithUsernamePassword(username, password, organization, contentTemplates, enableContent)
	if rhsm.IsOrgNotSpecified(err) {
		/* When organization was not specified using CLI option --organization, and it is
		   required, because user is member of more than one organization, then ask for
		   the organization. */
		var orgs []string
		orgs, err = registerServer.GetOrganizations(username, password)
		if err != nil {
			return err
		}

		// Ask for organization and display hint with list of organizations
		organization = readOrganization(orgs)

		// Try to register once again with given organization
		return registerServer.RegisterWithUsernamePassword(username, password, organization, contentTemplates, enableContent)
	}

	return err
}
