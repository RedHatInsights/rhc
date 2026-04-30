package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pelletier/go-toml"
	altsrc "github.com/urfave/cli-altsrc/v3"
	altsrctoml "github.com/urfave/cli-altsrc/v3/toml"
	docs "github.com/urfave/cli-docs/v3"
	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/feature"
	"github.com/redhatinsights/rhc/pkg/version"
)

const (
	cliLogLevel  = "log-level"
	cliCertFile  = "cert-file"
	cliKeyFile   = "key-file"
	cliAPIServer = "base-url"
)

// mainAction is triggered in the case, when no sub-command is specified
func mainAction(ctx context.Context, cmd *cli.Command) error {
	type GenerationFunc func(*cli.Command) (string, error)
	var generationFunc GenerationFunc
	if cmd.Bool("generate-man-page") {
		generationFunc = docs.ToMan
	} else if cmd.Bool("generate-markdown") {
		generationFunc = docs.ToMarkdown
	} else {
		cli.ShowAppHelpAndExit(cmd, 0)
	}
	data, err := generationFunc(cmd.Root())
	if err != nil {
		return cli.Exit(err, exitcode.Err)
	}
	fmt.Println(data)
	return nil
}

// configureUI sets up the global UI state by calling ui.ConfigureOutput
// with appropriate parameters.
func configureUI(cmd *cli.Command) {
	ui.ConfigureOutput(
		// Rich output (animations) is only enabled when all are true:
		// - we're printing in human-friendly format,
		// - stdout is an interactive console.
		!cmd.IsSet("format") && ui.IsInteractive(),
		// Colors are only enabled when all are true:
		// output is rich,
		// --no-color/$NO_COLOR are not set.
		!cmd.IsSet("no-color"),
		// Machine-readable output is enabled when all are true:
		// - we're printing in JSON or other parseable format.
		cmd.IsSet("format"),
	)
}

// beforeAction is triggered before other actions are triggered
func beforeAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	// check if --log-level was set via command line
	var logLevelSrc string
	if cmd.IsSet(cliLogLevel) {
		logLevelSrc = "command line"
	}

	// validate file is parseable TOML
	configPath := cmd.String("config")
	if configPath != "" {
		if _, err := toml.LoadFile(configPath); err != nil {
			return ctx, fmt.Errorf("invalid config file %s: %w", configPath, err)
		}
	}

	// check if log-level was set via config file (command line has precedence)
	if logLevelSrc == "" && cmd.IsSet(cliLogLevel) {
		logLevelSrc = fmt.Sprintf("config file: '%s'", cmd.String("config"))
	}

	conf.Config = conf.Conf{
		CertFile: cmd.String(cliCertFile),
		KeyFile:  cmd.String(cliKeyFile),
	}

	logLevelStr := cmd.String(cliLogLevel)
	if err := conf.Config.LogLevel.UnmarshalText([]byte(logLevelStr)); err != nil {
		slog.Error(fmt.Sprintf("invalid log level '%s' set via %s", logLevelStr, logLevelSrc))
		conf.Config.LogLevel = slog.LevelInfo
	}

	if !cmd.Bool("generate-man-page") && !cmd.Bool("generate-markdown") {
		configureFileLogging(conf.Config.LogLevel)
		slog.Info(cmd.Root().Name+" started", "version", version.Version, "pid", os.Getpid())
	}

	// When environment variable NO_COLOR or --no-color CLI option is set, then do not display colors
	// and animations too. The NO_COLOR environment variable have to have value "1" or "true",
	// "True", "TRUE" to take effect
	// When no-color is not set, then try to detect if the output goes to some file. In this case
	// colors nor animations will not be printed to file.
	if !isTerminal(os.Stdout.Fd()) {
		err := cmd.Set("no-color", "true")
		if err != nil {
			slog.Debug("Unable to set no-color flag to \"true\"")
		}
	}

	// Set up standard output preference: colors, icons, etc.
	configureUI(cmd)

	return ctx, nil
}

// afterAction is triggered after other actions are triggered
func afterAction(ctx context.Context, cmd *cli.Command) error {
	return closeLogFile()
}

// exitErrHandler is triggered when an action returns a cli.ExitCoder (e.g cli.Exit("error", 1))
func exitErrHandler(ctx context.Context, cmd *cli.Command, err error) {
	_ = closeLogFile()

	// continue with default ExitErrHandler behavior
	cli.HandleExitCoder(err)
}

