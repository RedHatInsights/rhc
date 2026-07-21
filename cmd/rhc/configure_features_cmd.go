package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/feature/prefcache"
	"github.com/redhatinsights/rhc/pkg/operations"
)

type ConfigureFeatureStatus struct {
	Preference  string `json:"preference,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Description string `json:"description,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ConfigureFeaturesStatus struct {
	Connected bool `json:"connected"`
	Features  struct {
		Content          ConfigureFeatureStatus `json:"content"`
		Analytics        ConfigureFeatureStatus `json:"analytics"`
		RemoteManagement ConfigureFeatureStatus `json:"remote_management"`
	} `json:"features"`
	returnCode int
}

func (status *ConfigureFeaturesStatus) setFeatureResult(featureID string, result ConfigureFeatureStatus) {
	switch featureID {
	case "content":
		status.Features.Content = result
	case "analytics":
		status.Features.Analytics = result
	case "remote-management":
		status.Features.RemoteManagement = result
	default:
		slog.Warn("unknown feature id for configure features status", "id", featureID)
	}
}

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
	isRegistered, err := operations.IsRegistered()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check registration status: %v", err), exitcode.Software)
	}

	if isRegistered {
		return featuresStatusActionRegistered(ctx, cmd)
	}
	return featuresStatusActionNotRegistered(ctx, cmd)
}

func printConfigureFeaturesStatus(
	cmd *cli.Command,
	status *ConfigureFeaturesStatus,
	headers []string,
	rows [][]string,
	connected bool,
) error {
	if ui.IsOutputMachineReadable() {
		if err := ui.PrintJSON(status); err != nil {
			return cli.Exit(
				fmt.Errorf("unable to print status as %s document: %s", cmd.String("format"), err.Error()),
				exitcode.IOErr,
			)
		}
		if status.returnCode != 0 {
			return cli.Exit("", exitcode.Err)
		}
		return nil
	}
	if connected {
		fmt.Println("Connected to Red Hat.")
	} else {
		fmt.Println("Not connected to Red Hat.")
	}
	fmt.Println("")
	ui.PrintTable(headers, rows)
	return nil
}

func featuresStatusActionNotRegistered(_ context.Context, cmd *cli.Command) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return err
	}

	var status = ConfigureFeaturesStatus{Connected: false}
	headers := []string{"FEATURE", "PREFERENCE", "DESCRIPTION"}
	rows := [][]string{}
	for _, f := range operations.AllFeatures() {
		featureID := f.String()
		enabled, err := cache.Get(featureID)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature preference: %v", err), exitcode.Software)
		}
		pref := "enable"
		if !enabled {
			pref = "skip"
		}
		status.setFeatureResult(
			featureID,
			ConfigureFeatureStatus{
				Preference:  pref,
				Description: f.Description(),
			},
		)
		if !ui.IsOutputMachineReadable() {
			rows = append(rows, []string{featureID, pref, f.Description()})
		}
	}

	return printConfigureFeaturesStatus(cmd, &status, headers, rows, false)
}

func featuresStatusActionRegistered(_ context.Context, cmd *cli.Command) (err error) {
	var status = ConfigureFeaturesStatus{Connected: true}
	headers := []string{"FEATURE", "STATE", "DESCRIPTION"}
	rows := [][]string{}
	for _, f := range operations.AllFeatures() {
		featureID := f.String()
		statusResult := operations.FeatureStatus(operations.FeatureOperationOptions{Feature: f})
		if statusResult.Err != nil {
			return cli.Exit(fmt.Sprintf("failed to get feature status: %v", statusResult.Err), exitcode.Software)
		}
		state := "enabled"
		if !statusResult.Enabled {
			state = "disabled"
		}
		status.setFeatureResult(
			featureID,
			ConfigureFeatureStatus{
				Enabled:     &statusResult.Enabled,
				Description: f.Description(),
			},
		)
		if !ui.IsOutputMachineReadable() {
			rows = append(rows, []string{featureID, state, f.Description()})
		}
	}

	return printConfigureFeaturesStatus(cmd, &status, headers, rows, true)
}

// beforeFeaturesEnableAction validates inputs before executing the enable action.
func beforeFeaturesEnableAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}
	configureUI(cmd)

	numFeatures := len(operations.AllFeatures())
	if cmd.Args().Len() == 0 || cmd.Args().Len() > numFeatures {
		return ctx, cli.Exit(
			fmt.Sprintf("this command requires 1 to %d FEATURE arguments", numFeatures),
			exitcode.Usage,
		)
	}
	featureNames := cmd.Args().Slice()
	for _, featureName := range featureNames {
		if _, err = operations.ParseFeature(featureName); err != nil {
			return ctx, cli.Exit(err.Error(), exitcode.DataErr)
		}
	}
	return ctx, nil
}

// featuresEnableAction enables one or more features.
func featuresEnableAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	isRegistered, err := operations.IsRegistered()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check registration status: %v", err), exitcode.Software)
	}

	requestedFeatures := cmd.Args().Slice()
	if isRegistered {
		return featuresEnableActionRegistered(ctx, cmd, requestedFeatures)
	}
	return featuresEnableActionNotRegistered(ctx, cmd, requestedFeatures)
}

