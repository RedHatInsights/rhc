package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

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
func registerUsernamePassword(username, password, organization string, environments []string, serverURL string) ([]string, error) {
	var orgs []string
	if serverURL != "" {
		if err := configureRHSM(serverURL); err != nil {
			return orgs, fmt.Errorf("cannot configure RHSM: %w", err)
		}
	}

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
	defer privConn.Close()

	if err := privConn.Auth(nil); err != nil {
		return orgs, err
	}

	var options = make(map[string]string)
	options["enable_content"] = "true"
	if len(environments) > 0 {
		options["environments"] = strings.Join(environments, ",")
	}

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

func registerActivationKey(orgID string, activationKeys []string, serverURL string) error {
	if serverURL != "" {
		if err := configureRHSM(serverURL); err != nil {
			return fmt.Errorf("cannot configure RHSM: %w", err)
		}
	}

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
	defer privConn.Close()

	if err := privConn.Auth(nil); err != nil {
		return err
	}

	if err := privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register").Call(
		"com.redhat.RHSM1.Register.RegisterWithActivationKeys",
		dbus.Flags(0),
		orgID,
		activationKeys,
		map[string]string{},
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

func configureRHSM(serverURL string) error {
	if _, err := os.Stat("/etc/rhsm/rhsm.conf.orig"); os.IsNotExist(err) {
		src, err := os.Open("/etc/rhsm/rhsm.conf")
		if err != nil {
			return fmt.Errorf("cannot open file for reading: %w", err)
		}
		defer src.Close()

		dst, err := os.Create("/etc/rhsm/rhsm.conf.orig")
		if err != nil {
			return fmt.Errorf("cannot open file for writing: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("cannot backup rhsm.conf: %w", err)
		}
		src.Close()
		dst.Close()
	}

	URL, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %w", err)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}

	locale := getLocale()

	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	// If the scheme is empty, attempt to set the server.hostname based on the
	// path component alone. This enables the --server argument to accept just a
	// host name without a full URI.
	if URL.Scheme == "" {
		if URL.Path != "" {
			if err := config.Call(
				"com.redhat.RHSM1.Config.Set",
				0,
				"server.hostname",
				URL.Path,
				locale).Err; err != nil {
				return unpackRHSMError(err)
			}
		}
	} else {
		if URL.Hostname() != "" {
			if err := config.Call(
				"com.redhat.RHSM1.Config.Set",
				0,
				"server.hostname",
				URL.Hostname(),
				locale).Err; err != nil {
				return unpackRHSMError(err)
			}
		}

		if URL.Port() != "" {
			if err := config.Call(
				"com.redhat.RHSM1.Config.Set",
				0,
				"server.port",
				URL.Port(),
				locale).Err; err != nil {
				return unpackRHSMError(err)
			}
		}

		if URL.Path != "" {
			if err := config.Call(
				"com.redhat.RHSM1.Config.Set",
				0,
				"server.prefix",
				URL.Path,
				locale).Err; err != nil {
				return unpackRHSMError(err)
			}
		}
	}

	return nil
}

func getRHSMConfigOption(name string, val interface{}) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}
	locale := getLocale()
	obj := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	if err := obj.Call(
		"com.redhat.RHSM1.Config.Get",
		dbus.Flags(0),
		name,
		locale).Store(val); err != nil {
		return unpackRHSMError(err)
	}

	return nil
}
