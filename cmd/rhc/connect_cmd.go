package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/feature"
	"github.com/redhatinsights/rhc/pkg/operations"
)

// checkFeatureFlags validates --enable-feature and --disable-feature flag combinations.
// Returns an error if the combination is invalid.
func checkFeatureFlags(toEnable, toDisable []string) error {
	toDisableMap := make(map[string]bool)
	for _, d := range toDisable {
		toDisableMap[d] = true
	}
	toEnableMap := make(map[string]bool)
	for _, e := range toEnable {
		toEnableMap[e] = true
	}

	// Check for feature in both lists
	for _, e := range toEnable {
		if toDisableMap[e] {
			return fmt.Errorf("invalid combination: enable '%s', disable '%s'", e, e)
		}
	}

	// Check for enabling a feature while disabling its dependencies
	for _, e := range toEnable {
		f, err := operations.ParseFeature(e)
		if err != nil {
			return err
		}
		for _, dep := range f.Requires() {
			if toDisableMap[dep.String()] {
				return fmt.Errorf("invalid combination: enable '%s', disable '%s'", e, dep.String())
			}
		}
	}

	// Check for disabling a feature while enabling features that depend on it
	for _, d := range toDisable {
		f, err := operations.ParseFeature(d)
		if err != nil {
			return err
		}
		for _, dependent := range f.RequiredBy() {
			if toEnableMap[dependent.String()] {
				return fmt.Errorf("invalid combination: enable '%s', disable '%s'", dependent.String(), d)
			}
		}
	}

	return nil
}

// beforeConnectAction ensures correct CLI flags have been passed in:
// correct values, no conflicts. On error, this method invokes cli.Exit()
// with appropriate message and error code.
func beforeConnectAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	// Verify --format contains valid value
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}
	// Configure UI globals
	configureUI(cmd)

	// Validate --enable-feature/--disable-feature combinations make sense
	err = checkFeatureFlags(
		cmd.StringSlice("enable-feature"),
		cmd.StringSlice("disable-feature"),
	)
	if err != nil {
		return ctx, cli.Exit(err.Error(), exitcode.Usage)
	}

	// Do not continue if the host is already registered
	slog.Info("Checking system connection status")
	registered, err := operations.IsRegistered()
	if err != nil {
		return ctx, cli.Exit(
			fmt.Sprintf("unable to check connection status: %s", err),
			exitcode.Software,
		)
	}
	if registered {
		slog.Info("System is already connected")
		return ctx, cli.Exit("this system is already connected", exitcode.Usage)
	}

	username := cmd.String("username")
	password := cmd.String("password")
	organization := cmd.String("organization")
	activationKeys := cmd.StringSlice("activation-key")
	contentTemplates := cmd.StringSlice("content-template")

	if len(activationKeys) > 0 {
		if username != "" {
			exitErr := cli.Exit(
				"--username and --activation-key can not be used together",
				exitcode.Usage,
			)
			return ctx, exitErr

		}
		if password != "" {
			exitErr := cli.Exit(
				"--password and --activation-key can not be used together",
				exitcode.Usage,
			)
			return ctx, exitErr

		}
		if organization == "" {
			exitErr := cli.Exit(
				"--organization is required, when --activation-key is used",
				exitcode.Usage,
			)
			return ctx, exitErr
		}
	}

	// Exit if username/password or activation key/organization haven't been provided,
	// and we cannot ask interactively.
	if !isInteractive() {
		if (username == "" || password == "") && (len(activationKeys) == 0 || organization == "") {
			exitErr := cli.Exit(
				"--username/--password or --organization/--activation-key are required when a machine-readable format is used",
				exitcode.Usage,
			)
			return ctx, exitErr
		}
	}

	// Load preference cache created by 'rhc configure features'.
	// If --enable-feature or --disable-feature flags are provided, ignore the cache file
	// and start with defaults.
	var cache *feature.PreferenceCache
	if len(cmd.StringSlice("enable-feature")) > 0 || len(cmd.StringSlice("disable-feature")) > 0 {
		cache, err = feature.NewDefaultCache(ConnectFeaturesPrefsPath)
		if err != nil {
			return ctx, cli.Exit(fmt.Sprintf("failed to create default cache: %v", err), exitcode.Software)
		}
		for _, name := range cmd.StringSlice("enable-feature") {
			feat, parseErr := operations.ParseFeature(name)
			if parseErr != nil {
				return ctx, cli.Exit(parseErr.Error(), exitcode.DataErr)
			}
			cache.Set(feat, true)
		}
		for _, name := range cmd.StringSlice("disable-feature") {
			feat, parseErr := operations.ParseFeature(name)
			if parseErr != nil {
				return ctx, cli.Exit(parseErr.Error(), exitcode.DataErr)
			}
			cache.Set(feat, false)
		}
		formatConnectIgnoringPrefs()
	} else {
		// No flags provided, load cache from file (or defaults if file doesn't exist)
		cache, err = feature.LoadCache(ConnectFeaturesPrefsPath)
		if err != nil {
			return ctx, cli.Exit(fmt.Sprintf("failed to load preferences: %v", err), exitcode.Software)
		}
	}

	cmd.Root().Metadata[connectCacheKey] = cache

	// Error out if we're trying to set content templates without having enabling content
	if !cache.Get(operations.Content) && len(contentTemplates) > 0 {
		return ctx, cli.Exit("content feature is disabled, cannot use --content-template", exitcode.Usage)
	}

	err = checkForUnknownArgs(cmd)
	if err != nil {
		return ctx, cli.Exit(err.Error(), exitcode.Usage)
	}

	return ctx, nil
}

