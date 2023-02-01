package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"git.sr.ht/~spc/go-log"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/urfave/cli/v2"
)

const redColor = "\u001B[31m"
const greenColor = "\u001B[32m"
const endColor = "\u001B[0m"

// Colorful prefixes
const ttyConnectedPrefix = greenColor + "●" + endColor
const ttyDisconnectedPrefix = redColor + "●" + endColor
const ttyErrorPrefix = redColor + "!" + endColor

// Black & white prefixes. Unicode characters
const bwConnectedPrefix = "✓"
const bwDisconnectedPrefix = "𐄂"
const bwErrorPrefix = "!"

// showProgress calls function and, when it is possible display spinner with
// some progress message.
func showProgress(progressMessage string, isColorful bool, function func() error) error {
	var s *spinner.Spinner
	if isColorful {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = progressMessage
		s.Start()
		// Stop spinner after running function
		defer func() { s.Stop() }()
	}
	return function()
}

// getColorPreferences tries to get color preferences form context
func getColorPreferences(ctx *cli.Context) (connectedPrefix string, disconnectedPrefix string,
	errorPrefix string, isColorful bool) {
	noColor := ctx.Bool("no-color")

	if noColor {
		isColorful = false
		connectedPrefix = bwConnectedPrefix
		disconnectedPrefix = bwDisconnectedPrefix
		errorPrefix = bwErrorPrefix
	} else {
		isColorful = true
		connectedPrefix = ttyConnectedPrefix
		disconnectedPrefix = ttyDisconnectedPrefix
		errorPrefix = ttyErrorPrefix
	}
	return
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
func showErrorMessages(action string, errorMessages map[string]error) error {
	if len(errorMessages) > 0 {
		fmt.Println()
		fmt.Printf("The following errors were encountered during %s:\n\n", action)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "STEP\tERROR\t")
		for svc, err := range errorMessages {
			_, _ = fmt.Fprintf(w, "%v\t%v\n", svc, err)
		}
		_ = w.Flush()
		return cli.Exit("", 1)
	}
	return nil
}

// registerRHSM tries to register system against Red Hat Subscription Management server (candlepin server)
func registerRHSM(ctx *cli.Context) (error, string) {
	uuid, err := getConsumerUUID()
	if err != nil {
		return cli.Exit(err, 1), "Unable to get consumer UUID"
	}
	var successMsg string

	_, _, _, isColorful := getColorPreferences(ctx)

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
					return cli.Exit(err, 1), "Unable to read password"
				}
				password = string(data)
				fmt.Printf("\n\n")
			}
		}

		var s *spinner.Spinner
		if isColorful {
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
			if organization != "" {
				err = registerPassword(username, password, organization, ctx.String("server"))
			} else {
				/* TODO: When organization was not specified using CLI option --organization and it is
				   required, because user is member of more than one organization, then ask for
				   the organization. RHSM D-Bus API can provide list of available organizations
				   in this case. */
				err = registerPassword(username, password, "", ctx.String("server"))
			}
		}
		if err != nil {
			return cli.Exit(err, 1), "Unable to register system to RHSM"
		}
		successMsg = "Connected to Red Hat Subscription Management"
	} else {
		successMsg = "This system is already connected to Red Hat Subscription Management"
	}
	return nil, successMsg
}

// connectAction tries to register system against Red Hat Subscription Management,
// connect system to Red Hat Insights and it also tries to start rhcd service
func connectAction(ctx *cli.Context) error {
	uid := os.Getuid()
	if uid != 0 {
		return cli.Exit(fmt.Errorf("error: non-root user cannot connect system"), 1)
	}

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]error)
	hostname, err := os.Hostname()
	if err != nil {
		return cli.Exit(err, 1)
	}

	fmt.Printf("Connecting %v to %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	connectedPrefix, disconnectedPrefix, errorPrefix, isColorful := getColorPreferences(ctx)

	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	var returnedMsg string
	err, returnedMsg = registerRHSM(ctx)
	if err != nil {
		errorMessages["rhsm"] = fmt.Errorf("cannot connect to Red Hat Subscription Management: %w", err)
		fmt.Printf(errorPrefix + " Cannot connect to Red Hat Subscription Management\n")
	} else {
		fmt.Printf(connectedPrefix + " " + returnedMsg + "\n")
	}
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	if _, exist := errorMessages["rhsm"]; exist {
		fmt.Printf(disconnectedPrefix + " Skipping connection to Red Hat Insights\n")
	} else {
		start = time.Now()
		err = showProgress(" Connecting to Red Hat Insights...", isColorful, registerInsights)
		if err != nil {
			errorMessages["insights"] = fmt.Errorf("cannot connect to Red Hat Insights: %w", err)
			fmt.Printf(errorPrefix + " Cannot connect to Red Hat Insights\n")
		} else {
			fmt.Printf(connectedPrefix + " Connected to Red Hat Insights\n")
		}
		durations["insights"] = time.Since(start)
	}

	/* 3. Start rhcd daemon */
	if _, exist := errorMessages["rhsm"]; exist {
		fmt.Printf(disconnectedPrefix+" Skipping activation of %v daemon\n", BrandName)
	} else {
		start = time.Now()
		progressMessage := fmt.Sprintf(" Activating the %v daemon", BrandName)
		err = showProgress(progressMessage, isColorful, activate)
		if err != nil {
			errorMessages[BrandName] = fmt.Errorf("cannot activate daemon: %w", err)
			fmt.Printf(errorPrefix+" Cannot activate the %v daemon\n", BrandName)
		} else {
			fmt.Printf(connectedPrefix+" Activated the %v daemon\n", BrandName)
		}
		durations[BrandName] = time.Since(start)
	}

	/* 4. Show footer message */
	fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

	/* 5. Optionally display duration time of each sub-action */
	showTimeDuration(durations)

	err = showErrorMessages("connect", errorMessages)
	if err != nil {
		return err
	}

	return nil
}

