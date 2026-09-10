package main

import (
	"fmt"
	"strings"

	"github.com/redhatinsights/rhc/pkg/operations"
)

type connectJSONFeature struct {
	Enabled    bool   `json:"enabled"`
	Successful bool   `json:"successful"`
	Error      string `json:"error,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
}

type connectJSONDocument struct {
	Hostname         string `json:"hostname"`
	HostnameError    string `json:"hostname_error,omitempty"`
	UID              int    `json:"uid"`
	UIDError         string `json:"uid_error,omitempty"`
	RHSMConnected    bool   `json:"rhsm_connected"`
	RHSMConnectError string `json:"rhsm_connect_error,omitempty"`
	Features         struct {
		Content          connectJSONFeature `json:"content"`
		Analytics        connectJSONFeature `json:"analytics"`
		RemoteManagement connectJSONFeature `json:"remote_management"`
	} `json:"features"`
}

func formatConnectRHSM(report operations.ConnectReport) {
	if report.RHSMConnected {
		formatConnectLine(indent.Small, icons.Ok, "Connected to Red Hat Subscription Management")
	} else {
		formatConnectLine(indent.Small, icons.Error, "Cannot connect to Red Hat Subscription Management")
		formatConnectLine(indent.Medium, icons.Error, "Skipping generation of Red Hat repository file")
	}
	switch {
	case report.Content.Successful:
		formatConnectLine(indent.Medium, icons.Ok, "Content ... System has access to content")
	case report.RHSMConnected:
		formatConnectLine(indent.Medium, "", "Content ... System has no access to content")
	}
}

func formatConnectFeature(f operations.ConnectFeatureResult, name, okMsg, errMsg string) {
	switch {
	case !f.Requested, f.Skipped && f.SkipDependency == "":
		formatConnectLine(indent.Medium, icons.Info, name+" ... Skipped")
	case f.Skipped:
		formatConnectLine(indent.Medium, icons.Warning,
			fmt.Sprintf("%s ... Skipped (dependency '%s' failed)", name, f.SkipDependency))
	case f.Successful:
		formatConnectLine(indent.Medium, icons.Ok, name+" ... "+okMsg)
	default:
		formatConnectLine(indent.Medium, icons.Error, name+" ... "+errMsg)
	}
}

func formatConnectLine(indent, icon, text string) {
	if icon == "" {
		printf("%s[ ] %s\n", indent, text)
		return
	}
	printf("%s[%v] %s\n", indent, icon, text)
}

func formatConnectStepsReport(report operations.ConnectReport) {
	formatConnectRHSM(report)
	formatConnectFeature(
		report.Analytics,
		"Analytics",
		"Connected to Red Hat Lightspeed (formerly Insights)",
		"Cannot connect to Red Hat Lightspeed (formerly Insights)",
	)
	formatConnectFeature(
		report.RemoteManagement,
		"Remote Management",
		"Activated the yggdrasil service",
		"Cannot activate the yggdrasil service",
	)
}

func connectJSONFeatureFrom(result operations.ConnectFeatureResult) connectJSONFeature {
	return connectJSONFeature{
		Enabled:    result.Enabled,
		Successful: result.Successful,
		Error:      result.Error,
		Skipped:    result.Skipped,
	}
}

func formatConnectJSON(report operations.ConnectReport) error {
	doc := connectJSONDocument{
		Hostname:         report.Hostname,
		HostnameError:    report.HostnameError,
		UID:              report.UID,
		UIDError:         report.UIDError,
		RHSMConnected:    report.RHSMConnected,
		RHSMConnectError: report.RHSMError,
	}
	doc.Features.Content = connectJSONFeatureFrom(report.Content)
	doc.Features.Analytics = connectJSONFeatureFrom(report.Analytics)
	doc.Features.RemoteManagement = connectJSONFeatureFrom(report.RemoteManagement)
	return printJSON(doc)
}

func formatConnectIgnoringPrefs() {
	printf("Notice: ignoring preferences set via 'rhc configure features'.\n\n")
}

func formatConnectHeader(hostname string, toEnable []string) {
	printf("Connecting %v to Red Hat.", hostname)
	if len(toEnable) > 0 {
		printf(" ")
		printf("Enabled features: %s.", strings.Join(toEnable, ", "))
	}
	printf("\nThis might take some time.\n\n")
}

func formatConnectSuccess() {
	printf("\nSuccessfully connected to Red Hat!\n")
}

func formatConnectFooter() {
	printf("\nManage your connected systems: https://red.ht/connector\n")
}
