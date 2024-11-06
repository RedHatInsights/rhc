package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/subpop/go-log"
	"golang.org/x/term"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

type LogMessage struct {
	level   log.Level
	message error
}

const (
	colorGreen  = "\u001B[32m"
	colorYellow = "\u001B[33m"
	colorRed    = "\u001B[31m"
	colorReset  = "\u001B[0m"
)

// userInterfaceSettings manages standard output preference.
// It tracks colors, icons and machine-readable output (e.g. json).
//
// It is instantiated via uiSettings by calling configureUISettings.
type userInterfaceSettings struct {
	// isMachineReadable describes the machine-readable mode (e.g., `--format json`)
	isMachineReadable bool
	// isRich describes the ability to display colors and animations
	isRich    bool
	iconOK    string
	iconInfo  string
	iconError string
}

// uiSettings is an instance that keeps actual data of output preference.
//
// It is managed by calling the configureUISettings method.
var uiSettings = userInterfaceSettings{}

// configureUISettings is called by the CLI library when it loads up.
// It sets up the uiSettings object.
func configureUISettings(ctx *cli.Context) {
	if ctx.Bool("no-color") {
		uiSettings = userInterfaceSettings{
			isRich:            false,
			isMachineReadable: false,
			iconOK:            "✓",
			iconInfo:          "·",
			iconError:         "𐄂",
		}
	} else {
		uiSettings = userInterfaceSettings{
			isRich:            true,
			isMachineReadable: false,
			iconOK:            colorGreen + "●" + colorReset,
			iconInfo:          colorYellow + "●" + colorReset,
			iconError:         colorRed + "●" + colorReset,
		}
	}
}

// showProgress calls function and, when it is possible display spinner with
// some progress message.
func showProgress(
	progressMessage string,
	function func() error,
) error {
	var s *spinner.Spinner
	if uiSettings.isRich {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = progressMessage
		s.Start()
		// Stop spinner after running function
		defer func() { s.Stop() }()
	}
	return function()
}

// showTimeDuration shows table with duration of each sub-action
func showTimeDuration(durations map[string]time.Duration) {
	if log.CurrentLevel() >= log.LevelDebug {
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "STEP\tDURATION\t")
		for step, duration := range durations {
			_, _ = fmt.Fprintf(w, "%v\t%v\t\n", step, duration.Truncate(time.Millisecond))
		}
		_ = w.Flush()
	}
}

// showErrorMessages shows table with all error messages gathered during action
func showErrorMessages(action string, errorMessages map[string]LogMessage) error {
	if hasPriorityErrors(errorMessages, log.CurrentLevel()) {
		if !uiSettings.isMachineReadable {
			fmt.Println()
			fmt.Printf("The following errors were encountered during %s:\n\n", action)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TYPE\tSTEP\tERROR\t")
			for step, logMsg := range errorMessages {
				if logMsg.level <= log.CurrentLevel() {
					_, _ = fmt.Fprintf(w, "%v\t%v\t%v\n", logMsg.level, step, logMsg.message)
				}
			}
			_ = w.Flush()
			if hasPriorityErrors(errorMessages, log.LevelError) {
				return cli.Exit("", 1)
			}
		}
	}
	return nil
}

// registerRHSM tries to register system against Red Hat Subscription Management server (candlepin server)
func registerRHSM(ctx *cli.Context) (string, error) {
	uuid, err := getConsumerUUID()
	if err != nil {
		return "Unable to get consumer UUID", cli.Exit(err, 1)
	}
	var successMsg string

	if uuid == "" {
		username := ctx.String("username")
		password := ctx.String("password")
		organization := ctx.String("organization")
		activationKeys := ctx.StringSlice("activation-key")

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
					return "Unable to read password", cli.Exit(err, 1)
				}
				password = string(data)
				fmt.Printf("\n\n")
			}
		}

		var s *spinner.Spinner
		if uiSettings.isRich {
			s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
			s.Suffix = " Connecting to Red Hat Subscription Management..."
			s.Start()
			defer s.Stop()
		}

		var err error
		if len(activationKeys) > 0 {
			err = registerActivationKey(
				organization,
				ctx.StringSlice("activation-key"),
				ctx.String("server"))
		} else {
			var orgs []string
			if organization != "" {
				_, err = registerUsernamePassword(username, password, organization, ctx.String("server"))
			} else {
				orgs, err = registerUsernamePassword(username, password, "", ctx.String("server"))
				/* When organization was not specified using CLI option --organization, and it is
				   required, because user is member of more than one organization, then ask for
				   the organization. */
				if len(orgs) > 0 {
					if uiSettings.isMachineReadable {
						return "Unable to register system to RHSM", cli.Exit("no organization specified", 1)
					}
					// Stop spinner to be able to display message and ask for organization
					if uiSettings.isRich {
						s.Stop()
					}

					// Ask for organization and display hint with list of organizations
					scanner := bufio.NewScanner(os.Stdin)
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
					_ = scanner.Scan()
					organization = strings.TrimSpace(scanner.Text())
					fmt.Printf("\n")

					// Start spinner again
					if uiSettings.isRich {
						s.Start()
					}

					// Try to register once again with given organization
					_, err = registerUsernamePassword(username, password, organization, ctx.String("server"))
				}
			}
		}
		if err != nil {
			return "Unable to register system to RHSM", cli.Exit(err, 1)
		}
		successMsg = "Connected to Red Hat Subscription Management"
	} else {
		successMsg = "This system is already connected to Red Hat Subscription Management"
	}
	return successMsg, nil
}

