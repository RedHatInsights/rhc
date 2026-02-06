package rhsm

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

const EnvTypeContentTemplate = "content-template"

// GetConsumerUUID returns the consumer uuid set on the system.
// If consumer uuid is not set, then an empty string is returned
func GetConsumerUUID() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	locale := localization.GetLocale()

	var uuid string
	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Consumer").Call(
		"com.redhat.RHSM1.Consumer.GetUuid",
		dbus.Flags(0),
		locale).Store(&uuid)
	if err != nil {
		return "", UnpackDBusError(err)
	}

	return uuid, nil
}

// IsSystemRegistered returns true when consumer uuid is set on the system,
// indicating the system is connected to Red Hat Subscription Management
func IsSystemRegistered() (bool, error) {
	uuid, err := GetConsumerUUID()
	if err != nil {
		return false, err
	}
	if uuid != "" {
		return true, nil
	}
	return false, nil
}

// Unregister unregisters the system from Red Hat Subscription Management
// using the RHSM D-Bus API
func Unregister() error {
	isRegistered, err := IsSystemRegistered()
	if err != nil {
		return err
	}
	if !isRegistered {
		return fmt.Errorf("warning: the system is already unregistered")
	}

	locale := localization.GetLocale()

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	slog.Debug("Calling method com.redhat.RHSM1.Unregister.Unregister")
	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Unregister").Call(
		"com.redhat.RHSM1.Unregister.Unregister",
		dbus.Flags(0),
		map[string]string{},
		locale).Err
	if err != nil {
		return UnpackDBusError(err)
	}

	return nil
}

// IsContentEnabled returns true when manage_repos is set to 1 in the RHSM config
func IsContentEnabled() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}

	locale := localization.GetLocale()

	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	slog.Debug("Calling method com.redhat.RHSM1.Config.Get")
	var contentEnabled string
	err = config.Call(
		"com.redhat.RHSM1.Config.Get",
		0,
		"rhsm.manage_repos",
		locale).Store(&contentEnabled)
	if err != nil {
		return false, UnpackDBusError(err)
	}

	return contentEnabled == "1", nil
}

type RegisterServer struct {
	privConn *dbus.Conn
}

// NewRegisterServer opens a private D-Bus UNIX socket for RHSM registration
func NewRegisterServer() (*RegisterServer, error) {
	privateDbusSocketURI, err := startRegisterServer()
	if err != nil {
		return nil, err
	}

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return nil, err
	}

	err = privConn.Auth(nil)
	if err != nil {
		return nil, err
	}

	return &RegisterServer{privConn: privConn}, nil
}

// startRegisterServer starts a private D-Bus UNIX socket to be used for RHSM registration
// and returns its URI
func startRegisterServer() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")
	locale := localization.GetLocale()

	slog.Debug("Calling method com.redhat.RHSM1.RegisterServer.Start")
	var privateDbusSocketURI string
	err = registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Start",
		dbus.Flags(0),
		locale,
	).Store(&privateDbusSocketURI)
	if err != nil {
		return "", UnpackDBusError(err)
	}

	return privateDbusSocketURI, nil
}

// Close closes the private D-Bus connection
func (r *RegisterServer) Close() error {
	slog.Debug("Closing connection to private D-Bus UNIX socket")
	err1 := r.privConn.Close()
	if err1 != nil {
		slog.Debug("Error closing connection to private D-Bus UNIX socket", "err", err1)
	}

	slog.Debug("Closing private D-Bus UNIX socket")
	err2 := stopRegisterServer()
	if err2 != nil {
		slog.Debug("Error closing private D-Bus UNIX socket", "err", err2)
	}

	return errors.Join(err1, err2)
}

// stopRegisterServer closes the private D-Bus UNIX socket used for RHSM registration
func stopRegisterServer() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	slog.Debug("Calling method com.redhat.RHSM1.RegisterServer.Stop")
	err = registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Stop",
		dbus.FlagNoReplyExpected,
		localization.GetLocale()).Err
	if err != nil {
		return UnpackDBusError(err)
	}
	return nil
}

