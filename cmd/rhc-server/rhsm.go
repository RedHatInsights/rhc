package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/varlink/contentapi"
	"github.com/redhatinsights/rhc/varlink/overrideapi"
)

type ClientError struct {
	Message string
}

func (e *ClientError) Error() string {
	return e.Message
}

type ServerError struct {
	Message string
}

func (e *ServerError) Error() string {
	return e.Message
}

// GetStatus retrieves the current status of the Red Hat Subscription Management (RHSM) server.
func GetStatus(ipcSender *string, locale *string, correlationID *string) (*rhsm2.RHSMStatus, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	// Create client information from provided parameters
	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	status, err := rhsmClient.GetServerStatus(&clientInfo)
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	return status, nil
}

// IsSystemRegistered checks if the system is registered with RHSM.
// When it is not possible to retrieve the consumer UUID, it returns false.
func IsSystemRegistered() (bool, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return false, &ClientError{Message: err.Error()}
	}

	_, err = rhsmClient.GetConsumerUUID()
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetContentOverrides retrieves content overrides from the candlepin server.
func GetContentOverrides(ipcSender *string, locale *string, correlationID *string) ([]rhsm2.ContentOverride, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	overrides, err := rhsmClient.GetContentOverrides(&clientInfo)
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	return overrides, nil
}

// UploadContentOverrides reads local DNF5 repo overrides and pushes them to the candlepin server.
func UploadContentOverrides(ipcSender *string, locale *string, correlationID *string) error {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return &ClientError{Message: err.Error()}
	}

	overrides, err := rhsm2.ReadLocalContentOverrides(rhsm2.Dnf5RedHatReposOverrideFilePath)
	if err != nil {
		return &ServerError{Message: fmt.Sprintf("failed to read local DNF5 repo overrides: %s", err)}
	}

	if len(overrides) == 0 {
		return nil
	}

	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	if err := rhsmClient.SendContentOverrides(overrides, &clientInfo); err != nil {
		return &ServerError{Message: err.Error()}
	}

	return nil
}

// WriteLocalContentOverrides writes content overrides to the local DNF5 repo override file.
func WriteLocalContentOverrides(overrides []rhsm2.ContentOverride) error {
	dir := filepath.Dir(rhsm2.Dnf5RedHatReposOverrideFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &overrideapi.IOFailedError{Message: fmt.Sprintf("failed to create override directory %s: %s", dir, err)}
	}
	if err := rhsm2.WriteDnf5RepoOverrides(overrides, rhsm2.Dnf5RedHatReposOverrideFilePath); err != nil {
		return &overrideapi.IOFailedError{Message: fmt.Sprintf("failed to write content overrides to local file: %s", err)}
	}
	return nil
}

// DownloadRelease downloads the current release information from the server. The release can be set
// on the candlepin server for the given system. The client has to have chance to get this information
// from the server.
func DownloadRelease(ipcSender *string, locale *string, correlationID *string) (*string, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	// Create client information from provided parameters
	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	// Try to get release from server
	release, err := rhsmClient.GetReleaseFromServer(&clientInfo)
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	return &release, nil
}

func GetAvailableReleases(ipcSender *string, locale *string, correlationID *string) (map[string]struct{}, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	// Create client information from provided parameters
	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	return rhsmClient.GetCdnReleases(&clientInfo)
}

func SetRelease(release string, ipcSender *string, locale *string, correlationID *string) error {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return &ClientError{Message: err.Error()}
	}
	// Create client information from provided parameters
	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	return rhsmClient.SetRelease(release, &clientInfo)
}

// RefreshContent tries to refresh the installed entitlement certificate, when needed,
// and regenerate the redhat.repo file accordingly. When force is true, the entitlement
// certificate is force recreated.
func RefreshContent(force bool, ipcSender *string, locale *string, correlationID *string) error {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return &ClientError{Message: err.Error()}
	}

	// Create client information from provided parameters
	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	// An up-to-date entitlement certificate is not a failure; the repo file is
	// still regenerated below.
	_, _, err = rhsmClient.UpdateEntitlementCertificate(force, &clientInfo)
	if _, upToDate := errors.AsType[*rhsm2.EntitlementCertificateUpToDateError](err); err != nil && !upToDate {
		return &ServerError{Message: err.Error()}
	}

	if err := rhsmClient.GenerateRepoFileFromInstalledEntitlementCerts(); err != nil {
		return &contentapi.IOFailedError{Message: fmt.Sprintf("failed to generate redhat.repo file: %s", err)}
	}

	return nil
}

// SendProfile gathers and uploads the combined RPM profile (installed RPMs and
// enabled repositories) to the candlepin server.
//
// The upload is performed synchronously (async=false): the Varlink request is
// itself the asynchronous boundary for callers, and the backend needs the
// result to report success/failure. The async mode of SendPackageProfile is for
// in-process callers such as the rhsm libdnf5 plugin, not this Varlink path.
//
// A *rhsm2.ProfileNotSupportedError is returned unchanged so the Varlink
// backend can distinguish "server does not support profile upload" from a
// generic failure; all other failures are wrapped as a ServerError.
func SendProfile(ipcSender *string, locale *string, correlationID *string) error {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return &ClientError{Message: err.Error()}
	}

	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	if err := rhsmClient.SendPackageProfile(false, &clientInfo); err != nil {
		if _, ok := errors.AsType[*rhsm2.ProfileNotSupportedError](err); ok {
			return err
		}
		return &ServerError{Message: err.Error()}
	}

	return nil
}
