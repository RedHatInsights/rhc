package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ini "git.sr.ht/~spc/go-ini"
	"git.sr.ht/~spc/go-log"

	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

// isTerminal returns true if the file descriptor is terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// BashCompleteCommand prints all visible flag options for the given command,
// and then recursively calls itself on each subcommand.
func BashCompleteCommand(cmd *cli.Command, w io.Writer) {
	for _, name := range cmd.Names() {
		fmt.Fprintf(w, "%v\n", name)
	}

	PrintFlagNames(cmd.VisibleFlags(), w)

	for _, command := range cmd.Subcommands {
		BashCompleteCommand(command, w)
	}
}

// PrintFlagNames prints the long and short names of each flag in the slice.
func PrintFlagNames(flags []cli.Flag, w io.Writer) {
	for _, flag := range flags {
		for _, name := range flag.Names() {
			if len(name) > 1 {
				fmt.Fprintf(w, "--%v\n", name)
			} else {
				fmt.Fprintf(w, "-%v\n", name)
			}
		}
	}
}

// BashComplete prints all commands, subcommands and flags to the application
// writer.
func BashComplete(c *cli.Context) {
	for _, command := range c.App.VisibleCommands() {
		BashCompleteCommand(command, c.App.Writer)

		// global flags
		PrintFlagNames(c.App.VisibleFlags(), c.App.Writer)
	}
}

func ConfigPath() (string, error) {
	// default config file path in `/etc/rhc/config.toml`
	filePath := filepath.Join("/etc", LongName, "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}

// GuessAPIURL gets the API server URL based on, insights-client.conf
// and rhsm.conf. This URL may differ from prod, stage and Satellite
func GuessAPIURL() (string, error) {
	var uString string
	var baseURL *url.URL

	// Check if the server api is set in insights conf
	// Create the structs needed to read the config file
	opts := ini.Options{
		AllowNumberSignComments: true,
	}
	type InsightsClientConf struct {
		BaseUrl string `ini:"base_url"`
	}
	type InsightsConf struct {
		InsightsClient InsightsClientConf `ini:"insights-client"`
	}

	var cfg InsightsConf
	// Read the config file
	confFilePath := "/etc/insights-client/insights-client.conf"
	data, err := os.ReadFile(confFilePath)
	if err != nil {
		return "", fmt.Errorf("fail to read file %v: %v", confFilePath, err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("configuratio file %v is empty", confFilePath)
	}
	// Get the config into the struct
	if err := ini.UnmarshalWithOptions(data, &cfg, opts); err != nil {
		return "", fmt.Errorf("fail to read configuration: %v", err)
	}
	APIServer := cfg.InsightsClient.BaseUrl

	if APIServer != "" {
		base, err := url.Parse("https://" + APIServer)
		if err != nil {
			return "", fmt.Errorf("cannot get base URL: %w", err)
		}
		p, _ := url.Parse("api/config-manager/v2/profiles/current")
		uString = base.ResolveReference(p).String()
	} else {
		// Get the server hostname where this host is connected
		var serverHost string
		err = getRHSMConfigOption("server.hostname", &serverHost)
		if err != nil {
			return "", fmt.Errorf("cannot get server hostname: %w", err)
		}
		// Get the final api server url to make the call
		// Check if it is the default api server
		if strings.Contains(serverHost, "subscription.rhsm.redhat.com") {
			baseURL, _ = url.Parse("https://cert.console.redhat.com")
			p, _ := url.Parse("api/config-manager/v2/profiles/current")
			uString = baseURL.ResolveReference(p).String()
		} else {
			// Otherwise it is connected to Satellite
			// Generate the base URL
			base, err := url.Parse("https://" + serverHost)
			if err != nil {
				return "", fmt.Errorf("cannot get base URL: %w", err)
			}
			p, _ := url.Parse("redhat_access/r/insights/platform/config-manager/v2/profiles/current")
			uString = base.ResolveReference(p).String()
		}
	}

	return uString, nil
}

// hasPriorityErrors checks if the errorMessage map has any error
// with a higher priority than the logLevel configure.
func hasPriorityErrors(errorMessages map[string]LogMessage, level log.Level) bool {
	for _, logMsg := range errorMessages {
		if logMsg.level <= level {
			return true
		}
	}
	return false
}

// getLocale tries to get current locale
func getLocale() string {
	// FIXME: Locale should be detected in more reliable way. We are going to support
	//        localization in better way. Maybe we could use following go module
	//        https://github.com/Xuanwo/go-locale. Maybe some other will be better.
	locale := os.Getenv("LANG")
	return locale
}