// RegisterWithUsernamePassword tries to register system against candlepin server (Red Hat Management Service)
// username and password are mandatory. When organization is not obtained, then this method
// returns a DBus error with the exception type "OrgNotSpecifiedException"
func (r *RegisterServer) RegisterWithUsernamePassword(username, password, organization string, environments []string, enableContent bool) error {
	options := make(map[string]string)
	if len(environments) != 0 {
		options["environment_names"] = strings.Join(environments, ",")
		options["environment_type"] = EnvTypeContentTemplate
	}
	options["enable_content"] = fmt.Sprintf("%v", enableContent)

	slog.Debug("Calling method com.redhat.RHSM1.Register.Register")
	err := r.privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register").Call(
		"com.redhat.RHSM1.Register.Register",
		dbus.Flags(0),
		organization,
		username,
		password,
		options,
		map[string]string{},
		localization.GetLocale(),
	).Err

	if err != nil {
		// Try to unpack D-Bus error
		return UnpackDBusError(err)
	}

	return nil
}

// RegisterWithActivationKeys tries to register system against candlepin server (Red Hat Management Service)
// with the provided activation keys.
func (r *RegisterServer) RegisterWithActivationKeys(orgID string, activationKeys []string, environments []string, enableContent bool) error {
	options := make(map[string]string)
	if len(environments) != 0 {
		options["environment_names"] = strings.Join(environments, ",")
		options["environment_type"] = EnvTypeContentTemplate
	}
	options["enable_content"] = fmt.Sprintf("%v", enableContent)

	slog.Debug("Calling method com.redhat.RHSM1.Register.RegisterWithActivationKeys")
	err := r.privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register").Call(
		"com.redhat.RHSM1.Register.RegisterWithActivationKeys",
		dbus.Flags(0),
		orgID,
		activationKeys,
		options,
		map[string]string{},
		localization.GetLocale(),
	).Err

	if err != nil {
		return UnpackDBusError(err)
	}

	return nil
}

// GetOrganizations tries to retrieve a list of organizations from candlepin server (Red Hat Management Service)
func (r *RegisterServer) GetOrganizations(username string, password string) ([]string, error) {
	locale := localization.GetLocale()

	slog.Debug("Calling method com.redhat.RHSM1.Register.GetOrgs")
	var s string
	err := r.privConn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Register",
	).Call(
		"com.redhat.RHSM1.Register.GetOrgs",
		dbus.Flags(0),
		username,
		password,
		map[string]string{},
		locale,
	).Store(&s)

	if err != nil {
		return nil, UnpackDBusError(err)
	}

	return unpackOrgs(s)
}

// organization is structure containing information about RHSM organization (sometimes called owner)
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
type organization struct {
	Key string `json:"key"`
}

// unpackOrgs tries to unpack list organization from JSON document returned by D-Bus method GetOrgs.
// When it is possible to unmarshal the JSON document, then return list of organization keys (IDs).
// When it is not possible to get list of organizations, then return empty slice and error.
func unpackOrgs(s string) ([]string, error) {
	var orgs []string

	var organizations []organization

	err := json.Unmarshal([]byte(s), &organizations)
	if err != nil {
		return orgs, err
	}

	for _, org := range organizations {
		orgs = append(orgs, org.Key)
	}

	return orgs, nil
}

// DBusError is used for parsing JSON document returned by D-Bus methods.
type DBusError struct {
	Exception string `json:"exception"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// Error returns textual representation of DBusError. This implements all necessary
// methods for the error interface. Thus, DBusError can be handled as a regular error.
func (dbusError DBusError) Error() string {
	return fmt.Sprintf("%v: %v", dbusError.Severity, dbusError.Message)
}

// IsOrgNotSpecified checks if an error is a DBusError with exception type OrgNotSpecifiedException.
// OrgNotSpecifiedException is returned when the user must specify an organization
// but did not provide one. This typically occurs when the user has access to
// multiple organizations.
func IsOrgNotSpecified(err error) bool {
	if err == nil {
		return false
	}
	dbusErr, ok := err.(DBusError)
	return ok && dbusErr.Exception == "OrgNotSpecifiedException"
}

// UnpackDBusError tries to unpack a JSON document (part of the error) into the structure DBusError. When it is
// not possible to parse an error into structure, then a corresponding or original error is returned.
// When it is possible to parse error into structure, then DBusError is returned
func UnpackDBusError(err error) error {
	dbusError := DBusError{}
	switch e := err.(type) {
	case dbus.Error:
		if e.Name == "com.redhat.RHSM1.Error" {
			if jsonErr := json.Unmarshal([]byte(e.Error()), &dbusError); jsonErr != nil {
				return jsonErr
			}
			return dbusError
		}
		return err
	}
	return err
}
