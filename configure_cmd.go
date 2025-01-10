package main

import (
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
	"os"
	"os/exec"
	"time"
)

// beforeSatelliteAction is run before satelliteAction is run. It is used
// for checking CLI options.
func beforeSatelliteAction(ctx *cli.Context) error {
	// First check if machine-readable format is used
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	satelliteUrlStr := ctx.String("url")
	if satelliteUrlStr == "" {
		return fmt.Errorf("no url provided using --url CLI option")
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}
	if uuid != "" {
		return fmt.Errorf("cannot configure connected system to use satellite; disconnect system firtst")
	}

	return checkForUnknownArgs(ctx)
}

// satelliteAction tries to get bootstrap script from Satellite server and run it.
// When it is not possible to download the script or running script returns
// non-zero exit code, then error is returned.
//
// It is really risky to download to run some script downloaded from the URL as a
// root user without any restriction. For this reason, we at least check that
// provided URL is URL of Satellite server.
//
// We would like to use different approach in the future. We would like to use
// some API endpoints not restricted by username & password for getting CA certs
// and rendered rhsm.conf, because it would be more secure, but it is not possible ATM
func satelliteAction(ctx *cli.Context) error {
	var configureSatelliteResult ConfigureSatelliteResult
	configureSatelliteResult.format = ctx.String("format")
	satelliteUrlStr := ctx.String("url")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot connect system"
		exitCode := 1
		return cli.Exit(fmt.Errorf("error: %satSpinner", errMsg), exitCode)
	}

	hostname, err := os.Hostname()
	if uiSettings.isMachineReadable {
		configureSatelliteResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if uiSettings.isMachineReadable {
			configureSatelliteResult.HostnameError = err.Error()
			return cli.Exit(configureSatelliteResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("could not acquire hostname: %w", err), exitCode)
		}
	}

	satelliteUrl, err := normalizeSatelliteScriptUrl(satelliteUrlStr)
	if err != nil {
		return cli.Exit(fmt.Errorf("could not parse satellite url: %w", err), 1)
	}

	configureSatelliteResult.SatelliteServerHostname = satelliteUrl.Hostname()
	configureSatelliteResult.SatelliteServerScriptUrl = satelliteUrl.String()

	var satSpinner *spinner.Spinner = nil
	if uiSettings.isRich {
		satSpinner = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		satSpinner.Suffix = fmt.Sprintf(" Configuring '%v' to use Satellite %v", hostname, satelliteUrl.Host)
		satSpinner.Start()
		// Stop spinner after running function
		defer func() { satSpinner.Stop() }()
	}

	if satSpinner != nil {
		satSpinner.Suffix = fmt.Sprintf(" Connecting to Satellite server: %v", satelliteUrl.Host)
	}

	satClient := NewSatelliteClient(satelliteUrl)
	_, err = satClient.Ping()
	if err != nil {
		return cli.Exit(fmt.Errorf("unable to verify that given server is Satellite server: %v", err), 1)
	}

	configureSatelliteResult.IsServerSatellite = true

	if satSpinner != nil {
		satSpinner.Suffix = fmt.Sprintf(" Downloading configuration from %v", satelliteUrl.Host)
	}
	satelliteScriptPath, err := satClient.downloadScript(ctx)
	if err != nil {
		return cli.Exit(fmt.Errorf("could not download satellite bootstrap script: %w", err), 1)
	}

	if ctx.Bool("keep-artifacts") != true {
		defer func() {
			// TODO: If error happens, then log this error
			_ = os.Remove(*satelliteScriptPath)
		}()
	}

	if satSpinner != nil {
		satSpinner.Suffix = fmt.Sprintf(
			" Configuring '%v' to use Satellite server: %v",
			hostname,
			satelliteUrl.Host,
		)
	}

	// Run the bash script. It should install CA certificate, change configuration of rhsm.conf.
	// In theory, it can do almost anything.
	cmd := exec.Command("/usr/bin/bash", *satelliteScriptPath)
	err = cmd.Run()
	if err != nil {
		return cli.Exit(fmt.Errorf("execution of %v script failed: %w", satelliteScriptPath, err), 1)
	}

	configureSatelliteResult.HostConfigured = true

	if uiSettings.isRich {
		satSpinner.Suffix = ""
		satSpinner.Stop()
	}

	interactivePrintf("Host '%v' configured to use Satellite server: %v\n", hostname, satelliteUrl.Host)

	return cli.Exit(configureSatelliteResult, 0)
}
