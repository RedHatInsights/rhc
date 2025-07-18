package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

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
	// check if --log-level was set via command line
	var logLevelSrc string
	if c.IsSet(cliLogLevel) {
		logLevelSrc = "command line"
	}

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

	// check if log-level was set via config file (command line has precedence)
	if logLevelSrc == "" && c.IsSet(cliLogLevel) {
		logLevelSrc = fmt.Sprintf("config file: '%s'", c.String("config"))
	}

	config = Conf{
		CertFile: c.String(cliCertFile),
		KeyFile:  c.String(cliKeyFile),
	}

	logLevelStr := c.String(cliLogLevel)
	if err := config.LogLevel.UnmarshalText([]byte(logLevelStr)); err != nil {
		slog.Error(fmt.Sprintf("invalid log level '%s' set via %s", logLevelStr, logLevelSrc))
		config.LogLevel = slog.LevelInfo
	}

	slog.SetLogLoggerLevel(config.LogLevel)

	// When environment variable NO_COLOR or --no-color CLI option is set, then do not display colors
	// and animations too. The NO_COLOR environment variable have to have value "1" or "true",
	// "True", "TRUE" to take effect
	// When no-color is not set, then try to detect if the output goes to some file. In this case
	// colors nor animations will not be printed to file.
	if !isTerminal(os.Stdout.Fd()) {
		err := c.Set("no-color", "true")
		if err != nil {
			slog.Debug("Unable to set no-color flag to \"true\"")
		}
	}

	// Set up standard output preference: colors, icons, etc.
	configureUISettings(c)

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

	var featureIdSlice []string
	for _, featureID := range KnownFeatures {
		featureIdSlice = append(featureIdSlice, featureID.ID)
	}
	featureIDs := strings.Join(featureIdSlice, ", ")

	defaultConfigFilePath, err := ConfigPath()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
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
				&cli.StringSliceFlag{
					Name:    "content-template",
					Usage:   "register with `CONTENT_TEMPLATE`",
					Aliases: []string{"c"},
				},
				&cli.StringSliceFlag{
					Name:    "enable-feature",
					Usage:   fmt.Sprintf("enable `FEATURE` during connection (allowed values: %s)", featureIDs),
					Aliases: []string{"e"},
				},
				&cli.StringSliceFlag{
					Name:    "disable-feature",
					Usage:   fmt.Sprintf("disable `FEATURE` during connection (allowed values: %s)", featureIDs),
					Aliases: []string{"d"},
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
			Name:        "collector",
			Usage:       "Collect data for analysis",
			UsageText:   fmt.Sprintf("%v collector", app.Name),
			Description: "Manage data collectors. System data collected by collectors are typically send for analysis to " + Provider,
			Subcommands: []*cli.Command{
				{
					Name: "run",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints status in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
						&cli.BoolFlag{
							Name:    "keep",
							Usage:   "do not delete collected data",
							Aliases: []string{"k"},
						},
						&cli.BoolFlag{
							Name:    "no-upload",
							Usage:   "do not upload collected data",
							Aliases: []string{"n"},
						},
					},
					Usage:       "Execute specific collector",
					UsageText:   fmt.Sprintf("%v collector run COLLECTOR", app.Name),
					Description: "Execute specific collector",
					Before:      beforeCollectorRunAction,
					Action:      collectorRunAction,
				},
				{
					Name: "info",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints status in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:       "Prints information about specific collector",
					UsageText:   fmt.Sprintf("%v collector info COLLECTOR", app.Name),
					Description: "Prints information about specific collector",
					Before:      beforeCollectorInfoAction,
					Action:      collectorInfoAction,
				},
				{
					Name: "list",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints status in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:       "List available collectors",
					UsageText:   fmt.Sprintf("%v collector list", app.Name),
					Description: "List all collectors installed on the system",
					Before:      beforeCollectorListAction,
					Action:      collectorListAction,
				},
				{
					Name: "timers",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints status in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:       "List collector timers",
					UsageText:   fmt.Sprintf("%v collector timers", app.Name),
					Description: "List collector timers install on the system",
					Before:      beforeCollectorTimersAction,
					Action:      collectorTimersAction,
				},
				{
					Name: "enable",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:    "now",
							Usage:   "Enable collector and trigger immediate collection",
							Aliases: []string{"n"},
						},
					},
					Usage:       "Enable collector",
					UsageText:   fmt.Sprintf("%v collector enable COLLECTOR", app.Name),
					Description: "Enable given collector",
					Before:      beforeCollectorEnableAction,
					Action:      collectorEnableAction,
				},
				{
					Name:        "disable",
					Usage:       "Disable collector",
					UsageText:   fmt.Sprintf("%v collector disable COLLECTOR", app.Name),
					Description: "Disable given collector",
					Before:      beforeCollectorDisableAction,
					Action:      collectorDisableAction,
				},
			},
			Before: nil,
			Action: nil,
		},
	}
	app.EnableBashCompletion = true
	app.BashComplete = BashComplete
	app.Action = mainAction
	app.Before = beforeAction

	if err := app.Run(os.Args); err != nil {
		slog.Error(err.Error())
	}
}