// featuresEnableActionNotRegistered handles enabling a feature on a non-registered system.
func featuresEnableActionNotRegistered(_ context.Context, _ *cli.Command, targetNames []string) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load feature preferences: %v", err), exitcode.Software)
	}

	for _, targetName := range targetNames {
		target, err := operations.ParseFeature(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to parse feature: %v", err), exitcode.Software)
		}
		// enable required features
		for _, required := range target.Requires() {
			requiredName := required.String()
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
		// enable target features
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
	}

	if err = cache.Save(); err != nil {
		return cli.Exit(fmt.Sprintf("failed to save feature preferences: %v", err), exitcode.Software)
	}
	return nil
}

// featuresEnableActionRegistered handles enabling a feature on a registered system.
func featuresEnableActionRegistered(_ context.Context, _ *cli.Command, targetNames []string) error {
	for _, targetName := range targetNames {
		target, err := operations.ParseFeature(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to parse feature: %v", err), exitcode.Software)
		}
		result := operations.EnableFeature(operations.FeatureOperationOptions{Feature: target})

		for _, dep := range result.DependenciesEnabled {
			switch dep.Status {
			case operations.EnableStatusEnabled:
				fmt.Printf("Feature '%s' enabled (required by '%s').\n", dep.Feature.String(), targetName)
			case operations.EnableStatusAlreadyEnabled:
				slog.Debug("feature is already enabled", "feature", dep.Feature.String())
			}
		}

		if result.Err != nil {
			return cli.Exit(
				fmt.Sprintf("failed to enable target feature '%s': %v", targetName, result.Err),
				exitcode.Software,
			)
		}

		switch result.Status {
		case operations.EnableStatusEnabled:
			fmt.Printf("Feature '%s' enabled.\n", targetName)
		case operations.EnableStatusAlreadyEnabled:
			slog.Debug("feature is already enabled", "feature", targetName)
		}
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

	numFeatures := len(operations.AllFeatures())
	if cmd.Args().Len() == 0 || cmd.Args().Len() > numFeatures {
		return ctx, cli.Exit(
			fmt.Sprintf("this command requires 1 to %d FEATURE arguments", numFeatures),
			exitcode.Usage,
		)
	}
	featureNames := cmd.Args().Slice()
	for _, featureName := range featureNames {
		if _, err = operations.ParseFeature(featureName); err != nil {
			return ctx, cli.Exit(err.Error(), exitcode.DataErr)
		}
	}
	return ctx, nil
}

// featuresDisableAction disables one or more features.
func featuresDisableAction(ctx context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)
	isRegistered, err := operations.IsRegistered()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check registration status: %v", err), exitcode.Software)
	}

	requestedFeatures := cmd.Args().Slice()
	if isRegistered {
		return featuresDisableActionRegistered(ctx, cmd, requestedFeatures)
	}
	return featuresDisableActionNotRegistered(ctx, cmd, requestedFeatures)
}

// featuresDisableActionNotRegistered handles disabling a feature on a non-registered system.
func featuresDisableActionNotRegistered(_ context.Context, _ *cli.Command, targetNames []string) error {
	cache, err := prefcache.LoadCache(ConnectFeaturesPrefsPath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load feature preferences: %v", err), exitcode.Software)
	}

	for _, targetName := range targetNames {
		target, err := operations.ParseFeature(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to parse feature: %v", err), exitcode.Software)
		}
		// disable dependent features
		for _, dependent := range target.RequiredBy() {
			dependentName := dependent.String()
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
		// disable target features
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
	}

	if err = cache.Save(); err != nil {
		return cli.Exit(fmt.Sprintf("failed to save feature preferences: %v", err), exitcode.Software)
	}
	return nil
}

// featuresDisableActionRegistered handles disabling a feature on a registered system.
func featuresDisableActionRegistered(_ context.Context, _ *cli.Command, targetNames []string) error {
	for _, targetName := range targetNames {
		target, err := operations.ParseFeature(targetName)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to parse feature: %v", err), exitcode.Software)
		}
		result := operations.DisableFeature(operations.FeatureOperationOptions{Feature: target})

		if result.Err != nil {
			return cli.Exit(
				fmt.Sprintf("failed to disable target feature '%s': %v", targetName, result.Err),
				exitcode.Software,
			)
		}

		for _, dep := range result.DependentsDisabled {
			switch dep.Status {
			case operations.DisableStatusDisabled:
				fmt.Printf("Feature '%s' disabled (depends on '%s').\n", dep.Feature.String(), targetName)
			case operations.DisableStatusAlreadyDisabled:
				slog.Debug("feature is already disabled", "feature", dep.Feature.String())
			}
		}

		switch result.Status {
		case operations.DisableStatusDisabled:
			fmt.Printf("Feature '%s' disabled.\n", targetName)
		case operations.DisableStatusAlreadyDisabled:
			slog.Debug("feature is already disabled", "feature", targetName)
		}
	}

	return nil
}
