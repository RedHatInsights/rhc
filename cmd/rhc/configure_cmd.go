package main

import (
	"fmt"
	"log/slog"

	"github.com/redhatinsights/rhc/internal/features"
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

// enableFeaturesAction enables features
func enableFeaturesAction(ctx *cli.Context) error {
	args := ctx.Args()
	for _, arg := range args.Slice() {
		slog.Info(fmt.Sprintf("Enabling feature: %s", arg))
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
	for _, arg := range args.Slice() {
		slog.Info(fmt.Sprintf("Disabling feature: %s", arg))
	}
	return nil
}

// beforeShowFeaturesAction is called before showFeaturesAction
func beforeShowFeaturesAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc configure features show' started")

	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	return checkForUnknownArgs(ctx)
}

// showFeaturesAction shows features
func showFeaturesAction(ctx *cli.Context) error {
	featuresConfig, err := features.GetFeaturesFromFile("/etc/rhc/config.toml.d/01-features.toml")
	if err != nil {
		slog.Warn(fmt.Sprintf("failed to get features from drop-in config file: %s", err.Error()))
		return err
	}
	if featuresConfig != nil {
		if *featuresConfig.Content {
			slog.Info("Content is enabled")
		} else {
			slog.Info("Content is disabled")
		}
		if *featuresConfig.Analytics {
			slog.Info("Analytics is enabled")
		} else {
			slog.Info("Analytics is disabled")
		}
		if *featuresConfig.Management {
			slog.Info("Management is enabled")
		} else {
			slog.Info("Management is disabled")
		}
	}
	return nil
}
