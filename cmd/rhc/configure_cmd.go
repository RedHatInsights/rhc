package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/features"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/urfave/cli/v2"
)

// beforeEnableFeaturesAction is called before enableFeaturesAction
func beforeEnableFeaturesAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc configure features enable' started")

	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	if ctx.Args().Len() == 0 {
		err = fmt.Errorf("error: required argument 'FEATURE' is missing")
		return cli.Exit(err, ExitCodeDataErr)
	}

	return nil
}

// enableFeaturesAction tries to enables features
func enableFeaturesAction(ctx *cli.Context) error {
	args := ctx.Args()
	enabledFeatures := args.Slice()

	for _, featureId := range enabledFeatures {
		if _, ok := features.KnownFeatures[featureId]; !ok {
			knownFeatIds := strings.Join(features.ListKnownFeatureIds(), ", ")
			slog.Error(fmt.Sprintf("Unknown feature ID: '%s' (allowed: %s)",
				featureId, knownFeatIds))
			return cli.Exit(
				fmt.Errorf("unknown feature ID: '%s' (allowed: %s)",
					featureId, knownFeatIds),
				ExitCodeDataErr)
		}
	}

	if !rhsm.IsRegistered() {
		var err error
		// First, check that enabled features are not in conflict with the configuration file
		enabledFeatures, _, err = features.ConsolidateSelectedFeatures(
			&conf.ConnectFeaturesPreferences,
			enabledFeatures,
			[]string{},
		)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to consolidate selected features: %s", err.Error()))
			return cli.Exit(err, ExitCodeDataErr)
		}
		slog.Debug(fmt.Sprintf("Features enabled using configuration file & CLI: %s", enabledFeatures))
	}

	// First, mark all features as disabled
	for _, feature := range features.KnownFeatures {
		feature.SetWantEnabled(false)
	}
	// Then mark wanted features as enabled
	for _, featureId := range enabledFeatures {
		feature := features.KnownFeatures[featureId]
		feature.SetWantEnabled(true)
	}

	// Then call Enable() on the features marked as enabled. Dependencies are automatically handled
	// in the Enable() function.
	var featuresResults features.FeaturesResults
	for _, feature := range features.KnownFeatures {
		if feature.WantEnabled() {
			if feature.IsEnabled() {
				slog.Debug(fmt.Sprintf("Feature '%s' is already enabled", feature.ID()))
				ui.Printf("%s[%v] %s ... already enabled\n", ui.Indent.Medium, ui.Icons.Ok, feature.Name())
				continue
			}
			slog.Debug(fmt.Sprintf("Feature '%s' will be enabled", feature.ID()))
			err := feature.Enable(&featuresResults)
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to enable feature '%s': %s", feature.ID(), err.Error()))
			}
		}
	}

	// When the system is not registered, then save the feature preferences at the end
	if !rhsm.IsRegistered() {
		err := features.SaveFeaturePreferencesToFile(
			features.RhcConnectFeaturesPreferencesPath,
			conf.ConnectFeaturesPreferences,
		)
		if err != nil {
			return fmt.Errorf("failed to save feature preferences: %w", err)
		}
		slog.Debug(
			fmt.Sprintf("Feature preferences saved in %s",
				features.RhcConnectFeaturesPreferencesPath),
		)
	}

	if ui.IsOutputMachineReadable() {
		content, err := json.MarshalIndent(featuresResults, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to marshal features results: %w", err)
		}
		fmt.Println(string(content))
	}

	return nil
}

func beforeDisableFeaturesAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc configure features disable' started")

	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	if ctx.Args().Len() == 0 {
		err = fmt.Errorf("error: required argument 'FEATURE' is missing")
		return cli.Exit(err, ExitCodeDataErr)
	}

	return nil
}

