package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/internal/collector"
	collectorapi "github.com/redhatinsights/rhc/varlink/collector"
	rhsmapi "github.com/redhatinsights/rhc/varlink/rhsm/testing"
	contentapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content"
	overrideapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content/override"
	releaseapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content/release"
)

// CollectorBackend implements the collectorapi.comredhatrhccollectorInterface interface.
type CollectorBackend struct{}

// NewCollectorBackend creates a new CollectorBackend instance.
func NewCollectorBackend() *CollectorBackend {
	return &CollectorBackend{}
}

// List implements the List method of the collector API.
// Returns a list of all available collectors with full details.
func (b *CollectorBackend) List(ctx context.Context, c collectorapi.VarlinkCall) error {
	// Get list of collector IDs
	collectorIDs, err := collector.GetCollectors()
	if err != nil {
		slog.Error("Failed to get collectors", "error", err)
		return fmt.Errorf("failed to get collectors: %w", err)
	}

	// Build the list of collectors with full details
	collectors := make([]collectorapi.CollectorInfo, 0, len(collectorIDs))
	for _, id := range collectorIDs {
		info, err := buildCollectorInfo(id)
		if err != nil {
			slog.Warn("Failed to build collector info, skipping it", "id", id, "error", err)
			continue
		}
		collectors = append(collectors, *info)
	}

	return c.ReplyList(ctx, collectors)
}

// Info implements the Info method of the collector API.
// Returns detailed information about a specific collector including timing and configuration.
func (b *CollectorBackend) Info(ctx context.Context, c collectorapi.VarlinkCall, id string) error {
	// Validate input parameter
	if _, err := collector.ValidateID(id); err != nil {
		return c.ReplyInvalidParameter(ctx, "id")
	}

	info, err := buildCollectorInfo(id)
	if err != nil {
		return c.ReplyNoSuchCollector(ctx, id)
	}

	return c.ReplyInfo(ctx, *info)
}

// RHSMBackend implements the rhsmapi.comredhatrhsmtestingInterface interface.
type RHSMBackend struct{}

// NewRHSMBackend creates a new RHSMBackend instance.
func NewRHSMBackend() *RHSMBackend {
	return &RHSMBackend{}
}

// Ping checks the status of the RHSM server.
func (b *RHSMBackend) Ping(ctx context.Context, c rhsmapi.VarlinkCall, metadata *rhsmapi.Metadata) error {
	var rhsmServerStatus *rhsm2.RHSMStatus
	var err error
	if metadata != nil {
		rhsmServerStatus, err = GetStatus(
			metadata.User_agent,
			metadata.Locale,
			metadata.Correlation_id,
		)
	} else {
		rhsmServerStatus, err = GetStatus(nil, nil, nil)
	}
	if err != nil {
		var typeClientErr *ClientError
		var typeServerErr *ServerError
		switch {
		case errors.As(err, &typeClientErr):
			return c.ReplyInvalidClientConnection(ctx, typeClientErr.Message)
		case errors.As(err, &typeServerErr):
			return c.ReplyFailedServerResponse(ctx, typeServerErr.Message)
		default:
			slog.Error("Failed to get RHSM status", "error", err)
			return c.ReplyFailedServerResponse(ctx, "failed to get status of the RHSM server")
		}
	}
	status, err := json.Marshal(rhsmServerStatus)
	if err != nil {
		slog.Error("Failed to marshal RHSM server status", "error", err)
		return c.ReplyFailedServerResponse(ctx, "failed to marshal status of the RHSM server")
	}
	return c.ReplyPing(ctx, status)
}

// IsRegistered checks if the system is registered with RHSM.
func (b *RHSMBackend) IsRegistered(ctx context.Context, c rhsmapi.VarlinkCall) error {
	registered, err := IsSystemRegistered()
	if err != nil {
		// When it is not possible to determine registration status, then log the reason
		// and return false
		slog.Debug("Failed to determine registration status", "error", err)
		return c.ReplyIsRegistered(ctx, false)
	}
	return c.ReplyIsRegistered(ctx, registered)
}

// ContentOverrideBackend implements the overrideapi.comredhatrhsmtestingcontentoverrideInterface interface.
type ContentOverrideBackend struct{}

// NewContentOverrideBackend creates a new ContentOverrideBackend instance.
func NewContentOverrideBackend() *ContentOverrideBackend {
	return &ContentOverrideBackend{}
}

