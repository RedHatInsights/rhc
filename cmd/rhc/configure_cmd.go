package main

import (
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

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
	var disabledFeatures []string

	mapFeatures := features.MapKnownFeatureIds()

	for _, featureId := range enabledFeatures {
		if feature, ok := mapFeatures[featureId]; ok {
			if feature.IsEnabledFunc() {
				ui.Printf("Feature '%s' is already enabled\n", featureId)
				continue
			}
			ui.Printf("Enabling '%s'\n", featureId)
		} else {
			slog.Error(fmt.Sprintf("Unknown feature ID: '%s'", featureId))
			return cli.Exit(fmt.Errorf("unknown feature ID: '%s'", featureId), ExitCodeDataErr)
		}
	}

	// First, check that enabled features are not in conflict with the configuration file
	enabledFeatures, disabledFeatures, err := features.ConsolidateSelectedFeatures(
		&conf.ConnectFeaturesPreferences,
		enabledFeatures,
		disabledFeatures,
	)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to consolidate selected features: %s", err.Error()))
		return cli.Exit(err, ExitCodeDataErr)
	}

	slog.Debug(fmt.Sprintf("Features enabled using configuration file & CLI: %s", enabledFeatures))

	// Try to enable features that are disabled, but are required by enabled features
	for _, featureId := range enabledFeatures {
		if feature, ok := mapFeatures[featureId]; ok {
			requiredFeatures := feature.Requires
			for _, requiredFeature := range requiredFeatures {
				for _, disabledFeature := range disabledFeatures {
					if requiredFeature.ID == disabledFeature {
						msg := fmt.Sprintf("Enabling '%s' (required by '%s')", requiredFeature.ID, featureId)
						ui.Printf("%s\n", msg)
						slog.Debug(msg)
						enabledFeatures = append(enabledFeatures, requiredFeature.ID)
						break
					}
				}
			}
		}
	}

	// Try to enable the features
	for _, featureId := range enabledFeatures {
		if feature, ok := mapFeatures[featureId]; ok {
			if feature.IsEnabledFunc() {
				slog.Debug(fmt.Sprintf("Feature '%s' is already enabled", featureId))
				continue
			}
			slog.Debug(fmt.Sprintf("Feature '%s' will be enabled", featureId))
			err = feature.EnableFunc(ctx)
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to enable feature '%s': %s", featureId, err.Error()))
			}
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
	var enabledFeatures []string

	mapFeatures := features.MapKnownFeatureIds()
	for _, featureId := range disabledFeatures {
		if feature, ok := mapFeatures[featureId]; ok {
			if !feature.IsEnabledFunc() {
				ui.Printf("Feature '%s' is already disabled\n", featureId)
				continue
			}
			ui.Printf("Disabling '%s'\n", featureId)
		} else {
			slog.Error(fmt.Sprintf("Unknown feature ID: '%s'", featureId))
			return cli.Exit(fmt.Errorf("unknown feature ID: '%s'", featureId), ExitCodeDataErr)
		}

	}
	slog.Debug(fmt.Sprintf("Features disabled using CLI: %s", disabledFeatures))

	// First, check that enabled features are not in conflict with the configuration file
	enabledFeatures, disabledFeatures, err := features.ConsolidateSelectedFeatures(
		&conf.ConnectFeaturesPreferences,
		enabledFeatures,
		disabledFeatures,
	)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to consolidate selected features: %s", err.Error()))
		return cli.Exit(err, ExitCodeDataErr)
	}

	slog.Debug(fmt.Sprintf("Features disabled using configuration file & CLI: %s", disabledFeatures))

	for _, featureId := range enabledFeatures {
		feature := mapFeatures[featureId]
		requiredFeatures := feature.Requires
		for _, requiredFeature := range requiredFeatures {
			for _, disabledFeature := range disabledFeatures {
				if requiredFeature.ID == disabledFeature {
					msg := fmt.Sprintf("Disabling '%s' (depends on '%s')", featureId, requiredFeature.ID)
					ui.Printf("%s\n", msg)
					slog.Debug(msg)
					disabledFeatures = append(disabledFeatures, featureId)
					break
				}
			}
		}
	}

	for _, featureId := range disabledFeatures {
		feature := mapFeatures[featureId]
		if !feature.IsEnabledFunc() {
			slog.Debug(fmt.Sprintf("Feature '%s' is already disabled", featureId))
			continue
		}
		slog.Debug(fmt.Sprintf("Feature '%s' will be disabled", featureId))
		err = feature.DisableFunc(ctx)
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

	return nil
}

// beforeStatusFeaturesAction is called before statusFeaturesAction
func beforeStatusFeaturesAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc configure features show' started")

	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	return checkForUnknownArgs(ctx)
}

// statusFeaturesAction shows features
func statusFeaturesAction(ctx *cli.Context) error {
	const contentFeatureDescription = "Access to package repositories"
	const analyticsFeatureDescription = "Red Hat Lightspeed data collection"
	const managementFeatureDescription = "Red Hat Lightspeed remote management"

	if !ui.IsOutputMachineReadable() {
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
		// content
		if features.ContentFeature.IsEnabledFunc() {
			_, _ = fmt.Fprintf(w, "content\t%s\t%s\n", ui.Icons.Enabled, contentFeatureDescription)
			slog.Debug("Content is enabled")
		} else {
			_, _ = fmt.Fprintf(w, "content\t%s\t%s\n", ui.Icons.Disabled, contentFeatureDescription)
			slog.Debug("Content is disabled")
		}

		// analytics
		if features.AnalyticsFeature.IsEnabledFunc() {
			_, _ = fmt.Fprintf(w, "analytics\t%s\t%s\n", ui.Icons.Enabled, analyticsFeatureDescription)
			slog.Debug("Analytics is enabled")
		} else {
			_, _ = fmt.Fprintf(w, "analytics\t%s\t%s\n", ui.Icons.Disabled, analyticsFeatureDescription)
			slog.Debug("Analytics is disabled")
		}

		// management
		if features.ManagementFeature.IsEnabledFunc() {
			_, _ = fmt.Fprintf(w, "management\t%s\t%s\n", ui.Icons.Enabled, managementFeatureDescription)
			slog.Debug("Management is enabled")
		} else {
			_, _ = fmt.Fprintf(w, "management\t%s\t%s\n", ui.Icons.Disabled, managementFeatureDescription)
			slog.Debug("Management is disabled")
		}

		_ = w.Flush()
	}

	return nil
}