// disableFeaturesAction disables features
func disableFeaturesAction(ctx *cli.Context) error {
	args := ctx.Args()
	disabledFeatures := args.Slice()

	for _, featureId := range disabledFeatures {
		if _, ok := features.KnownFeatures[featureId]; !ok {
			slog.Error(fmt.Sprintf("Unknown feature ID: '%s'", featureId))
			return cli.Exit(fmt.Errorf("unknown feature ID: '%s'", featureId), ExitCodeDataErr)
		}
	}

	if !rhsm.IsRegistered() {
		var err error
		// First, check that enabled features are not in conflict with the configuration file
		_, disabledFeatures, err = features.ConsolidateSelectedFeatures(
			&conf.ConnectFeaturesPreferences,
			[]string{},
			disabledFeatures,
		)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to consolidate selected features: %s", err.Error()))
			return cli.Exit(err, ExitCodeDataErr)
		}
		slog.Debug(fmt.Sprintf("Features disabled using configuration file & CLI: %s", disabledFeatures))
	}

	// Then mark wanted features as disabled
	for _, featureId := range disabledFeatures {
		feature := features.KnownFeatures[featureId]
		feature.SetWantEnabled(false)
	}

	// Then call Disable() on the features marked as disabled. Dependencies are automatically handled
	var featuresResults features.FeaturesResults
	for _, featureId := range disabledFeatures {
		feature := features.KnownFeatures[featureId]
		if !feature.IsEnabled() {
			slog.Debug(fmt.Sprintf("Feature '%s' is already disabled", featureId))
			continue
		}
		slog.Debug(fmt.Sprintf("Feature '%s' will be disabled", featureId))
		err := feature.Disable(&featuresResults)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to disable feature '%s': %s", featureId, err.Error()))
		}
	}

	if !rhsm.IsRegistered() {
		err := features.SaveFeaturePreferencesToFile(
			features.RhcConnectFeaturesPreferencesPath,
			conf.ConnectFeaturesPreferences,
		)
		if err != nil {
			return fmt.Errorf("failed to save feature preferences: %w", err)
		}
		slog.Debug(
			fmt.Sprintf("Feature preferences saved in %s",
				features.RhcConnectFeaturesPreferencesPath),
		)
	}

	if ui.IsOutputMachineReadable() {
		content, err := json.MarshalIndent(featuresResults, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to marshal features results: %w", err)
		}
		fmt.Println(string(content))
	}

	return nil
}

// beforeStatusFeaturesAction is called before statusFeaturesAction
func beforeStatusFeaturesAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc configure features status' started")

	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	return checkForUnknownArgs(ctx)
}

// statusFeaturesAction shows features
func statusFeaturesAction(ctx *cli.Context) error {
	if !ui.IsOutputMachineReadable() {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = " "
		s.Start()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		// Note: If you want to use some color in some other column, then the header has to be encapsulated
		// with ui.DefaultText and ui.ColorReset.
		if rhsm.IsRegistered() {
			if ui.IsOutputColored() {
				_, _ = fmt.Fprintf(w, "FEATURE\t%sSTATE%s\tDESCRIPTION\n", ui.DefaultText, ui.ColorReset)
			} else {
				_, _ = fmt.Fprintf(w, "FEATURE\tSTATE\tDESCRIPTION\n")
			}
		} else {
			if ui.IsOutputColored() {
				_, _ = fmt.Fprintf(w, "FEATURE\t%sPREFERENCE%s\tDESCRIPTION\n", ui.DefaultText, ui.ColorReset)
			} else {
				_, _ = fmt.Fprintf(w, "FEATURE\tPREFERENCE\tDESCRIPTION\n")
			}
		}

		for _, feature := range features.KnownFeaturesList {
			s.Suffix = fmt.Sprintf(" Checking status of '%s' feature...", feature.ID())
			if feature.IsEnabled() {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", feature.ID(), ui.Icons.Enabled, feature.Description())
			} else {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", feature.ID(), ui.Icons.Disabled, feature.Description())
			}
		}
		s.Stop()
		_ = w.Flush()
	} else {
		if rhsm.IsRegistered() {
			var featPrefs conf.ConnectFeaturesPrefs
			// When the system is registered, then show the actual state of the features
			for _, feature := range features.KnownFeaturesList {
				enabled := feature.IsEnabled()
				switch feature.ID() {
				case features.ContentFeatureID:
					featPrefs.Content = &enabled
				case features.AnalyticsFeatureID:
					featPrefs.Analytics = &enabled
				case features.ManagementFeatureID:
					featPrefs.RemoteManagement = &enabled
				default:
					continue
				}
			}
			content, err := json.MarshalIndent(featPrefs, "", "    ")
			if err != nil {
				return fmt.Errorf("failed to marshal features preferences: %w", err)
			}
			fmt.Println(string(content))
		} else {
			// When the system is not registered, then show the preferences from the configuration file
			content, err := json.MarshalIndent(conf.ConnectFeaturesPreferences, "", "    ")
			if err != nil {
				return fmt.Errorf("failed to marshal features preferences: %w", err)
			}
			fmt.Println(string(content))
		}
	}
	return nil
}
