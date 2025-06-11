package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/godbus/dbus/v5"
)

const EnvTypeContentTemplate = "content-template"

func getConsumerUUID() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	locale := getLocale()

	var uuid string
	if err := conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Consumer").Call(
		"com.redhat.RHSM1.Consumer.GetUuid",
		dbus.Flags(0),
		locale).Store(&uuid); err != nil {
		return "", unpackRHSMError(err)
	}
	return uuid, nil
}

// Organization is structure containing information about RHSM organization (sometimes called owner)
// JSON document returned from candlepin server can have the following format. We care only about key,
// but it can be extended and more information can be added to the structure in the future.
//
//	{
//	   "created": "2022-11-02T16:00:23+0000",
//	   "updated": "2022-11-02T16:00:48+0000",
//	   "id": "4028face84391264018439127db10004",
//	   "displayName": "Donald Duck",
//	   "key": "donaldduck",
//	   "contentPrefix": null,
//	   "defaultServiceLevel": null,
//	   "logLevel": null,
//	   "contentAccessMode": "org_environment",
//	   "contentAccessModeList": "entitlement,org_environment",
//	   "autobindHypervisorDisabled": false,
//	   "autobindDisabled": false,
//	   "lastRefreshed": "2022-11-02T16:00:48+0000",
//	   "parentOwner": null,
//	   "upstreamConsumer": null
//	}
type Organization struct {
	Key string `json:"key"`
}

// unpackOrgs tries to unpack list organization from JSON document returned by D-Bus method GetOrgs.
// When it is possible to unmarshal the JSON document, then return list of organization keys (IDs).
// When it is not possible to get list of organizations, then return empty slice and error.
func unpackOrgs(s string) ([]string, error) {
	var orgs []string

	var organizations []Organization

	err := json.Unmarshal([]byte(s), &organizations)
	if err != nil {
		return orgs, err
	}

	for _, org := range organizations {
		orgs = append(orgs, org.Key)
	}

	return orgs, nil
}

// registerUsernamePassword tries to register system against candlepin server (Red Hat Management Service)
// username and password are mandatory. When organization is not obtained, then this method
// returns list of available organization and user can select one organization from the list.
func registerUsernamePassword(username, password, organization string, environments []string, enableContent bool) ([]string, error) {
	var orgs []string

	conn, err := dbus.SystemBus()
	if err != nil {
		return orgs, err
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return orgs, err
	}
	if uuid != "" {
		return orgs, fmt.Errorf("warning: the system is already registered")
	}

	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	locale := getLocale()

	var privateDbusSocketURI string
	if err := registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Start",
		dbus.Flags(0),
		locale).Store(&privateDbusSocketURI); err != nil {
		return orgs, err
	}
	defer registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Stop",
		dbus.FlagNoReplyExpected,
		locale)

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return orgs, err
	}
	defer func() {
		err = privConn.Close()
		if err != nil {
			slog.Error(
				"unable to close connection to private dbus socket",
				"socket", privateDbusSocketURI,
				"err", err,
			)
		}
	}()

	if err := privConn.Auth(nil); err != nil {
		return orgs, err
	}

	options := make(map[string]string)
	if len(environments) != 0 {
		options["environment_names"] = strings.Join(environments, ",")
		options["environment_type"] = EnvTypeContentTemplate

	}

	options["enable_content"] = fmt.Sprintf("%v", enableContent)

	if err := privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register").Call(
		"com.redhat.RHSM1.Register.Register",
		dbus.Flags(0),
		organization,
		username,
		password,
		options,
		map[string]string{},
		locale).Err; err != nil {

		// Try to unpack D-Bus method
		err := unpackRHSMError(err)

		// Is unpacked error RHSMError
		rhsmError, ok := err.(RHSMError)
		if !ok {
			return orgs, err
		}

		// When organization was not specified, and it is required to specify it, then
		// try to get list of available organizations
		if organization == "" && rhsmError.Exception == "OrgNotSpecifiedException" {
			var s string
			orgsCall := privConn.Object(
				"com.redhat.RHSM1",
				"/com/redhat/RHSM1/Register",
			).Call(
				"com.redhat.RHSM1.Register.GetOrgs",
				dbus.Flags(0),
				username,
				password,
				map[string]string{},
				locale,
			)

			err = orgsCall.Store(&s)
			if err != nil {
				return orgs, err
			}

			orgs, err = unpackOrgs(s)
			return orgs, err
		}
		return orgs, unpackRHSMError(err)
	}

	return orgs, nil
}

