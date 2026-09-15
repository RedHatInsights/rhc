package main

import (
	"fmt"
	"strings"

	"github.com/redhatinsights/rhc/internal/ui"
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
		formatConnectLine(ui.Indent.Small, ui.Icons.Ok, "Connected to Red Hat Subscription Management")
	} else {
		formatConnectLine(ui.Indent.Small, ui.Icons.Error, "Cannot connect to Red Hat Subscription Management")
		formatConnectLine(ui.Indent.Medium, ui.Icons.Error, "Skipping generation of Red Hat repository file")
	}
	switch {
	case report.Content.Successful:
		formatConnectLine(ui.Indent.Medium, ui.Icons.Ok, "Content ... System has access to content")
	case report.RHSMConnected:
		formatConnectLine(ui.Indent.Medium, "", "Content ... System has no access to content")
	}
}

func formatConnectFeature(f operations.ConnectFeatureResult, name, okMsg, errMsg string) {
	switch {
	case !f.Requested, f.Skipped && f.SkipDependency == "":
		formatConnectLine(ui.Indent.Medium, ui.Icons.Info, name+" ... Skipped")
	case f.Skipped:
		formatConnectLine(ui.Indent.Medium, ui.Icons.Warning,
			fmt.Sprintf("%s ... Skipped (dependency '%s' failed)", name, f.SkipDependency))
	case f.Successful:
		formatConnectLine(ui.Indent.Medium, ui.Icons.Ok, name+" ... "+okMsg)
	default:
		formatConnectLine(ui.Indent.Medium, ui.Icons.Error, name+" ... "+errMsg)
	}
}

func formatConnectLine(indent, icon, text string) {
	if icon == "" {
		ui.Printf("%s[ ] %s\n", indent, text)
		return
	}
	ui.Printf("%s[%v] %s\n", indent, icon, text)
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
	return ui.PrintJSON(doc)
}

func formatConnectIgnoringPrefs() {
	ui.Printf("Notice: ignoring preferences set via 'rhc configure features'.\n\n")
}

func formatConnectHeader(hostname string, toEnable []string) {
	ui.Printf("Connecting %v to Red Hat.", hostname)
	if len(toEnable) > 0 {
		ui.Printf(" ")
		ui.Printf("Enabled features: %s.", strings.Join(toEnable, ", "))
	}
	ui.Printf("\nThis might take some time.\n\n")
}

func formatConnectSuccess() {
	ui.Printf("\nSuccessfully connected to Red Hat!\n")
}

func formatConnectFooter() {
	ui.Printf("\nManage your connected systems: https://red.ht/connector\n")
}
