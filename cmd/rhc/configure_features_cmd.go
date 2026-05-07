package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/feature"
	"github.com/redhatinsights/rhc/pkg/feature/prefcache"
)

// TODO Handle machine-readable output by always returning a DTO from business logic;
//  a place here should only act as a presentation layer.

// TODO All methods should return 'cli.ExitCoder' instead of plain 'error'

// TODO Use ui.Icons.Ok when we have UTF-8 capable tabwriter

// beforeFeaturesStatusAction validates inputs before executing the status action.
func beforeFeaturesStatusAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}
	configureUI(cmd)
	return ctx, checkForUnknownArgs(cmd)
}

// featuresStatusAction displays the current status or preferences of all features.
func featuresStatusAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	isRegistered, err := rhsm.IsRHSMRegistered()
	if err != nil {
		return err
	}

	if isRegistered {
		return featuresStatusActionRegistered(ctx, cmd)
	}
	return featuresStatusActionNotRegistered(ctx, cmd)
}

func featuresStatusActionNotRegistered(_ context.Context, _ *cli.Command) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return err
	}
	fmt.Println("Not connected to Red Hat.")
	fmt.Println("")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "FEATURE\tPREFERENCE\tDESCRIPTION\n")
	for _, f := range feature.All() {
		icon := "enable"
		enabled, err := cache.Get(f.ID())
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		if !enabled {
			icon = "skip"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", f.ID(), icon, f.Description())
	}
	_ = w.Flush()
	return nil
}

func featuresStatusActionRegistered(_ context.Context, _ *cli.Command) error {
	fmt.Println("Connected to Red Hat.")
	fmt.Println("")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "FEATURE\tSTATE\tDESCRIPTION\n")
	for _, f := range feature.All() {
		icon := "enabled"
		enabled, err := f.IsEnabled()
		if err != nil {
			icon = "error"
		} else if !enabled {
			icon = "disabled"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", f.ID(), icon, f.Description())
	}
	_ = w.Flush()
	return nil
}

// beforeFeaturesEnableAction validates inputs before executing the enable action.
func beforeFeaturesEnableAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}
	configureUI(cmd)

	if cmd.Args().Len() != 1 {
		return ctx, cli.Exit("this command requires a single FEATURE argument", exitcode.Usage)
	}
	if _, err = feature.Get(cmd.Args().First()); err != nil {
		return ctx, cli.Exit(err.Error(), exitcode.DataErr)
	}
	return ctx, nil
}

// featuresEnableAction enables a single feature.
func featuresEnableAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	isRegistered, err := rhsm.IsRHSMRegistered()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check registration status: %v", err), exitcode.Software)
	}

	requestedFeature := cmd.Args().First()
	if isRegistered {
		return featuresEnableActionRegistered(ctx, cmd, requestedFeature)
	}
	return featuresEnableActionNotRegistered(ctx, cmd, requestedFeature)
}

// featuresEnableActionNotRegistered handles enabling a feature on a non-registered system.
func featuresEnableActionNotRegistered(_ context.Context, _ *cli.Command, targetName string) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load feature preferences: %v", err), exitcode.Software)
	}

	target := feature.MustGet(targetName)

	// enable required features
	for _, requiredName := range target.Requires() {
		enabled, err := cache.Get(requiredName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		if !enabled {
			fmt.Printf("During registration, '%s' will be enabled (required by '%s').\n", requiredName, targetName)
			if err = cache.Set(requiredName, true); err != nil {
				return cli.Exit(fmt.Sprintf("failed to update preference: %v", err), exitcode.Software)
			}
			slog.Debug("enabling feature", "name", requiredName)
		}
	}
	// enable target feature
	{
		enabled, err := cache.Get(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		if !enabled {
			fmt.Printf("During registration, '%s' will be enabled.\n", targetName)
			if err = cache.Set(targetName, true); err != nil {
				return cli.Exit(fmt.Sprintf("failed to update preference: %v", err), exitcode.Software)
			}
			slog.Debug("enabling feature", "name", targetName)
		}
	}

	if err = cache.Save(); err != nil {
		return cli.Exit(fmt.Sprintf("failed to save feature preferences: %v", err), exitcode.Software)
	}
	return nil
}

// featuresEnableActionRegistered handles enabling a feature on a registered system.
func featuresEnableActionRegistered(_ context.Context, _ *cli.Command, targetName string) error {
	target := feature.MustGet(targetName)

	// enable required features
	for _, requiredName := range target.Requires() {
		required := feature.MustGet(requiredName)
		requiredEnabled, err := required.IsEnabled()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to check status of required feature '%s': %v", requiredName, err), exitcode.Software)
		}
		if requiredEnabled {
			slog.Debug("feature is already enabled", "feature", requiredName)
			continue
		}
		if err = required.Enable(); err != nil {
			return cli.Exit(fmt.Sprintf("failed to enable required feature '%s': %v", requiredName, err), exitcode.Software)
		}
		fmt.Printf("Feature '%s' enabled (required by '%s').\n", requiredName, targetName)
	}
	// enable target feature
	{
		featureEnabled, err := target.IsEnabled()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to check status of target feature '%s': %v", targetName, err), exitcode.Software)
		}
		if featureEnabled {
			slog.Debug("feature is already enabled", "feature", targetName)
			return nil
		}
		if err = target.Enable(); err != nil {
			return cli.Exit(fmt.Sprintf("failed to enable target feature '%s': %v", targetName, err), exitcode.Software)
		}
		fmt.Printf("Feature '%s' enabled.\n", targetName)
	}

	return nil
}