func setConnectFeatureStatus(report *operations.ConnectReport) {
	for _, item := range []struct {
		feature operations.Feature
		result  *operations.ConnectFeatureResult
	}{
		{operations.Content, &report.Content},
		{operations.Analytics, &report.Analytics},
		{operations.RemoteManagement, &report.RemoteManagement},
	} {
		status := operations.FeatureStatus(
			operations.FeatureOperationOptions{Feature: item.feature},
		)
		if status.Err != nil {
			slog.Debug("failed to get feature status", "feature", item.feature, "error", status.Err)
			item.result.Enabled = item.result.Successful
			continue
		}
		item.result.Enabled = status.Enabled
	}
}

func connectErrorMessages(report operations.ConnectReport) map[string]string {
	errorMessages := make(map[string]string)
	if report.RHSMError != "" {
		errorMessages["rhsm"] = report.RHSMError
	}
	if report.Analytics.Error != "" && !report.Analytics.Skipped {
		errorMessages["insights"] = report.Analytics.Error
	}
	if report.RemoteManagement.Error != "" && !report.RemoteManagement.Skipped {
		errorMessages["yggdrasil"] = report.RemoteManagement.Error
	}
	return errorMessages
}

// connectAction is the CLI handler for 'rhc connect'.
// It checks identity, collects credentials, runs operations.Connect
// with a spinner, handles an organization prompt
// if RHSM requires one, and prints the human readable or JSON result.
func connectAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	cache := cmd.Root().Metadata[connectCacheKey].(*feature.PreferenceCache)

	var report operations.ConnectReport

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot connect system"
		slog.Error(errMsg)
		report.UID = uid
		report.UIDError = errMsg
		return printConnectTextOrJSON(cmd, report, errors.New(errMsg), exitcode.NoPerm)
	}

	hostname, err := os.Hostname()
	if err != nil {
		slog.Error("Error retrieving system hostname", "error", err)
		report.HostnameError = err.Error()
		return printConnectTextOrJSON(cmd, report, err, exitcode.Err)
	}

	var toEnable []string
	if cache.Get(operations.Content) {
		toEnable = append(toEnable, "content")
	}
	if cache.Get(operations.Analytics) {
		toEnable = append(toEnable, "analytics")
	}
	if cache.Get(operations.RemoteManagement) {
		toEnable = append(toEnable, "remote management")
	}

	formatConnectHeader(hostname, toEnable)

	opts, err := buildConnectionOptions(cmd, cache)
	if err != nil {
		return cli.Exit(err, exitcode.Err)
	}

	report, err = connectWithSpinner(opts, hostname, uid)

	if errors.Is(err, operations.ErrOrganizationRequired) && !isConnectFormatMachineReadable(cmd) {
		org, orgErr := promptOrganization(opts.Username, opts.Password)
		if orgErr != nil {
			recordOrganizationFailure(&report, orgErr.Error())
			err = nil
		} else {
			opts.Organization = org
			report, err = connectWithSpinner(opts, hostname, uid)
		}
	}

	if errors.Is(err, operations.ErrOrganizationRequired) {
		recordOrganizationFailure(&report, "no organization specified")
		err = nil
	}

	if err != nil {
		return cli.Exit(err, exitcode.Err)
	}

	formatConnectStepsReport(report)
	if report.RHSMConnected {
		formatConnectSuccess()
	}

	if !isConnectFormatMachineReadable(cmd) {
		formatConnectFooter()
		showTimeDuration(report.Durations)
	}

	if err := showErrorMessages("connect", connectErrorMessages(report)); err != nil {
		return err
	}

	if isConnectFormatMachineReadable(cmd) {
		setConnectFeatureStatus(&report)
		if printErr := formatConnectJSON(report); printErr != nil {
			return cli.Exit(
				fmt.Errorf("unable to print connect result as %s document: %s", cmd.String("format"), printErr.Error()),
				exitcode.IOErr)
		}
	}

	if err := cache.Delete(); err != nil {
		slog.Debug("could not delete preferences cache", "err", err)
	}

	return nil
}