func registerActivationKey(orgID string, activationKeys []string, environments []string, enableContent bool) error {

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return err
	}
	if uuid != "" {
		return fmt.Errorf("warning: the system is already registered")
	}

	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	locale := getLocale()

	var privateDbusSocketURI string
	if err := registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Start",
		dbus.Flags(0),
		locale).Store(&privateDbusSocketURI); err != nil {
		return err
	}
	defer registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Stop",
		dbus.FlagNoReplyExpected,
		locale)

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return err
	}
	defer func() {
		err = privConn.Close()
		if err != nil {
			slog.Error(
				"unable to close connection to private dbus socket",
				"socket", privateDbusSocketURI,
				"err", err,
			)
		}
	}()

	if err := privConn.Auth(nil); err != nil {
		return err
	}

	options := make(map[string]string)
	if len(environments) != 0 {
		options["environment_names"] = strings.Join(environments, ",")
		options["environment_type"] = EnvTypeContentTemplate
	}

	options["enable_content"] = fmt.Sprintf("%v", enableContent)

	if err := privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register").Call(
		"com.redhat.RHSM1.Register.RegisterWithActivationKeys",
		dbus.Flags(0),
		orgID,
		activationKeys,
		options,
		map[string]string{},
		locale).Err; err != nil {
		return unpackRHSMError(err)
	}

	return nil
}

func unregister() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return err
	}
	if uuid == "" {
		return fmt.Errorf("warning: the system is already unregistered")
	}

	locale := getLocale()

	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Unregister").Call(
		"com.redhat.RHSM1.Unregister.Unregister",
		dbus.Flags(0),
		map[string]string{},
		locale).Err

	if err != nil {
		return unpackRHSMError(err)
	}

	return nil
}

// RHSMError is used for parsing JSON document returned by D-Bus methods.
type RHSMError struct {
	Exception string `json:"exception"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// Error return textual representation of RHSMError. This implements all needed
// methods for error interface. Thus, RHSMError can be handled as regular error.
func (rhsmError RHSMError) Error() string {
	return fmt.Sprintf("%v: %v", rhsmError.Severity, rhsmError.Message)
}

// unpackRHSMError tries to unpack JSON document (part of error) into the structure RHSMError. When it is
// not possible to parse error into structure, then corresponding or original error is returned.
// When it is possible to parse error into structure, then RHSMError is returned
func unpackRHSMError(err error) error {
	rhsmError := RHSMError{}
	switch e := err.(type) {
	case dbus.Error:
		if e.Name == "com.redhat.RHSM1.Error" {
			if jsonErr := json.Unmarshal([]byte(e.Error()), &rhsmError); jsonErr != nil {
				return jsonErr
			}
			return rhsmError
		}
		return fmt.Errorf("unable to parse D-Bus error due to unsupported error name: %s", e.Name)
	}
	return err
}

// registerRHSM tries to register system against Red Hat Subscription Management server (candlepin server)
func registerRHSM(ctx *cli.Context, enableContent bool) (string, error) {
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
		contentTemplates := ctx.StringSlice("content-template")

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
			s.Prefix = smallIndent + "["
			s.Suffix = "] Connecting to Red Hat Subscription Management..."
			s.Start()
			defer s.Stop()
		}

		var err error
		if len(activationKeys) > 0 {
			err = registerActivationKey(
				organization,
				ctx.StringSlice("activation-key"),
				contentTemplates,
				enableContent)
		} else {
			var orgs []string
			if organization != "" {
				_, err = registerUsernamePassword(username, password, organization, contentTemplates, enableContent)
			} else {
				orgs, err = registerUsernamePassword(username, password, "", contentTemplates, enableContent)
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
					_, err = registerUsernamePassword(username, password, organization, contentTemplates, enableContent)
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

// isRHSMRegistered returns true, when system is registered
func isRHSMRegistered() (bool, error) {
	uuid, err := getConsumerUUID()
	if err != nil {
		return false, err
	}
	if uuid != "" {
		return true, nil
	}
	return false, nil
}