// beforeFeaturesDisableAction validates inputs before executing the disable action.
func beforeFeaturesDisableAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}
	configureUI(cmd)

	if cmd.Args().Len() != 1 {
		return ctx, cli.Exit("this command requires a single FEATURE argument", exitcode.Usage)
	}
	if _, err = feature.Get(cmd.Args().First()); err != nil {
		return ctx, cli.Exit(err.Error(), exitcode.DataErr)
	}
	return ctx, nil
}

// featuresDisableAction disables a single feature.
func featuresDisableAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	isRegistered, err := rhsm.IsRHSMRegistered()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check registration status: %v", err), exitcode.Software)
	}

	requestedFeature := cmd.Args().First()
	if isRegistered {
		return featuresDisableActionRegistered(ctx, cmd, requestedFeature)
	}
	return featuresDisableActionNotRegistered(ctx, cmd, requestedFeature)
}

// featuresDisableActionNotRegistered handles disabling a feature on a non-registered system.
func featuresDisableActionNotRegistered(_ context.Context, _ *cli.Command, targetName string) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load feature preferences: %v", err), exitcode.Software)
	}

	target := feature.MustGet(targetName)

	// disable dependent features
	for _, dependentName := range target.RequiredBy() {
		enabled, err := cache.Get(dependentName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		if enabled {
			fmt.Printf("During registration, '%s' will not be enabled (depends on '%s').\n", dependentName, targetName)
			if err = cache.Set(dependentName, false); err != nil {
				return cli.Exit(fmt.Sprintf("failed to update preference: %v", err), exitcode.Software)
			}
			slog.Debug("disabling feature", "name", dependentName)
		}
	}
	// disable target feature
	{
		enabled, err := cache.Get(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		if enabled {
			fmt.Printf("During registration, '%s' will not be enabled.\n", targetName)
			if err = cache.Set(targetName, false); err != nil {
				return cli.Exit(fmt.Sprintf("failed to update preference: %v", err), exitcode.Software)
			}
			slog.Debug("disabling feature", "name", targetName)
		}
	}

	if err = cache.Save(); err != nil {
		return cli.Exit(fmt.Sprintf("failed to save feature preferences: %v", err), exitcode.Software)
	}
	return nil
}

// featuresDisableActionRegistered handles disabling a feature on a registered system.
func featuresDisableActionRegistered(_ context.Context, _ *cli.Command, targetName string) error {
	target := feature.MustGet(targetName)

	// disable dependent features
	for _, dependentName := range target.RequiredBy() {
		dependent := feature.MustGet(dependentName)
		dependentEnabled, err := dependent.IsEnabled()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to check status of dependent feature '%s': %v", dependentName, err), exitcode.Software)
		}
		if !dependentEnabled {
			slog.Debug("feature is already disabled", "feature", dependentName)
			continue
		}
		if err = dependent.Disable(); err != nil {
			return cli.Exit(fmt.Sprintf("failed to disable dependent feature '%s': %v", dependentName, err), exitcode.Software)
		}
		fmt.Printf("Feature '%s' disabled (depends on '%s').\n", dependentName, targetName)
	}
	// disable target feature
	{
		featureEnabled, err := target.IsEnabled()
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to check status of target feature '%s': %v", targetName, err), exitcode.Software)
		}
		if !featureEnabled {
			slog.Debug("feature is already disabled", "feature", targetName)
			return nil
		}
		if err = target.Disable(); err != nil {
			return cli.Exit(fmt.Sprintf("failed to disable target feature '%s': %v", targetName, err), exitcode.Software)
		}
		fmt.Printf("Feature '%s' disabled.\n", targetName)
	}

	return nil
}