// beforeConnectAction ensures that user has supplied a correct CLI options
// and there is no conflict between provided options
func beforeConnectAction(ctx *cli.Context) error {
	// First check if machine-readable format is used
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	if IsConnected() {
		return fmt.Errorf("this system is already connected")
	}

	username := ctx.String("username")
	password := ctx.String("password")
	organization := ctx.String("organization")
	activationKeys := ctx.StringSlice("activation-key")

	if len(activationKeys) > 0 {
		if username != "" {
			return fmt.Errorf("--username and --activation-key can not be used together")
		}
		if organization == "" {
			return fmt.Errorf("--organization is required, when --activation-key is used")
		}
	}

	// When machine-readable format is used, then additional requirements have to be met
	if uiSettings.isMachineReadable {
		if username == "" || password == "" {
			return fmt.Errorf("--username/--password or --organization/--activation-key are required when a machine-readable format is used")
		}
	}

	return checkForUnknownArgs(ctx)
}

// connectAction tries to register system against Red Hat Subscription Management,
// gather the profile information that the system will configure
// connect system to Red Hat Insights, and it also tries to start rhcd service
func connectAction(ctx *cli.Context) error {
	var connectResult ConnectResult
	connectResult.format = ctx.String("format")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot connect system"
		exitCode := 1
		if uiSettings.isMachineReadable {
			connectResult.UID = uid
			connectResult.UIDError = errMsg
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("error: %s", errMsg), exitCode)
		}
	}

	hostname, err := os.Hostname()
	if uiSettings.isMachineReadable {
		connectResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if uiSettings.isMachineReadable {
			connectResult.HostnameError = err.Error()
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(err, exitCode)
		}
	}

	enableFeatureAnalytics := false
	enableFeatureManagement := false
	for _, featureID := range GetFeaturesState().Enabled {
		if featureID == AnalyticsFeature.ID {
			enableFeatureAnalytics = true
		}
		if featureID == ManagementFeature.ID {
			enableFeatureManagement = true
		}
	}

	interactivePrintf("Connecting %v to %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]LogMessage)
	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	var returnedMsg string
	returnedMsg, err = registerRHSM(ctx)
	if err != nil {
		connectResult.RHSMConnected = false
		errorMessages["rhsm"] = LogMessage{
			level: log.LevelError,
			message: fmt.Errorf("cannot connect to Red Hat Subscription Management: %w",
				err)}
		if uiSettings.isMachineReadable {
			connectResult.RHSMConnectError = errorMessages["rhsm"].message.Error()
		} else {
			fmt.Printf(
				"%v Cannot connect to Red Hat Subscription Management\n",
				uiSettings.iconError,
			)
		}
	} else {
		connectResult.RHSMConnected = true
		interactivePrintf("%v %v\n", uiSettings.iconOK, returnedMsg)
	}
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	if errors, exist := errorMessages["rhsm"]; exist {
		if errors.level == log.LevelError {
			interactivePrintf(
				"%v Skipping connection to Red Hat Insights\n",
				uiSettings.iconError,
			)
		}
	} else if !enableFeatureAnalytics {
		interactivePrintf(
			"%v Skipping connection to Red Hat Insights: Analytics feature is disabled\n",
			uiSettings.iconInfo,
		)
	} else {
		start = time.Now()
		err = showProgress(" Connecting to Red Hat Insights...", registerInsights)
		if err != nil {
			connectResult.InsightsConnected = false
			errorMessages["insights"] = LogMessage{
				level:   log.LevelError,
				message: fmt.Errorf("cannot connect to Red Hat Insights: %w", err)}
			if uiSettings.isMachineReadable {
				connectResult.InsightsError = errorMessages["insights"].message.Error()
			} else {
				fmt.Printf("%v Cannot connect to Red Hat Insights\n", uiSettings.iconError)
			}
		} else {
			connectResult.InsightsConnected = true
			interactivePrintf("%v Connected to Red Hat Insights\n", uiSettings.iconOK)
		}
		durations["insights"] = time.Since(start)
	}

	/* 3. Start yggdrasil (rhcd) service */
	if rhsmErrMsg, exist := errorMessages["rhsm"]; exist && rhsmErrMsg.level == log.LevelError {
		connectResult.YggdrasilStarted = false
		interactivePrintf(
			"%v Skipping activation of %v service\n",
			uiSettings.iconError,
			ServiceName,
		)
	} else if !enableFeatureManagement {
		connectResult.YggdrasilStarted = false
		interactivePrintf(
			"%v Skipping activation of %v service: Management feature is disabled\n",
			uiSettings.iconInfo,
			ServiceName,
		)
	} else {
		start = time.Now()
		progressMessage := fmt.Sprintf(" Activating the %v service", ServiceName)
		err = showProgress(progressMessage, activateService)
		if err != nil {
			connectResult.YggdrasilStarted = false
			errorMessages[ServiceName] = LogMessage{
				level: log.LevelError,
				message: fmt.Errorf("cannot activate %s service: %w",
					ServiceName, err)}
			if uiSettings.isMachineReadable {
				connectResult.YggdrasilStartedError = errorMessages[ServiceName].message.Error()
			} else {
				fmt.Printf("%v Cannot activate the %v service\n", uiSettings.iconError, ServiceName)
			}
		} else {
			connectResult.YggdrasilStarted = true
			interactivePrintf("%v Activated the %v service\n", uiSettings.iconOK, ServiceName)
		}
		durations[ServiceName] = time.Since(start)
		interactivePrintf("\nSuccessfully connected to Red Hat!\n")
	}

	if !uiSettings.isMachineReadable {
		/* 5. Show footer message */
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

		/* 6. Optionally display duration time of each sub-action */
		showTimeDuration(durations)
	}

	err = showErrorMessages("connect", errorMessages)
	if err != nil {
		return err
	}

	{ // update cache to say we have successfully connected
		log.Info("system has been connected, updating state file")
		SetConnected(true)
	}
	return cli.Exit(connectResult, 0)
}

