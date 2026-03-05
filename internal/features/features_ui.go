package features

import (
	"fmt"
	"log/slog"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/redhatinsights/rhc/internal/ui"
)

type FeatureResult struct {
	Enabled    *bool  `json:"enabled"`
	Successful *bool  `json:"successful"`
	Error      string `json:"error,omitempty"`
}

type FeaturesResults struct {
	Content          FeatureResult `json:"content"`
	Analytics        FeatureResult `json:"analytics"`
	RemoteManagement FeatureResult `json:"remote_management"`
}

// BoolPtr is a helper function that converts a bool
// to a bool pointer
func BoolPtr(b bool) *bool {
	return &b
}

// TryEnableContent will attempt to enable content management.
// If this fails, then Features.Content.Successful will be set to false, and the
// error message will be stored in Features.Content.Error. This method is used
// during `rhc connect` and during `rhc configuration feature enable content`.
func (featureResults *FeaturesResults) TryEnableContent(wanted bool) {
	if !wanted {
		featureResults.Content.Enabled = BoolPtr(false)
		featureResults.Content.Successful = BoolPtr(false)
		slog.Info("Content management disabled (content feature disabled)")
		ui.Printf(
			"%s[ ] Skipping content management\n",
			ui.Indent.Medium,
		)
		return
	}
	featureResults.Content.Enabled = BoolPtr(true)

	if !rhsm.IsRegistered() {
		slog.Warn("Skipping enabling content (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping enabling content\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	slog.Info("Enabling content management")
	err := ui.Spinner(
		rhsm.EnableContentManagement,
		ui.Indent.Medium,
		"Enabling content management...")
	if err != nil {
		featureResults.Content.Successful = BoolPtr(false)
		featureResults.Content.Error = fmt.Sprintf("cannot enable content management: %v", err)
		slog.Error(featureResults.Content.Error)
		ui.Printf(
			"%s[%v] Content ... Cannot enable content\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.Content.Successful = BoolPtr(true)
	infoMsg := "Red Hat repository file generated"
	slog.Info(infoMsg)
	ui.Printf("%s[%v] Content ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
}

// TryDisableContent will attempt to disable content management.
// If this fails, then Features.Content.Successful will be set to false, and the
// error message will be stored in Features.Content.Error. This method is used
// during `rhc connect` and during `rhc configuration feature disable content`.
func (featureResults *FeaturesResults) TryDisableContent() {
	featureResults.Content.Enabled = BoolPtr(false)
	if !rhsm.IsRegistered() {
		slog.Warn("Skipping disabling content (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping disabling content\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	slog.Info("Disabling content management")
	err := ui.Spinner(
		rhsm.DisableContentManagement,
		ui.Indent.Medium,
		"Disabling content management...")
	if err != nil {
		featureResults.Content.Successful = BoolPtr(false)
		featureResults.Content.Error = fmt.Sprintf("cannot disable content management: %v", err)
		slog.Error(featureResults.Content.Error)
		ui.Printf(
			"%s[%v] Content ... Cannot disable content\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.Content.Successful = BoolPtr(true)
	infoMsg := "Red Hat repository file deleted"
	slog.Info(infoMsg)
	ui.Printf("%s[ ] Content ... %s\n", ui.Indent.Medium, infoMsg)
}

// TryRegisterInsightsClient will attempt to register the system with Red Hat Lightspeed.
// If this fails, then Features.Analytics.Successful will be set to false, and the
// error message will be stored in Features.Analytics.Error.
func (featureResults *FeaturesResults) TryRegisterInsightsClient(wanted bool, reasons *string) {
	if !wanted {
		featureResults.Analytics.Enabled = BoolPtr(false)
		featureResults.Analytics.Successful = BoolPtr(false)
		slog.Info("The Red Hat Lightspeed (formerly Insights) disabled (analytics feature disabled)")
		ui.Printf("%s[ ] Analytics ... Connecting to Red Hat Lightspeed (formerly Insights) disabled\n",
			ui.Indent.Medium)
		return
	}
	featureResults.Analytics.Enabled = BoolPtr(true)

	if !rhsm.IsRegistered() {
		slog.Warn("Skipping connection to Red Hat Lightspeed (formerly Insights) (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping connection to Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	var infoMsg string
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Connecting to Red Hat Lightspeed (formerly Insights) (%s)", *reasons)
	} else {
		infoMsg = "Connecting to Red Hat Lightspeed (formerly Insights)"
	}
	slog.Info(infoMsg)
	err := ui.Spinner(datacollection.RegisterInsightsClient, ui.Indent.Medium,
		fmt.Sprintf("%s ...", infoMsg))
	if err != nil {
		featureResults.Analytics.Successful = BoolPtr(false)
		featureResults.Analytics.Error = fmt.Sprintf("cannot connect to Red Hat Lightspeed (formerly Insights): %v", err)
		slog.Error(featureResults.Analytics.Error)
		ui.Printf(
			"%s[%v] Analytics ... Cannot connect to Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.Analytics.Successful = BoolPtr(true)
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Connected to Red Hat Lightspeed (formerly Insights) (%s)", *reasons)
	} else {
		infoMsg = "Connected to Red Hat Lightspeed (formerly Insights)"
	}
	slog.Info(infoMsg)
	ui.Printf("%s[%v] Analytics ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
}

// TryUnRegisterInsightsClient will attempt to unregister the system from Red Hat Lightspeed.
// If this fails, then Features.Analytics.Successful will be set to false, and the
// error message will be stored in Features.Analytics.Error.
func (featureResults *FeaturesResults) TryUnRegisterInsightsClient(reasons *string) {
	featureResults.Analytics.Enabled = BoolPtr(false)
	if !rhsm.IsRegistered() {
		slog.Warn("Skipping disconnection from Red Hat Lightspeed (formerly Insights) (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping disconnection from Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	var infoMsg string
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Disconnecting from Red Hat Lightspeed (formerly Insights) (%s)", *reasons)
	} else {
		infoMsg = "Disconnecting from Red Hat Lightspeed (formerly Insights)"
	}
	slog.Info(infoMsg)
	err := ui.Spinner(datacollection.UnregisterInsightsClient,
		ui.Indent.Medium,
		fmt.Sprintf("%s ...", infoMsg),
	)
	if err != nil {
		featureResults.Analytics.Successful = BoolPtr(false)
		featureResults.Analytics.Error = fmt.Sprintf("cannot disconnect from Red Hat Lightspeed (formerly Insights): %v", err)
		slog.Error(featureResults.Analytics.Error)
		ui.Printf(
			"%s[%v] Analytics ... Cannot disconnect from Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.Analytics.Successful = BoolPtr(true)
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Disconnected from Red Hat Lightspeed (formerly Insights) (%s)", *reasons)
	} else {
		infoMsg = "Disconnected from Red Hat Lightspeed (formerly Insights)"
	}
	slog.Info(infoMsg)
	ui.Printf("%s[ ] Analytics ... %s\n", ui.Indent.Medium, infoMsg)
}

// TryActivateServices will attempt to activate the yggdrasil service.
// If this fails, then Features.RemoteManagement.Successful will be set to false, and the
// error message will be stored in Features.RemoteManagement.Error.
func (featureResults *FeaturesResults) TryActivateServices(wanted bool, reasons *string) {
	if !wanted {
		featureResults.RemoteManagement.Enabled = BoolPtr(false)
		featureResults.RemoteManagement.Successful = BoolPtr(false)
		var infoMsg string
		if reasons != nil && *reasons != "" {
			infoMsg = fmt.Sprintf("Starting yggdrasil service disabled (%s)", *reasons)
		} else {
			infoMsg = "Starting yggdrasil service disabled"
		}
		ui.Printf("%s[ ] Management ... %s\n", ui.Indent.Medium, infoMsg)
		slog.Info(infoMsg)
		return
	}
	featureResults.RemoteManagement.Enabled = BoolPtr(true)

	if !rhsm.IsRegistered() {
		featureResults.RemoteManagement.Successful = BoolPtr(false)
		slog.Warn("Skipping activation of yggdrasil service (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping activation of yggdrasil service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	slog.Info("Activating yggdrasil service")
	progressMessage := " Activating the yggdrasil service"
	err := ui.Spinner(remotemanagement.ActivateServices, ui.Indent.Medium, progressMessage)
	if err != nil {
		featureResults.RemoteManagement.Successful = BoolPtr(false)
		featureResults.RemoteManagement.Error = fmt.Sprintf("cannot activate the yggdrasil service: %v", err)
		slog.Error(featureResults.RemoteManagement.Error)
		ui.Printf(
			"%s[%v] Remote Management ... Cannot activate the yggdrasil service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.RemoteManagement.Successful = BoolPtr(true)
	var infoMsg string
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Activated the yggdrasil service (%s)", *reasons)
	} else {
		infoMsg = "Activated the yggdrasil service"
	}
	slog.Info(infoMsg)
	ui.Printf("%s[%v] Remote Management ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
}

// TryDeactivateServices will attempt to deactivate the yggdrasil service.
// If this fails, then Features.RemoteManagement.Successful will be set to false, and the
// error message will be stored in Features.RemoteManagement.Error.
func (featureResults *FeaturesResults) TryDeactivateServices(reasons *string) {
	featureResults.RemoteManagement.Enabled = BoolPtr(false)
	if !rhsm.IsRegistered() {
		featureResults.RemoteManagement.Successful = BoolPtr(false)
		slog.Warn("Skipping deactivation of yggdrasil service (not RHSM registered)")
		ui.Printf(
			"%s[%v] Skipping deactivation of yggdrasil service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	slog.Info("Deactivating yggdrasil service")
	var progressMessage string
	if reasons != nil && *reasons != "" {
		progressMessage = fmt.Sprintf(" Deactivating the yggdrasil service (%s)", *reasons)
	} else {
		progressMessage = " Deactivating the yggdrasil service"
	}
	err := ui.Spinner(remotemanagement.DeactivateServices, ui.Indent.Medium, progressMessage)
	if err != nil {
		featureResults.RemoteManagement.Successful = BoolPtr(false)
		featureResults.RemoteManagement.Error = fmt.Sprintf("cannot deactivate the yggdrasil service: %v", err)
		slog.Error(featureResults.RemoteManagement.Error)
		ui.Printf(
			"%s[%v] Remote Management ... Cannot deactivate the yggdrasil service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	featureResults.RemoteManagement.Successful = BoolPtr(true)
	var infoMsg string
	if reasons != nil && *reasons != "" {
		infoMsg = fmt.Sprintf("Deactivated the yggdrasil service (%s)", *reasons)
	} else {
		infoMsg = "Deactivated the yggdrasil service"
	}
	slog.Info(infoMsg)
	ui.Printf("%s[ ] Remote Management ... %s\n", ui.Indent.Medium, infoMsg)
}