func buildConnectionOptions(cmd *cli.Command, cache *feature.PreferenceCache) (operations.ConnectOptions, error) {
	opts := operations.ConnectOptions{
		Username:               cmd.String("username"),
		Password:               cmd.String("password"),
		Organization:           cmd.String("organization"),
		ActivationKeys:         cmd.StringSlice("activation-key"),
		ContentTemplates:       cmd.StringSlice("content-template"),
		EnableContent:          cache.Get(operations.Content),
		EnableAnalytics:        cache.Get(operations.Analytics),
		EnableRemoteManagement: cache.Get(operations.RemoteManagement),
	}

	if len(opts.ActivationKeys) > 0 {
		return opts, nil
	}

	if opts.Username == "" {
		opts.Password = ""
		fmt.Print("Username: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return opts, fmt.Errorf("unable to read username: %w", err)
			}
			return opts, errors.New("unable to read username: EOF")
		}
		opts.Username = strings.TrimSpace(scanner.Text())
	}
	if opts.Password == "" {
		fmt.Print("Password: ")
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return opts, fmt.Errorf("unable to read password: %w", err)
		}
		opts.Password = string(data)
		fmt.Print("\n\n")
	}
	return opts, nil
}

func connectWithSpinner(
	opts operations.ConnectOptions,
	hostname string,
	uid int,
) (report operations.ConnectReport, err error) {
	err = ui.Spinner(func() error {
		report, err = operations.Connect(opts)
		return err
	}, ui.Indent.Small, "Connecting to Red Hat Subscription Management...")
	report.Hostname = hostname
	report.UID = uid
	return report, err
}

func recordOrganizationFailure(report *operations.ConnectReport, msg string) {
	report.RHSMError = msg
	slog.Error(report.RHSMError)
}

func promptOrganization(username, password string) (string, error) {
	orgs, err := operations.GetOrganizations(username, password)
	if err != nil {
		return "", fmt.Errorf("cannot retrieve organizations: %w", err)
	}

	fmt.Println("Available Organizations:")
	writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for i, org := range orgs {
		_, _ = fmt.Fprintf(writer, "%v\t", org)
		if (i+1)%4 == 0 {
			_, _ = fmt.Fprint(writer, "\n")
		}
	}
	_ = writer.Flush()

	fmt.Print("\nOrganization: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("unable to read organization: %w", err)
		}
		return "", errors.New("unable to read organization: EOF")
	}
	org := strings.TrimSpace(scanner.Text())
	if org == "" {
		return "", errors.New("no organization specified")
	}
	fmt.Print("\n")
	return org, nil
}

func isConnectFormatMachineReadable(cmd *cli.Command) bool {
	return cmd.String("format") != ""
}

func printConnectTextOrJSON(cmd *cli.Command, report operations.ConnectReport, human error, code int) error {
	if !isConnectFormatMachineReadable(cmd) {
		return cli.Exit(human, code)
	}
	if printErr := formatConnectJSON(report); printErr != nil {
		return cli.Exit(
			fmt.Errorf(
				"unable to print connect result as %s document: %s",
				cmd.String("format"), printErr.Error(),
			),
			exitcode.IOErr,
		)
	}
	return cli.Exit("", code)
}