// setupFormatOption ensures the user has supplied a correct `--format` flag
// and set values in uiSettings, when JSON format is used.
func setupFormatOption(ctx *cli.Context) error {
	// This is run after the `app.Before()` has been run,
	// the uiSettings is already set up for us to modify.
	format := ctx.String("format")
	switch format {
	case "":
		return nil
	case "json":
		uiSettings.isMachineReadable = true
		uiSettings.isRich = false
		return nil
	default:
		err := fmt.Errorf(
			"unsupported format: %s (supported formats: %s)",
			format,
			`"json"`,
		)
		return cli.Exit(err, 1)
	}
}

// DisconnectResult is structure holding information about result of
// disconnect command. The result could be printed in machine-readable format.
type DisconnectResult struct {
	Hostname                  string `json:"hostname"`
	HostnameError             string `json:"hostname_error,omitempty"`
	UID                       int    `json:"uid"`
	UIDError                  string `json:"uid_error,omitempty"`
	RHSMDisconnected          bool   `json:"rhsm_disconnected"`
	RHSMDisconnectedError     string `json:"rhsm_disconnect_error,omitempty"`
	InsightsDisconnected      bool   `json:"insights_disconnected"`
	InsightsDisconnectedError string `json:"insights_disconnected_error,omitempty"`
	YggdrasilStopped          bool   `json:"yggdrasil_stopped"`
	YggdrasilStoppedError     string `json:"yggdrasil_stopped_error,omitempty"`
	format                    string
}

// Error implement error interface for structure DisconnectResult
func (disconnectResult DisconnectResult) Error() string {
	var result string
	switch disconnectResult.format {
	case "json":
		data, err := json.MarshalIndent(disconnectResult, "", "    ")
		if err != nil {
			return err.Error()
		}
		result = string(data)
	case "":
		break
	default:
		result = "error: unsupported document format: " + disconnectResult.format
	}
	return result
}