func main() {
	app := &cli.Command{}
	app.Name = "rhc"
	app.Version = version.Version
	app.Usage = "control the system's connection to Red Hat"
	app.Description = "The " + app.Name + " command controls the system's connection to Red Hat.\n\n" +
		"To connect the system using an activation key:\n" +
		"\t" + app.Name + " connect --organization ID --activation-key KEY\n\n" +
		"To connect the system using a username and password:\n" +
		"\t" + app.Name + " connect --username USERNAME --password PASSWORD\n\n" +
		"To disconnect the system:\n" +
		"\t" + app.Name + " disconnect\n\n" +
		"Run '" + app.Name + " command --help' for more details."

	var featureIdSlice []string
	for _, f := range feature.All() {
		featureIdSlice = append(featureIdSlice, f.ID())
	}
	featureIDs := strings.Join(featureIdSlice, ", ")

	configFilePath, err := ConfigPath()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(exitcode.Err)
	}

	configSource := altsrc.NewStringPtrSourcer(&configFilePath)

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
			Sources: cli.EnvVars("NO_COLOR"),
		},
		&cli.StringFlag{
			Name:        "config",
			Hidden:      true,
			Value:       configFilePath,
			Destination: &configFilePath,
			TakesFile:   true,
			Usage:       "Read config values from `FILE`",
		},
		&cli.StringFlag{
			Name:   cliCertFile,
			Hidden: true,
			Usage:  "Use `FILE` as the client certificate",
			Sources: cli.NewValueSourceChain(
				altsrctoml.TOML(cliCertFile, configSource),
			),
		},
		&cli.StringFlag{
			Name:   cliKeyFile,
			Hidden: true,
			Usage:  "Use `FILE` as the client's private key",
			Sources: cli.NewValueSourceChain(
				altsrctoml.TOML(cliKeyFile, configSource),
			),
		},
		&cli.StringFlag{
			Name:   cliLogLevel,
			Value:  "info",
			Hidden: true,
			Usage:  "Set the logging output level to `LEVEL`",
			Sources: cli.NewValueSourceChain(
				altsrctoml.TOML(cliLogLevel, configSource),
			),
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
			Usage:       "Connects the system to Red Hat",
			UsageText:   fmt.Sprintf("%v connect [command options]", app.Name),
			Description: "The connect command connects the system to Red Hat Subscription Management, Red Hat Lightspeed (formerly Insights) and Red Hat and activates the yggdrasil service that enables Red Hat to interact with the system. For details visit: https://red.ht/connector",
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
			Usage:       "Disconnects the system from Red Hat",
			UsageText:   fmt.Sprintf("%v disconnect", app.Name),
			Description: "The disconnect command disconnects the system from Red Hat Subscription Management, Red Hat Lightspeed (formerly Insights) and Red Hat and deactivates the yggdrasil service. Red Hat will no longer be able to interact with the system.",
			Before:      beforeDisconnectAction,
			Action:      disconnectAction,
		},
		{
			Name:        "configure",
			Usage:       "Configure system features",
			UsageText:   fmt.Sprintf("%v configure COMMAND", app.Name),
			Description: "The configure command allows you to manage feature preferences before or after system registration.",
			Commands: []*cli.Command{
				{
					Name:        "features",
					Usage:       "Manage feature levels",
					UsageText:   fmt.Sprintf("%v configure features COMMAND", app.Name),
					Description: "Enable or disable content management, analytics, or remote management.",
					Commands: []*cli.Command{
						{
							Name:   "status",
							Usage:  "Show status",
							Before: beforeFeaturesStatusAction,
							Action: featuresStatusAction,
						},
						{
							Name:      "enable",
							Usage:     "Enable a feature",
							ArgsUsage: fmt.Sprintf("FEATURE (allowed values: %s)", featureIDs),
							Before:    beforeFeaturesEnableAction,
							Action:    featuresEnableAction,
						},
						{
							Name:      "disable",
							Usage:     "Disable a feature",
							ArgsUsage: fmt.Sprintf("FEATURE (allowed values: %s)", featureIDs),
							Before:    beforeFeaturesDisableAction,
							Action:    featuresDisableAction,
						},
					},
				},
			},
		},
		{
			Name:        "canonical-facts",
			Hidden:      true,
			Usage:       "Prints canonical facts about the system.",
			UsageText:   fmt.Sprintf("%v canonical-facts", app.Name),
			Description: "The canonical-facts command prints data that uniquely identifies the system in the Red Hat inventory service. Use only as directed for debugging purposes.",
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
			Usage:       "Prints status of the system's connection to Red Hat",
			UsageText:   fmt.Sprintf("%v status", app.Name),
			Description: "The status command prints the state of the connection to Red Hat Subscription Management, Red Hat Lightspeed (formerly Insights) and Red Hat.",
			Before:      beforeStatusAction,
			Action:      statusAction,
		},
		{
			Name:      "collector",
			Usage:     "Collect data for analysis",
			UsageText: fmt.Sprintf("%v collector COMMAND [command options]", app.Name),
			Commands: []*cli.Command{
				{
					Name: "info",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints collector information in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:     "Display collector information",
					UsageText: fmt.Sprintf("%v collector info COLLECTOR", app.Name),
					Before:    beforeCollectorInfoAction,
					Action:    collectorInfoAction,
				},
				{
					Name: "list",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints list of collectors in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:     "List available collectors",
					UsageText: fmt.Sprintf("%v collector list", app.Name),
					Before:    beforeCollectorListAction,
					Action:    collectorListAction,
				},
				{
					Name: "timers",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "format",
							Usage:   "prints list of collector timers in machine-readable format (supported formats: \"json\")",
							Aliases: []string{"f"},
						},
					},
					Usage:     "List collector timers",
					UsageText: fmt.Sprintf("%v collector timers", app.Name),
					Before:    beforeCollectorTimersAction,
					Action:    collectorTimersAction,
				},
				{
					Name: "enable",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "now",
							Usage: "enable collector and trigger immediate collection",
						},
					},
					Usage:     "Enable timer-based collection",
					UsageText: fmt.Sprintf("%v collector enable COLLECTOR", app.Name),
					Before:    beforeCollectorEnableAction,
					Action:    collectorEnableAction,
				},
				{
					Name: "disable",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "now",
							Usage: "disable collector and stop the collection immediately",
						},
					},
					Usage:     "Disable timer-based collection",
					UsageText: fmt.Sprintf("%v collector disable COLLECTOR", app.Name),
					Before:    beforeCollectorDisableAction,
					Action:    collectorDisableAction,
				},
			},
		},
	}
	app.EnableShellCompletion = true
	app.ShellComplete = ShellComplete
	app.Action = mainAction
	app.Before = beforeAction
	app.After = afterAction
	app.ExitErrHandler = exitErrHandler

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, os.Args); err != nil {
		slog.Error(err.Error())
	}
}