// Download fetches content overrides from the candlepin server.
func (b *ContentOverrideBackend) Download(ctx context.Context, c overrideapi.VarlinkCall, metadata *overrideapi.Metadata) error {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return c.ReplySystemNotRegistered(ctx)
	}

	var overrides []rhsm2.ContentOverride

	if metadata != nil {
		overrides, err = GetContentOverrides(
			metadata.User_agent,
			metadata.Locale,
			metadata.Correlation_id,
		)
	} else {
		overrides, err = GetContentOverrides(nil, nil, nil)
	}
	if err != nil {
		slog.Error("Failed to download content overrides", "error", err)
		return replyRHSMError(ctx, &c, err)
	}

	result := make([]overrideapi.ContentOverride, 0, len(overrides))
	for _, o := range overrides {
		co := overrideapi.ContentOverride{
			Content_label: o.ContentLabel,
			Name:          o.Name,
			Value:         o.Value,
		}
		if o.Created != "" {
			created := o.Created
			co.Created = &created
		}
		if o.Updated != "" {
			updated := o.Updated
			co.Updated = &updated
		}
		result = append(result, co)
	}

	return c.ReplyDownload(ctx, result)
}

// WriteContentOverrides writes the given content overrides to the local DNF5 repo override file.
func (b *ContentOverrideBackend) WriteContentOverrides(ctx context.Context, c overrideapi.VarlinkCall, contentOverrides []overrideapi.ContentOverride) error {
	overrides := make([]rhsm2.ContentOverride, 0, len(contentOverrides))
	for _, co := range contentOverrides {
		overrides = append(overrides,
			rhsm2.ContentOverride{
				ContentLabel: co.Content_label,
				Name:         co.Name,
				Value:        co.Value,
			},
		)
	}

	if err := WriteLocalContentOverrides(overrides); err != nil {
		slog.Error("Failed to write content overrides to local DNF5 repo override file", "error", err)
		var ioErr *IOError
		if errors.As(err, &ioErr) {
			return c.ReplyIOFailed(ctx, ioErr.Message)
		}
		return err
	}

	return c.ReplyWriteContentOverrides(ctx, true)
}

// Upload reads local DNF5 repo overrides and sends them to the candlepin server.
func (b *ContentOverrideBackend) Upload(ctx context.Context, c overrideapi.VarlinkCall, metadata *overrideapi.Metadata) error {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return c.ReplySystemNotRegistered(ctx)
	}

	if metadata != nil {
		err = UploadContentOverrides(
			metadata.User_agent,
			metadata.Locale,
			metadata.Correlation_id,
		)
	} else {
		err = UploadContentOverrides(nil, nil, nil)
	}
	if err != nil {
		slog.Error("Failed to upload content overrides", "error", err)
		return replyRHSMError(ctx, &c, err)
	}

	return c.ReplyUpload(ctx, true)
}

// ComRedhatRhsmContentReleaseBackend implements the releaseapi.comredhatrhsmtestingcontentreleaseInterface interface
// for the com.redhat.rhsm.testing.content.release.varlink
type ComRedhatRhsmContentReleaseBackend struct{}

// NewComRedhatRhsmContentReleaseRHSMBackend creates a new ComRedhatRhsmContentReleaseBackend instance.
func NewComRedhatRhsmContentReleaseRHSMBackend() *ComRedhatRhsmContentReleaseBackend {
	return &ComRedhatRhsmContentReleaseBackend{}
}

// Download implements the interface for the com.redhat.rhsm.testing.content.release.varlink.
// This method downloads the release from the server and returns it. It does not have any side effects.
// The release is not written to disk. If the client application wants to write the release to disk, then
// another Varlink method SetRelease() should be used for this purpose.
func (b ComRedhatRhsmContentReleaseBackend) Download(ctx context.Context, c releaseapi.VarlinkCall, metadata *releaseapi.Metadata) error {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return c.ReplySystemNotRegistered(ctx)
	}

	var release *string
	if metadata != nil {
		release, err = DownloadRelease(
			metadata.User_agent,
			metadata.Locale,
			metadata.Correlation_id,
		)
	} else {
		release, err = DownloadRelease(nil, nil, nil)
	}

	if err != nil {
		return replyRHSMError(ctx, &c, err)
	}

	return c.ReplyDownload(ctx, *release)
}