// disconnectAction tries to stop rhscd service, disconnect from Red Hat Insights and finally
// it unregister system from Red Hat Subscription Management
func disconnectAction(ctx *cli.Context) error {
	uid := os.Getuid()
	if uid != 0 {
		return cli.Exit(fmt.Errorf("error: non-root user cannot disconnect system"), 1)
	}

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]error)
	hostname, err := os.Hostname()
	if err != nil {
		return cli.Exit(err, 1)
	}
	fmt.Printf("Disconnecting %v from %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	_, disconnectedPrefix, errorPrefix, isColorful := getColorPreferences(ctx)

	/* 1. Deactivate rhcd daemon */
	start = time.Now()
	progressMessage := fmt.Sprintf(" Deactivating the %v daemon", BrandName)
	err = showProgress(progressMessage, isColorful, deactivate)
	if err != nil {
		errorMessages[BrandName] = fmt.Errorf("cannot deactivate daemon: %w", err)
		fmt.Printf(errorPrefix+" Cannot deactivate the %v daemon\n", BrandName)
	} else {
		fmt.Printf(disconnectedPrefix+" Deactivated the %v daemon\n", BrandName)
	}
	durations[BrandName] = time.Since(start)

	/* 2. Disconnect from Red Hat Insights */
	start = time.Now()
	err = showProgress(" Disconnecting from Red Hat Insights...", isColorful, unregisterInsights)
	if err != nil {
		errorMessages["insights"] = fmt.Errorf("cannot disconnect from Red Hat Insights: %w", err)
		fmt.Printf(errorPrefix + " Cannot disconnect from Red Hat Insights\n")
	} else {
		fmt.Print(disconnectedPrefix + " Disconnected from Red Hat Insights\n")
	}
	durations["insights"] = time.Since(start)

	/* 3. Unregister system from Red Hat Subscription Management */
	err = showProgress(" Disconnecting from Red Hat Subscription Management...", isColorful, unregister)
	if err != nil {
		errorMessages["rhsm"] = fmt.Errorf("cannot disconnect from Red Hat Subscription Management: %w", err)
		fmt.Printf(errorPrefix + " Cannot disconnect from Red Hat Subscription Management\n")
	} else {
		fmt.Printf(disconnectedPrefix + " Disconnected from Red Hat Subscription Management\n")
	}
	durations["rhsm"] = time.Since(start)

	fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

	showTimeDuration(durations)

	err = showErrorMessages("disconnect", errorMessages)
	if err != nil {
		return err
	}

	return nil
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
	HostnameError     error  `json:"hostname_error,omitempty"`
	RHSMConnected     bool   `json:"rhsm_connected"`
	RHSMError         error  `json:"rhsm_error,omitempty"`
	InsightsConnected bool   `json:"insights_connected"`
	InsightsError     error  `json:"insights_error,omitempty"`
	RHCDRunning       bool   `json:"rhcd_running"`
	RHCDError         error  `json:"rhcd_error,omitempty"`
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

// statusAction tries to print status of system. It means that it gives
// answer on following questions:
// 1. Is system registered to Red Hat Subscription Management?
// 2. Is system connected to Red Hat Insights?
// 3. Is rhcd.service running?
// Status can be printed as human-readable text or machine-readable JSON document.
// Format is influenced by --format json CLI option stored in CLI context
func statusAction(ctx *cli.Context) (err error) {
	var systemStatus SystemStatus
	machineReadable := false
	var machineReadablePrintFunc func(systemStatus *SystemStatus) error

	// Only JSON file format is supported ATM
	format := ctx.String("format")
	if format != "" {
		switch format {
		case "json":
			machineReadable = true
			machineReadablePrintFunc = printJSONStatus
		default:
			err := fmt.Errorf(
				"unsuported machine-readable format: %s (supported formats: %s)",
				format, "\"json\"",
			)
			return cli.Exit(err, 1)
		}
	}

	connectedPrefix, disconnectedPrefix, errorPrefix, isColorful := getColorPreferences(ctx)

	// When printing of status is requested, then print machine-readable file format
	// at the end of this function
	if machineReadable {
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
		}(&systemStatus)
		// Disable all colors and animations
		isColorful = false
	}

	hostname, err := os.Hostname()
	if err != nil {
		if machineReadable {
			systemStatus.HostnameError = err
		} else {
			return cli.Exit(err, 1)
		}
	}

	if machineReadable {
		systemStatus.SystemHostname = hostname
	} else {
		fmt.Printf("Connection status for %v:\n\n", hostname)
	}

	/* 1. Get Status of RHSM */
	uuid, err := getConsumerUUID()
	if err != nil {
		return cli.Exit(err, 1)
	}
	if uuid == "" {
		if machineReadable {
			systemStatus.RHSMConnected = false
		} else {
			fmt.Printf(disconnectedPrefix + " Not connected to Red Hat Subscription Management\n")
		}
	} else {
		if machineReadable {
			systemStatus.RHSMConnected = true
		} else {
			fmt.Printf(connectedPrefix + " Connected to Red Hat Subscription Management\n")
		}
	}

	/* 2. Get status of insights-client */
	var s *spinner.Spinner
	if isColorful {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Checking Red Hat Insights..."
		s.Start()
	}
	isRegistered, err := insightsIsRegistered()
	if isColorful {
		s.Stop()
	}
	if isRegistered {
		if machineReadable {
			systemStatus.InsightsConnected = true
		} else {
			fmt.Print(connectedPrefix + " Connected to Red Hat Insights\n")
		}
	} else {
		if err == nil {
			if machineReadable {
				systemStatus.InsightsConnected = true
			} else {
				fmt.Print(disconnectedPrefix + " Not connected to Red Hat Insights\n")
			}
		} else {
			if machineReadable {
				systemStatus.InsightsConnected = false
				systemStatus.InsightsError = err
			} else {
				fmt.Printf(errorPrefix+" Cannot execute insights-client: %v\n", err)
			}
		}
	}

	/* 3. Get status of rhcd */
	conn, err := systemd.NewSystemConnection()
	if err != nil {
		systemStatus.RHCDRunning = false
		systemStatus.RHCDError = err
		return cli.Exit(err, 1)
	}
	defer conn.Close()
	unitName := ShortName + "d.service"
	properties, err := conn.GetUnitProperties(unitName)
	if err != nil {
		systemStatus.RHCDRunning = false
		systemStatus.RHCDError = err
		return cli.Exit(err, 1)
	}
	activeState := properties["ActiveState"]
	if activeState.(string) == "active" {
		if machineReadable {
			systemStatus.RHCDRunning = true
		} else {
			fmt.Printf(connectedPrefix+" The %v daemon is active\n", BrandName)
		}
	} else {
		if machineReadable {
			systemStatus.RHCDRunning = false
		} else {
			fmt.Printf(disconnectedPrefix+" The %v daemon is inactive\n", BrandName)
		}
	}

	if !machineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
	}

	return nil
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
	level, err := log.ParseLevel(c.String("log-level"))
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

	return nil
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

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:   "generate-man-page",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "generate-markdown",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "log-level",
			Hidden: true,
			Value:  "error",
		},
		&cli.BoolFlag{
			Name:    "no-color",
			Hidden:  false,
			Value:   false,
			EnvVars: []string{"NO_COLOR"},
		},
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
					Name:  "server",
					Usage: "register against `URL`",
				},
			},
			Usage:       "Connects the system to " + Provider,
			UsageText:   fmt.Sprintf("%v connect [command options]", app.Name),
			Description: fmt.Sprintf("The connect command connects the system to Red Hat Subscription Management, Red Hat Insights and %v and activates the %v daemon that enables %v to interact with the system. For details visit: https://red.ht/connector", Provider, BrandName, Provider),
			Action:      connectAction,
		},
		{
			Name:        "disconnect",
			Usage:       "Disconnects the system from " + Provider,
			UsageText:   fmt.Sprintf("%v disconnect", app.Name),
			Description: fmt.Sprintf("The disconnect command disconnects the system from Red Hat Subscription Management, Red Hat Insights and %v and deactivates the %v daemon. %v will no longer be able to interact with the system.", Provider, BrandName, Provider),
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
			Action:      statusAction,
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