// ConnectResult is structure holding information about results
// of connect command. The result could be printed in machine-readable format.
type ConnectResult struct {
	Hostname              string `json:"hostname"`
	HostnameError         string `json:"hostname_error,omitempty"`
	UID                   int    `json:"uid"`
	UIDError              string `json:"uid_error,omitempty"`
	RHSMConnected         bool   `json:"rhsm_connected"`
	RHSMConnectError      string `json:"rhsm_connect_error,omitempty"`
	InsightsConnected     bool   `json:"insights_connected"`
	InsightsError         string `json:"insights_connect_error,omitempty"`
	YggdrasilStarted      bool   `json:"yggdrasil_started"`
	YggdrasilStartedError string `json:"yggdrasil_started_error,omitempty"`
	format                string
}

// Error implement error interface for structure ConnectResult
func (connectResult ConnectResult) Error() string {
	var result string
	switch connectResult.format {
	case "json":
		data, err := json.MarshalIndent(connectResult, "", "    ")
		if err != nil {
			return err.Error()
		}
		result = string(data)
	case "":
		break
	default:
		result = "error: unsupported document format: " + connectResult.format
	}
	return result
}

// beforeDisconnectAction ensures the used has supplied a correct `--format` flag
func beforeDisconnectAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	return checkForUnknownArgs(ctx)
}

// interactivePrintf is method for printing human-readable output. It suppresses output, when
// machine-readable format is used.
func interactivePrintf(format string, a ...interface{}) {
	if !uiSettings.isMachineReadable {
		fmt.Printf(format, a...)
	}
}

// disconnectAction tries to stop (yggdrasil) rhcd service, disconnect from Red Hat Insights,
// and finally it unregisters system from Red Hat Subscription Management
func disconnectAction(ctx *cli.Context) error {
	var disconnectResult DisconnectResult
	disconnectResult.format = ctx.String("format")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot disconnect system"
		exitCode := 1
		if uiSettings.isMachineReadable {
			disconnectResult.UID = uid
			disconnectResult.UIDError = errMsg
			return cli.Exit(disconnectResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("error: %s", errMsg), exitCode)
		}
	}

	hostname, err := os.Hostname()
	if uiSettings.isMachineReadable {
		disconnectResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if uiSettings.isMachineReadable {
			disconnectResult.HostnameError = err.Error()
			return cli.Exit(disconnectResult, exitCode)
		} else {
			return cli.Exit(err, exitCode)
		}
	}

	interactivePrintf("Disconnecting %v from %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]LogMessage)

	/* 1. Deactivate yggdrasil (rhcd) service */
	start = time.Now()
	progressMessage := fmt.Sprintf(" Deactivating the %v service", ServiceName)
	err = showProgress(progressMessage, deactivateService)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot deactivate %s service: %v", ServiceName, err)
		errorMessages[ServiceName] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.YggdrasilStopped = false
		disconnectResult.YggdrasilStoppedError = errMsg
		interactivePrintf("%v %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.YggdrasilStopped = true
		interactivePrintf("%v Deactivated the %v service\n", uiSettings.iconOK, ServiceName)
	}
	durations[ServiceName] = time.Since(start)

	/* 2. Disconnect from Red Hat Insights */
	start = time.Now()
	err = showProgress(" Disconnecting from Red Hat Insights...", unregisterInsights)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Insights: %v", err)
		errorMessages["insights"] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.InsightsDisconnected = false
		disconnectResult.InsightsDisconnectedError = errMsg
		interactivePrintf("%v %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.InsightsDisconnected = true
		interactivePrintf("%v Disconnected from Red Hat Insights\n", uiSettings.iconOK)
	}
	durations["insights"] = time.Since(start)

	/* 3. Unregister system from Red Hat Subscription Management */
	err = showProgress(
		" Disconnecting from Red Hat Subscription Management...", unregister,
	)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Subscription Management: %v", err)
		errorMessages["rhsm"] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}

		disconnectResult.RHSMDisconnected = false
		disconnectResult.RHSMDisconnectedError = errMsg
		interactivePrintf("%v %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.RHSMDisconnected = true
		interactivePrintf("%v Disconnected from Red Hat Subscription Management\n", uiSettings.iconOK)
	}
	durations["rhsm"] = time.Since(start)

	if !uiSettings.isMachineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
		showTimeDuration(durations)

		err = showErrorMessages("disconnect", errorMessages)
		if err != nil {
			return err
		}
	}

	{ // update cache to say we have successfully disconnected
		log.Info("system has been disconnected, updating state file")
		SetConnected(false)
	}
	return cli.Exit(disconnectResult, 0)
}