// GetAvailableReleases retrieves a list of available releases and sorts them alphabetically.
func (b ComRedhatRhsmContentReleaseBackend) GetAvailableReleases(ctx context.Context, c releaseapi.VarlinkCall, metadata *releaseapi.Metadata) error {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return c.ReplySystemNotRegistered(ctx)
	}

	var releases map[string]struct{}
	if metadata != nil {
		releases, err = GetAvailableReleases(metadata.User_agent, metadata.Locale, metadata.Correlation_id)
	} else {
		releases, err = GetAvailableReleases(nil, nil, nil)
	}

	if err != nil {
		var noProductCertInstalledError *rhsm2.NoProductCertInstalledError
		var noEntitlementCertInstalledError *rhsm2.NoEntitlementCertInstalledError
		var noInstalledProductCertMatchesOsReleaseError *rhsm2.NoInstalledProductCertMatchesOsReleaseError
		switch {
		case errors.As(err, &noProductCertInstalledError):
			return c.ReplyNoProductCertificateInstalled(ctx)
		case errors.As(err, &noEntitlementCertInstalledError):
			return c.ReplyNoEntitlementCertificateInstalled(ctx)
		case errors.As(err, &noInstalledProductCertMatchesOsReleaseError):
			return c.ReplyNoInstalledProductCertificateMatchesOsRelease(
				ctx,
				noInstalledProductCertMatchesOsReleaseError.OsReleaseTag,
				noInstalledProductCertMatchesOsReleaseError.NotMatchingTags,
			)
		default:
			return replyRHSMError(ctx, &c, err)
		}
	}

	// Convert map of structure (workaround for absence of set in Go) to array of strings
	// and sort it alphabetically.
	releaseList := make([]string, 0, len(releases))
	for release := range releases {
		releaseList = append(releaseList, release)
	}
	slices.Sort(releaseList)

	return c.ReplyGetAvailableReleases(ctx, releaseList)
}

// GetCurrentRelease returns the current release.
func (b ComRedhatRhsmContentReleaseBackend) GetCurrentRelease(ctx context.Context, c releaseapi.VarlinkCall) error {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return replyRHSMError(ctx, &c, &ClientError{Message: err.Error()})
	}

	release, err := rhsmClient.GetDnfVarsRelease()
	if err != nil {
		return replyRHSMError(ctx, &c, &ServerError{Message: err.Error()})
	}

	// When release is empty string, return nil pointer
	if release == "" {
		return c.ReplyGetCurrentRelease(ctx, nil)
	}

	return c.ReplyGetCurrentRelease(ctx, &release)
}

// SetRelease sets the current release and it tries to send release to the candlepin server.
// Sending of release to the candlepin server is done asynchronously.
func (b ComRedhatRhsmContentReleaseBackend) SetRelease(ctx context.Context, c releaseapi.VarlinkCall, release string, metadata *releaseapi.Metadata) error {
	var err error
	if metadata != nil {
		err = SetRelease(release, metadata.User_agent, metadata.Locale, metadata.Correlation_id)
	} else {
		err = SetRelease(release, nil, nil, nil)
	}
	if err != nil {
		return replyRHSMError(ctx, &c, &ServerError{Message: err.Error()})
	}

	return c.ReplySetRelease(ctx, true)
}

// UnsetRelease unsets the current release and it tries to send release to the candlepin server.
// Unsetting the release is done asynchronously.
func (b ComRedhatRhsmContentReleaseBackend) UnsetRelease(ctx context.Context, c releaseapi.VarlinkCall, metadata *releaseapi.Metadata) error {
	var err error
	if metadata != nil {
		err = SetRelease("", metadata.User_agent, metadata.Locale, metadata.Correlation_id)
	} else {
		err = SetRelease("", nil, nil, nil)
	}
	if err != nil {
		return replyRHSMError(ctx, &c, &ServerError{Message: err.Error()})
	}

	return c.ReplyUnsetRelease(ctx, true)
}

// ComRedhatRhsmContentBackend implements the contentapi.comredhatrhsmtestingcontentInterface interface
// for the com.redhat.rhsm.testing.content.varlink
type ComRedhatRhsmContentBackend struct{}

// NewComRedhatRhsmContentBackend creates a new ComRedhatRhsmContentBackend instance.
func NewComRedhatRhsmContentBackend() *ComRedhatRhsmContentBackend {
	return &ComRedhatRhsmContentBackend{}
}

// Refresh implements the interface for the com.redhat.rhsm.testing.content.varlink.
// It refreshes the installed entitlement certificate, when needed, and regenerates
// the redhat.repo file accordingly.
func (b ComRedhatRhsmContentBackend) Refresh(ctx context.Context, c contentapi.VarlinkCall, force bool, metadata *contentapi.Metadata) error {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return c.ReplySystemNotRegistered(ctx)
	}

	if metadata != nil {
		err = RefreshContent(force, metadata.User_agent, metadata.Locale, metadata.Correlation_id)
	} else {
		err = RefreshContent(force, nil, nil, nil)
	}
	if err != nil {
		slog.Error("Failed to refresh content", "error", err)
		var ioErr *IOError
		if errors.As(err, &ioErr) {
			return c.ReplyIOFailed(ctx, ioErr.Message)
		}
		return replyRHSMError(ctx, &c, err)
	}

	return c.ReplyRefresh(ctx, true)
}
