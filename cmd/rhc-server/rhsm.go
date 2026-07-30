package main

import (
	"fmt"

	"github.com/jirihnidek/rhsm2"
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
	appName := AppName
	rhsmClient, err := rhsm2.GetRHSMClient(&appName, nil)
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
	appName := AppName
	rhsmClient, err := rhsm2.GetRHSMClient(&appName, nil)
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
	appName := AppName
	rhsmClient, err := rhsm2.GetRHSMClient(&appName, nil)
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
	appName := AppName
	rhsmClient, err := rhsm2.GetRHSMClient(&appName, nil)
	if err != nil {
		return &ClientError{Message: err.Error()}
	}

	overrides, err := rhsm2.ReadLocalContentOverrides(rhsm2.Dnf5RedHatReposOverrideFilePath)
	if err != nil {
		return &ServerError{Message: fmt.Sprintf("failed to read local DNF5 repo overrides: %s", err)}
	}

	clientInfo := rhsm2.RequestMetadata{IPCSender: ipcSender, Locale: locale, CorrelationId: correlationID}
	if err := rhsmClient.SendContentOverrides(overrides, &clientInfo); err != nil {
		return &ServerError{Message: err.Error()}
	}

	return nil
}
