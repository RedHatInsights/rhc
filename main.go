package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

const successPrefix = "\033[32m●\033[0m"
const failPrefix = "\033[31m●\033[0m"
const errorPrefix = "\033[31m!\033[0m"

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
			Action: func(c *cli.Context) error {
				var start time.Time
				durations := make(map[string]time.Duration)
				hostname, err := os.Hostname()
				if err != nil {
					return cli.Exit(err, 1)
				}

				fmt.Printf("Connecting %v to %v.\nThis might take a few seconds.\n\n", hostname, Provider)

				start = time.Now()
				uuid, err := getConsumerUUID()
				if err != nil {
					return cli.Exit(err, 1)
				}

				if uuid == "" {
					username := c.String("username")
					password := c.String("password")

					if c.String("organization") == "" {
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
								return cli.Exit(err, 1)
							}
							password = string(data)
							fmt.Printf("\n\n")
						}
					}

					s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
					s.Suffix = " Connecting to Red Hat Subscription Management..."
					s.Start()
					var err error
					if c.String("organization") != "" {
						err = registerActivationKey(c.String("organization"), c.StringSlice("activation-key"), c.String("server"))
					} else {
						err = registerPassword(username, password, c.String("server"))
					}
					if err != nil {
						s.Stop()
						return cli.Exit(err, 1)
					}
					s.Stop()
					fmt.Printf(successPrefix + " Connected to Red Hat Subscription Management\n")
				} else {
					fmt.Printf(successPrefix + " This system is already connected to Red Hat Subscription Management\n")
				}
				durations["rhsm"] = time.Since(start)

				start = time.Now()
				s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

				s.Suffix = " Connecting to Red Hat Insights..."
				s.Start()
				if err := registerInsights(); err != nil {
					s.Stop()
					return cli.Exit(err, 1)
				}
				s.Stop()
				fmt.Printf(successPrefix + " Connected to Red Hat Insights\n")
				durations["insights"] = time.Since(start)

				start = time.Now()
				s.Suffix = fmt.Sprintf(" Activating the %v daemon", BrandName)
				s.Start()
				if err := activate(); err != nil {
					s.Stop()
					return cli.Exit(err, 1)
				}
				s.Stop()
				fmt.Printf(successPrefix+" Activated the %v daemon\n", BrandName)
				durations[BrandName] = time.Since(start)

				fmt.Printf("\nManage your Red Hat connector systems: https://red.ht/connector\n")

				if log.CurrentLevel() >= log.LevelDebug {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "STEP\tDURATION\t")
					for step, duration := range durations {
						fmt.Fprintf(w, "%v\t%v\t\n", step, duration.Truncate(time.Millisecond))
					}
					w.Flush()
				}

				return nil
			},
		},
		{
			Name:        "disconnect",
			Usage:       "Disconnects the system from " + Provider,
			UsageText:   fmt.Sprintf("%v disconnect", app.Name),
			Description: fmt.Sprintf("The disconnect command disconnects the system from Red Hat Subscription Management, Red Hat Insights and %v and deactivates the %v daemon. %v will no longer be able to interact with the system.", Provider, BrandName, Provider),
			Action: func(c *cli.Context) error {
				var start time.Time
				durations := make(map[string]time.Duration)
				errorMessages := make(map[string]error)
				hostname, err := os.Hostname()
				if err != nil {
					return cli.Exit(err, 1)
				}
				fmt.Printf("Disconnecting %v from %v.\nThis might take a few seconds.\n\n", hostname, Provider)

				s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

				start = time.Now()
				s.Suffix = fmt.Sprintf(" Deactivating the %v daemon", BrandName)
				s.Start()
				if err := deactivate(); err != nil {
					errorMessages[BrandName] = fmt.Errorf("cannot deactivate daemon: %w", err)
					s.Stop()
					fmt.Printf(errorPrefix+" Cannot deactivate the %v daemon\n", BrandName)
				} else {
					s.Stop()
					fmt.Printf(failPrefix+" Deactivated the %v daemon\n", BrandName)
				}
				durations[BrandName] = time.Since(start)

				start = time.Now()
				s.Suffix = " Disconnecting from Red Hat Insights..."
				s.Start()
				if err := unregisterInsights(); err != nil {
					errorMessages["insights"] = fmt.Errorf("cannot disconnect from Red Hat Insights: %w", err)
					s.Stop()
					fmt.Printf(errorPrefix + " Cannot disconnect from Red Hat Insights\n")
				} else {
					s.Stop()
					fmt.Print(failPrefix + " Disconnected from Red Hat Insights\n")
				}
				durations["insights"] = time.Since(start)

				start = time.Now()
				s.Suffix = " Disconnecting from Red Hat Subscription Management..."
				s.Start()
				if err := unregister(); err != nil {
					errorMessages["rhsm"] = fmt.Errorf("cannot disconnect from Red Hat Subscription Management: %w", err)
					s.Stop()
					fmt.Printf(errorPrefix + " Cannot disconnect from Red Hat Subscription Management\n")
				} else {
					s.Stop()
					fmt.Printf(failPrefix + " Disconnected from Red Hat Subscription Management\n")
				}
				durations["rhsm"] = time.Since(start)

				fmt.Printf("\nManage your Red Hat connector systems: https://red.ht/connector\n")

				if log.CurrentLevel() >= log.LevelDebug {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "STEP\tDURATION\t")
					for step, duration := range durations {
						fmt.Fprintf(w, "%v\t%v\t\n", step, duration.Truncate(time.Millisecond))
					}
					w.Flush()
				}

				if len(errorMessages) > 0 {
					fmt.Println()
					fmt.Printf("The following errors were encountered during disconnect:\n\n")
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "STEP\tERROR\t")
					for svc, err := range errorMessages {
						fmt.Fprintf(w, "%v\t%v\n", svc, err)
					}
					w.Flush()
					return cli.Exit("", 1)
				}

				return nil
			},
		},
		{
			Name:        "canonical-facts",
			Hidden:      true,
			Usage:       "Prints canonical facts about the system.",
			UsageText:   fmt.Sprintf("%v canonical-facts", app.Name),
			Description: fmt.Sprintf("The canonical-facts command prints data that uniquely identifies the system in the %v inventory service. Use only as directed for debugging purposes.", Provider),
			Action: func(c *cli.Context) error {
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
			},
		},
		{
			Name:        "status",
			Usage:       "Prints status of the system's connection to " + Provider,
			UsageText:   fmt.Sprintf("%v status", app.Name),
			Description: fmt.Sprintf("The status command prints the state of the connection to Red Hat Subscription Management, Red Hat Insights and %v.", Provider),
			Action: func(c *cli.Context) error {
				hostname, err := os.Hostname()
				if err != nil {
					return cli.Exit(err, 1)
				}

				fmt.Printf("Connection status for %v:\n\n", hostname)

				uuid, err := getConsumerUUID()
				if err != nil {
					return cli.Exit(err, 1)
				}
				if uuid == "" {
					fmt.Printf(failPrefix + " Not connected to Red Hat Subscription Management\n")
				} else {
					fmt.Printf(successPrefix + " Connected to Red Hat Subscription Management\n")
				}

				s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

				s.Suffix = " Checking Red Hat Insights..."
				s.Start()
				isRegistered := insightsIsRegistered()
				s.Stop()

				if isRegistered {
					fmt.Print(successPrefix + " Connected to Red Hat Insights\n")
				} else {
					fmt.Print(failPrefix + " Not connected to Red Hat Insights\n")
				}

				conn, err := systemd.NewSystemConnection()
				if err != nil {
					return cli.Exit(err, 1)
				}
				defer conn.Close()

				unitName := ShortName + "d.service"

				properties, err := conn.GetUnitProperties(unitName)
				if err != nil {
					return cli.Exit(err, 1)
				}

				activeState := properties["ActiveState"]
				if activeState.(string) == "active" {
					fmt.Printf(successPrefix+" The %v daemon is active\n", BrandName)
				} else {
					fmt.Printf(failPrefix+" The %v daemon is inactive\n", BrandName)
				}

				fmt.Printf("\nManage your Red Hat connector systems: https://red.ht/connector\n")

				return nil
			},
		},
	}
	app.EnableBashCompletion = true
	app.BashComplete = BashComplete
	app.Action = func(c *cli.Context) error {
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
	app.Before = func(c *cli.Context) error {
		level, err := log.ParseLevel(c.String("log-level"))
		if err != nil {
			return cli.Exit(err, 1)
		}
		log.SetLevel(level)

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
	}
}
