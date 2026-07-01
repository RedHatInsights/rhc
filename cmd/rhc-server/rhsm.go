package main

import (
	"fmt"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/internal/subman"
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
	return subman.IsRegistered()
}

// GetConsumerUUID retrieves the consumer UUID from the installed consumer certificate.
func GetConsumerUUID() (*string, error) {
	uuid, err := subman.GetConsumerUUID()
	if err != nil {
		return nil, err
	}
	if uuid == "" {
		return nil, fmt.Errorf("system is not registered")
	}
	return &uuid, nil
}

// GetConsumerOrganization retrieves the organization for the registered consumer.
func GetConsumerOrganization() (*subman.Organization, error) {
	return subman.GetOrganization()
}

// GetConsumerEnvironments retrieves the environments from the registered consumer.
func GetConsumerEnvironments() ([]subman.Environment, error) {
	return subman.GetEnvironments()
}