// canonicalFactAction tries to gather canonical facts about system,
// and it prints JSON with facts to stdout.
func canonicalFactAction(_ *cli.Context) error {
	// NOTE: CLI context is not useful for anything
	facts, err := GetCanonicalFacts()
	if err != nil {
		return cli.Exit(err, 1)
	}
	data, err := json.MarshalIndent(facts, "", "   ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// SystemStatus is structure holding information about system status
// When more file format is supported, then add more tags for fields
// like xml:"hostname"
type SystemStatus struct {
	SystemHostname    string `json:"hostname"`
	HostnameError     string `json:"hostname_error,omitempty"`
	RHSMConnected     bool   `json:"rhsm_connected"`
	RHSMError         string `json:"rhsm_error,omitempty"`
	InsightsConnected bool   `json:"insights_connected"`
	InsightsError     string `json:"insights_error,omitempty"`
	YggdrasilRunning  bool   `json:"yggdrasil_running"`
	YggdrasilError    string `json:"yggdrasil_error,omitempty"`
	returnCode        int
}

// printJSONStatus tries to print the system status as JSON to stdout.
// When marshaling of systemStatus fails, then error is returned
func printJSONStatus(systemStatus *SystemStatus) error {
	data, err := json.MarshalIndent(systemStatus, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// beforeStatusAction ensures the user has supplied a correct `--format` flag.
func beforeStatusAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	return checkForUnknownArgs(ctx)
}

// statusAction tries to print status of system. It means that it gives
// answer on following questions:
// 1. Is system registered to Red Hat Subscription Management?
// 2. Is system connected to Red Hat Insights?
// 3. Is yggdrasil.service (rhcd.service) running?
// Status can be printed as human-readable text or machine-readable JSON document.
// Format is influenced by --format json CLI option stored in CLI context
func statusAction(ctx *cli.Context) (err error) {
	var systemStatus SystemStatus
	var machineReadablePrintFunc func(systemStatus *SystemStatus) error

	format := ctx.String("format")
	switch format {
	case "json":
		machineReadablePrintFunc = printJSONStatus
	default:
		break
	}

	// When printing of status is requested, then print machine-readable file format
	// at the end of this function
	if uiSettings.isMachineReadable {
		defer func(systemStatus *SystemStatus) {
			err = machineReadablePrintFunc(systemStatus)
			// When it was not possible to print status to machine-readable format, then
			// change returned error to CLI exit error to be able to set exit code to
			// a non-zero value
			if err != nil {
				err = cli.Exit(
					fmt.Errorf("unable to print status as %s document: %s", format, err.Error()),
					1)
			}
			// When any of status is not correct, then return 1 exit code
			if systemStatus.returnCode != 0 {
				err = cli.Exit("", 1)
			}
		}(&systemStatus)
	}

	hostname, err := os.Hostname()
	if err != nil {
		if uiSettings.isMachineReadable {
			systemStatus.HostnameError = err.Error()
		} else {
			return cli.Exit(err, 1)
		}
	}

	if uiSettings.isMachineReadable {
		systemStatus.SystemHostname = hostname
	} else {
		fmt.Printf("Connection status for %v:\n\n", hostname)
	}

	/* 1. Get Status of RHSM */
	err = rhsmStatus(&systemStatus)
	if err != nil {
		return cli.Exit(err, 1)
	}

	/* 2. Get status of insights-client */
	insightStatus(&systemStatus)

	/* 3. Get status of yggdrasil (rhcd) service */
	err = serviceStatus(&systemStatus)
	if err != nil {
		return cli.Exit(err, 1)
	}

	if !uiSettings.isMachineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
	}

	// At the end check if all statuses are correct.
	// If not, return 1 exit code without any message.
	if systemStatus.returnCode != 0 {
		return cli.Exit("", 1)
	}

	return nil
}

func configureFeatureAction(ctx *cli.Context) error {
	if !ctx.IsSet("enable") && !ctx.IsSet("disable") {
		// no flag entered, let's display help and currently enabled features
		_ = cli.ShowSubcommandHelp(ctx)
		return nil
	}

	// apply determines whether a change should be applied or not
	var apply bool = IsConnected()

	enableRequest, disableRequest, err := resolveFeatureInput(ctx.StringSlice("enable"), ctx.StringSlice("disable"))
	if err != nil {
		return err
	}

	state := GetFeaturesState()
	// newState holds true for features that are currently enabled
	newState := map[string]bool{}
	{
		for _, featureID := range state.Enabled {
			newState[featureID] = true
		}
	}

	enable, disable, err := resolveFeatureTransitions(state, enableRequest, disableRequest)
	if err != nil {
		return err
	}

	{ // log what we have resolved
		var enableIDs, disableIDs []string
		for _, feature := range enable {
			enableIDs = append(enableIDs, feature.ID)
		}
		for _, feature := range disable {
			disableIDs = append(disableIDs, feature.ID)
		}
		log.Infof(
			"resolved features: state=%s enable=%s disable=%s",
			strings.Join(state.Enabled, ","), strings.Join(enableIDs, ","), strings.Join(disableIDs, ","),
		)
	}

	{ // first, let's disable features one by one from the most to the least dependent
		knownFeaturesReversed := make([]*Feature, len(KnownFeatures))
		copy(knownFeaturesReversed, KnownFeatures)
		slices.Reverse(knownFeaturesReversed)

		for _, feature := range knownFeaturesReversed {
			for _, featureToBeDisabled := range disable {
				if feature.ID == featureToBeDisabled.ID {
					log.Debugf("Disabling feature %s", feature.ID)
					if apply {
						err = feature.DisableFunc(ctx)
						if err != nil {
							log.Errorf("Could not disable feature %s: %v", feature.ID, err)
							return err
						}
					}
					newState[feature.ID] = false
					log.Debugf("Disabled feature %s", feature.ID)
					break
				}
			}
		}
	}

	{ // next, enable features one by one from the last to the most dependent
		for _, feature := range KnownFeatures {
			for _, featureToBeEnabled := range enable {
				if feature.ID == featureToBeEnabled.ID {
					log.Debugf("Enabling feature %s", feature.ID)
					if apply {
						err = feature.EnableFunc(ctx)
						if err != nil {
							log.Errorf("Could not enable feature %s: %v", feature.ID, err)
							return err
						}
					}
					newState[feature.ID] = true
					log.Debugf("Enabled feature %s", feature.ID)
					break
				}
			}
		}
	}

	{ // update the state cache
		var featureList []string
		for featureID, enabled := range newState {
			if enabled {
				featureList = append(featureList, featureID)
			}
		}
		state.Enabled = featureList
		log.Debugf("Caching enabled features into %s: %s", FeaturesStatePath, strings.Join(state.Enabled, ","))
		if err = state.Save(); err != nil {
			return err
		}
	}
	return nil
}

// resolveFeatureInput determines which features should be enabled and which disabled.
// It does not take into account the system state, it only evaluates the dependency tree.
// On conflict, returns an error.
func resolveFeatureInput(inEnable, inDisable []string) (
	enable []*Feature, disable []*Feature, err error,
) {
	toEnable := map[string]bool{}
	toDisable := map[string]bool{}

	for _, featureID := range inEnable {
		feature, ok := GetFeature(featureID)
		if !ok {
			err = fmt.Errorf("unknown feature: %s", featureID)
			return
		}
		toEnable[featureID] = true
		// enable features this feature depends on
		for _, dependent := range feature.Requires {
			toEnable[dependent.ID] = true
		}
	}

	for _, featureID := range inDisable {
		feature, ok := GetFeature(featureID)
		if !ok {
			err = fmt.Errorf("unknown feature: %s", featureID)
			return
		}
		for _, requiredFeature := range RequiredFeatures {
			if featureID == requiredFeature.ID {
				err = fmt.Errorf("feature '%s' cannot be disabled", featureID)
				return
			}
		}

		toDisable[featureID] = true
		// disable features that depend on this feature
		for _, knownFeature := range KnownFeatures {
			for _, knownFeatureDependent := range knownFeature.Requires {
				if knownFeatureDependent.ID == feature.ID {
					toDisable[knownFeature.ID] = true
				}
			}
		}
	}

	// conflictingFeatures contains a sorted list of conflicting features
	var conflictingFeatures []string
	for _, feature := range KnownFeatures {
		wantsEnable, wantsDisable := false, false
		for featureIDEnable, _ := range toEnable {
			if featureIDEnable == feature.ID {
				wantsEnable = true
			}
		}
		for featureIDDisable, _ := range toDisable {
			if featureIDDisable == feature.ID {
				wantsDisable = true
			}
		}
		if wantsEnable && wantsDisable {
			conflictingFeatures = append(conflictingFeatures, feature.ID)
		}
	}
	if len(conflictingFeatures) > 0 {
		err = fmt.Errorf(
			"features can't be enabled and disabled at the same time: %s",
			strings.Join(conflictingFeatures, ", "),
		)
		return
	}

	for featureID, _ := range toEnable {
		feature, _ := GetFeature(featureID)
		enable = append(enable, feature)
	}
	for featureID, _ := range toDisable {
		feature, _ := GetFeature(featureID)
		disable = append(disable, feature)
	}

	// featureIDs is lambda mapping features to their IDs
	featureIDs := func(features []*Feature) []string {
		result := make([]string, len(features))
		for i, feature := range features {
			result[i] = feature.ID
		}
		return result
	}
	log.Debugf(
		"resolved features input: enable=%s disable=%s",
		strings.Join(featureIDs(enable), ","),
		strings.Join(featureIDs(disable), ","),
	)
	return
}

// resolveFeatureTransitions determines which features need to be enabled and disabled.
func resolveFeatureTransitions(
	state *FeaturesState, enableRequest []*Feature, disableRequest []*Feature,
) (
	enable []*Feature, disable []*Feature, err error,
) {
	for _, feature := range enableRequest {
		isEnabled := false
		for _, enabledFeatureID := range state.Enabled {
			if feature.ID == enabledFeatureID {
				isEnabled = true
			}
		}
		if !isEnabled {
			enable = append(enable, feature)
		}
	}
	for _, feature := range disableRequest {
		for _, enabledFeatureID := range state.Enabled {
			if feature.ID == enabledFeatureID {
				disable = append(disable, feature)
			}
		}
	}
	return
}

// mainAction is triggered in the case, when no sub-command is specified
func mainAction(c *cli.Context) error {
	type GenerationFunc func() (string, error)
	var generationFunc GenerationFunc
	if c.Bool("generate-man-page") {
		generationFunc = c.App.ToMan
	} else if c.Bool("generate-markdown") {
		generationFunc = c.App.ToMarkdown
	} else {
		cli.ShowAppHelpAndExit(c, 0)
	}
	data, err := generationFunc()
	if err != nil {
		return cli.Exit(err, 1)
	}
	fmt.Println(data)
	return nil
}

// beforeAction is triggered before other actions are triggered
func beforeAction(c *cli.Context) error {
	/* Load the configuration values from the config file specified*/
	filePath := c.String("config")
	if filePath != "" {
		inputSource, err := altsrc.NewTomlSourceFromFile(filePath)
		if err != nil {
			return err
		}
		if err := altsrc.ApplyInputSourceValues(c, inputSource, c.App.Flags); err != nil {
			return err
		}
	}

	config = Conf{
		LogLevel: c.String(cliLogLevel),
		CertFile: c.String(cliCertFile),
		KeyFile:  c.String(cliKeyFile),
	}

	level, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		return cli.Exit(err, 1)
	}
	log.SetLevel(level)

	// When environment variable NO_COLOR or --no-color CLI option is set, then do not display colors
	// and animations too. The NO_COLOR environment variable have to have value "1" or "true",
	// "True", "TRUE" to take effect
	// When no-color is not set, then try to detect if the output goes to some file. In this case
	// colors nor animations will not be printed to file.
	if !isTerminal(os.Stdout.Fd()) {
		err := c.Set("no-color", "true")
		if err != nil {
			log.Debug("Unable to set no-color flag to \"true\"")
		}
	}

	// Set up standard output preference: colors, icons, etc.
	configureUISettings(c)

	return nil
}

var config = Conf{}

func getConfigureFeaturesDescription() string {
	state := GetFeaturesState()

	var result = []string{"Current state of features:"}
	for _, feature := range KnownFeatures {
		line := ""
		// FIXME This should probably look the same as the reporting during 'connect'
		if state.IsEnabled(feature.ID) {
			line += "[x] "
		} else {
			line += "[ ] "
		}
		line += fmt.Sprintf("%s: %s", feature.ID, feature.Description)
		if len(feature.Requires) > 0 {
			line += " (requires: "
			var dependsOn []string
			for _, dependency := range feature.Requires {
				dependsOn = append(dependsOn, dependency.ID)
			}
			line += strings.Join(dependsOn, ", ")
			line += ")"
		}
		if state.IsDefault(feature.ID) {
			line += " (default)"
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func main() {
	app := cli.NewApp()
	app.Name = ShortName
	app.Version = Version
	app.Usage = "control the system's connection to " + Provider
	app.Description = "The " + app.Name + " command controls the system's connection to " + Provider + ".\n\n" +
		"To connect the system using an activation key:\n" +
		"\t" + app.Name + " connect --organization ID --activation-key KEY\n\n" +
		"To connect the system using a username and password:\n" +
		"\t" + app.Name + " connect --username USERNAME --password PASSWORD\n\n" +
		"To disconnect the system:\n" +
		"\t" + app.Name + " disconnect\n\n" +
		"Run '" + app.Name + " command --help' for more details."

	log.SetFlags(0)
	log.SetPrefix("")

	defaultConfigFilePath, err := ConfigPath()
	if err != nil {
		log.Fatal(err)
	}

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:    "no-color",
			Hidden:  false,
			Value:   false,
			EnvVars: []string{"NO_COLOR"},
		},
		&cli.StringFlag{
			Name:      "config",
			Hidden:    true,
			Value:     defaultConfigFilePath,
			TakesFile: true,
			Usage:     "Read config values from `FILE`",
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   cliCertFile,
			Hidden: true,
			Usage:  "Use `FILE` as the client certificate",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   cliKeyFile,
			Hidden: true,
			Usage:  "Use `FILE` as the client's private key",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:   cliLogLevel,
			Value:  "info",
			Hidden: true,
			Usage:  "Set the logging output level to `LEVEL`",
		}),
	}

	app.Commands = []*cli.Command{
		{
			Name: "connect",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "username",
					Usage:   "register with `USERNAME`",
					Aliases: []string{"u"},
				},
				&cli.StringFlag{
					Name:    "password",
					Usage:   "register with `PASSWORD`",
					Aliases: []string{"p"},
				},
				&cli.StringFlag{
					Name:    "organization",
					Usage:   "register with `ID`",
					Aliases: []string{"o"},
				},
				&cli.StringSliceFlag{
					Name:    "activation-key",
					Usage:   "register with `KEY`",
					Aliases: []string{"a"},
				},
				&cli.StringFlag{
					Name:   "server",
					Hidden: true,
					Usage:  "register against `URL`",
				},
				&cli.StringFlag{
					Name:    "format",
					Usage:   "prints output of connection in machine-readable format (supported formats: \"json\")",
					Aliases: []string{"f"},
				},
			},
			Usage:       "Connects the system to " + Provider,
			UsageText:   fmt.Sprintf("%v connect [command options]", app.Name),
			Description: fmt.Sprintf("The connect command connects the system to Red Hat Subscription Management, Red Hat Insights and %v and activates the %v service that enables %v to interact with the system. For details visit: https://red.ht/connector", Provider, ServiceName, Provider),
			Before:      beforeConnectAction,
			Action:      connectAction,
		},
		{
			Name: "disconnect",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "format",
					Usage:   "prints output of disconnection in machine-readable format (supported formats: \"json\")",
					Aliases: []string{"f"},
				},
			},
			Usage:       "Disconnects the system from " + Provider,
			UsageText:   fmt.Sprintf("%v disconnect", app.Name),
			Description: fmt.Sprintf("The disconnect command disconnects the system from Red Hat Subscription Management, Red Hat Insights and %v and deactivates the %v service. %v will no longer be able to interact with the system.", Provider, ServiceName, Provider),
			Before:      beforeDisconnectAction,
			Action:      disconnectAction,
		},
		{
			Name:        "canonical-facts",
			Hidden:      true,
			Usage:       "Prints canonical facts about the system.",
			UsageText:   fmt.Sprintf("%v canonical-facts", app.Name),
			Description: fmt.Sprintf("The canonical-facts command prints data that uniquely identifies the system in the %v inventory service. Use only as directed for debugging purposes.", Provider),
			Action:      canonicalFactAction,
		},
		{
			Name: "status",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "format",
					Usage:   "prints status in machine-readable format (supported formats: \"json\")",
					Aliases: []string{"f"},
				},
			},
			Usage:       "Prints status of the system's connection to " + Provider,
			UsageText:   fmt.Sprintf("%v status", app.Name),
			Description: fmt.Sprintf("The status command prints the state of the connection to Red Hat Subscription Management, Red Hat Insights and %v.", Provider),
			Before:      beforeStatusAction,
			Action:      statusAction,
		},
		{
			Name:      "configure",
			Usage:     "Configures the program functionality",
			UsageText: fmt.Sprintf("%v configure", app.Name),
			Subcommands: []*cli.Command{
				{
					Name:  "features",
					Usage: fmt.Sprintf("Manages %s features", app.Name),
					Description: fmt.Sprintf(
						"Allows enabling and disabling features of %s.\n\n%s",
						app.Name, getConfigureFeaturesDescription(),
					),
					Flags: []cli.Flag{
						&cli.StringSliceFlag{
							Name:  "enable",
							Usage: "Enable feature",
						},
						&cli.StringSliceFlag{
							Name:  "disable",
							Usage: "Disable feature",
						},
					},
					Action: configureFeatureAction,
				},
			},
		},
	}
	app.EnableBashCompletion = true
	app.BashComplete = BashComplete
	app.Action = mainAction
	app.Before = beforeAction

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
	}
}
