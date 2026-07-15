package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
	"github.com/redhatinsights/rhc/varlink/overrideapi"
	"github.com/redhatinsights/rhc/varlink/releaseapi"
	"github.com/redhatinsights/rhc/varlink/rhsmapi"
)

// CollectorBackend implements the collectorapi.Backend interface.
type CollectorBackend struct{}

// NewCollectorBackend creates a new CollectorBackend instance.
func NewCollectorBackend() *CollectorBackend {
	return &CollectorBackend{}
}

// List implements the List method of the collector API.
// Returns a list of all available collectors with full details.
func (b *CollectorBackend) List(_ *collectorapi.ListIn) (*collectorapi.ListOut, error) {
	// Get list of collector IDs
	collectorIDs, err := collector.GetCollectors()
	if err != nil {
		return nil, fmt.Errorf("failed to get collectors: %w", err)
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

	return &collectorapi.ListOut{Collectors: collectors}, nil
}

// Info implements the Info method of the collector API.
// Returns detailed information about a specific collector including timing and configuration.
func (b *CollectorBackend) Info(in *collectorapi.InfoIn) (*collectorapi.InfoOut, error) {
	// Validate input parameter
	if _, err := collector.ValidateID(in.Id); err != nil {
		return nil, &collectorapi.InvalidParameterError{
			Parameter: "id",
		}
	}

	info, err := buildCollectorInfo(in.Id)
	if err != nil {
		return nil, &collectorapi.NoSuchCollectorError{
			Id: in.Id,
		}
	}

	return &collectorapi.InfoOut{Info: *info}, nil
}

// RHSMBackend implements the rhsmapi.Backend interface.
type RHSMBackend struct{}

// NewRHSMBackend creates a new RHSMBackend instance.
func NewRHSMBackend() *RHSMBackend {
	return &RHSMBackend{}
}

// Ping checks the status of the RHSM server.
func (b *RHSMBackend) Ping(in *rhsmapi.PingIn) (*rhsmapi.PingOut, error) {
	var rhsmServerStatus *rhsm2.RHSMStatus
	var err error
	if in.Metadata != nil {
		rhsmServerStatus, err = GetStatus(
			in.Metadata.UserAgent,
			in.Metadata.Locale,
			in.Metadata.CorrelationId,
		)
	} else {
		rhsmServerStatus, err = GetStatus(nil, nil, nil)
	}
	if err != nil {
		var typeClientErr *ClientError
		var typeServerErr *ServerError
		switch {
		case errors.As(err, &typeClientErr):
			return nil, &rhsmapi.InvalidClientConnectionError{Message: typeClientErr.Message}
		case errors.As(err, &typeServerErr):
			return nil, &rhsmapi.FailedServerResponseError{Message: typeServerErr.Message}
		default:
			slog.Error("Failed to get RHSM status", "error", err)
			return nil, err
		}
	}
	status, err := json.Marshal(rhsmServerStatus)
	if err != nil {
		return nil, &rhsmapi.FailedServerResponseError{Message: err.Error()}
	}
	return &rhsmapi.PingOut{Status: status}, nil
}

// IsRegistered checks if the system is registered with RHSM.
func (b *RHSMBackend) IsRegistered(in *rhsmapi.IsRegisteredIn) (*rhsmapi.IsRegisteredOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil {
		// When it is not possible to determine registration status, then log the reason
		// and return false
		slog.Debug("Failed to determine registration status", "error", err)
		return &rhsmapi.IsRegisteredOut{Registered: false}, nil
	}
	return &rhsmapi.IsRegisteredOut{Registered: registered}, nil
}

// ContentOverrideBackend implements the overrideapi.Backend interface.
type ContentOverrideBackend struct{}

// NewContentOverrideBackend creates a new ContentOverrideBackend instance.
func NewContentOverrideBackend() *ContentOverrideBackend {
	return &ContentOverrideBackend{}
}

// Download fetches content overrides from the candlepin server.
func (b *ContentOverrideBackend) Download(in *overrideapi.DownloadIn) (*overrideapi.DownloadOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return nil, &overrideapi.SystemNotRegisteredError{}
	}

	var overrides []rhsm2.ContentOverride

	if in.Metadata != nil {
		overrides, err = GetContentOverrides(
			in.Metadata.UserAgent,
			in.Metadata.Locale,
			in.Metadata.CorrelationId,
		)
	} else {
		overrides, err = GetContentOverrides(nil, nil, nil)
	}
	if err != nil {
		slog.Error("Failed to download content overrides", "error", err)
		return nil, err
	}

	result := make([]overrideapi.ContentOverride, 0, len(overrides))
	for _, o := range overrides {
		co := overrideapi.ContentOverride{
			ContentLabel: o.ContentLabel,
			Name:         o.Name,
			Value:        o.Value,
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

	return &overrideapi.DownloadOut{ContentOverrides: result}, nil
}

// Upload reads local DNF5 repo overrides and sends them to the candlepin server.
func (b *ContentOverrideBackend) Upload(in *overrideapi.UploadIn) (*overrideapi.UploadOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return nil, &overrideapi.SystemNotRegisteredError{}
	}

	if in.Metadata != nil {
		err = UploadContentOverrides(
			in.Metadata.UserAgent,
			in.Metadata.Locale,
			in.Metadata.CorrelationId,
		)
	} else {
		err = UploadContentOverrides(nil, nil, nil)
	}
	if err != nil {
		slog.Error("Failed to upload content overrides", "error", err)
		return nil, err
	}

	return &overrideapi.UploadOut{Success: true}, nil
}

// ComRedhatRhsmContentReleaseBackend implements the interface for the com.redhat.rhsm.content.release.varlink
type ComRedhatRhsmContentReleaseBackend struct{}

// NewComRedhatRhsmContentReleaseRHSMBackend creates a new ComRedhatRhsmContentReleaseBackend instance.
func NewComRedhatRhsmContentReleaseRHSMBackend() *ComRedhatRhsmContentReleaseBackend {
	return &ComRedhatRhsmContentReleaseBackend{}
}

// Download implements the interface for the com.redhat.rhsm.testing.content.release.varlink.
// This method downloads the release from the server and returns it. It does not have any side effects.
// The release is not written to disk. If the client application wants to write the release to disk, then
// another Varlink method SetRelease() should be used for this purpose.
func (c ComRedhatRhsmContentReleaseBackend) Download(in *releaseapi.DownloadIn) (*releaseapi.DownloadOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return nil, &releaseapi.SystemNotRegisteredError{}
	}

	var release *string
	if in.Metadata != nil {
		release, err = DownloadRelease(
			in.Metadata.UserAgent,
			in.Metadata.Locale,
			in.Metadata.CorrelationId,
		)
	} else {
		release, err = DownloadRelease(nil, nil, nil)
	}

	if err != nil {
		return nil, err
	}

	return &releaseapi.DownloadOut{Release: *release}, nil
}

// GetAvailableReleases retrieves a list of available releases and sorts them alphabetically.
func (c ComRedhatRhsmContentReleaseBackend) GetAvailableReleases(in *releaseapi.GetAvailableReleasesIn) (*releaseapi.GetAvailableReleasesOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil || !registered {
		slog.Debug("System is not registered", "error", err)
		return nil, &releaseapi.SystemNotRegisteredError{}
	}

	var releases map[string]struct{}
	if in.Metadata != nil {
		releases, err = GetAvailableReleases(in.Metadata.UserAgent, in.Metadata.Locale, in.Metadata.CorrelationId)
	} else {
		releases, err = GetAvailableReleases(nil, nil, nil)
	}

	if err != nil {
		return nil, err
	}

	// Convert map of structure (workaround for absence of set in Go) to array of strings
	// and sort it alphabetically.
	releaseList := make([]string, 0, len(releases))
	for release := range releases {
		releaseList = append(releaseList, release)
	}
	slices.Sort(releaseList)

	return &releaseapi.GetAvailableReleasesOut{Releases: releaseList}, nil
}

// GetCurrentRelease returns the current release.
func (c ComRedhatRhsmContentReleaseBackend) GetCurrentRelease(in *releaseapi.GetCurrentReleaseIn) (*releaseapi.GetCurrentReleaseOut, error) {
	rhsmClient, err := rhsm2.GetRHSMClient(nil, nil)
	if err != nil {
		return nil, &ClientError{Message: err.Error()}
	}

	release, err := rhsmClient.GetDnfVarsRelease()
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	// When release is empty string, return nil pointer
	if release == "" {
		return &releaseapi.GetCurrentReleaseOut{Release: nil}, nil
	}

	return &releaseapi.GetCurrentReleaseOut{Release: &release}, nil
}

// SetRelease sets the current release and it tries to send release to the candlepin server.
// Sending of release to the candlepin server is done asynchronously.
func (c ComRedhatRhsmContentReleaseBackend) SetRelease(in *releaseapi.SetReleaseIn) (*releaseapi.SetReleaseOut, error) {
	var err error
	if in.Metadata != nil {
		err = SetRelease(in.Release, in.Metadata.UserAgent, in.Metadata.Locale, in.Metadata.CorrelationId)
	} else {
		err = SetRelease(in.Release, nil, nil, nil)
	}
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	return &releaseapi.SetReleaseOut{Success: true}, nil
}

// UnsetRelease unsets the current release and it tries to send release to the candlepin server.
// Unsetting the release is done asynchronously.
func (c ComRedhatRhsmContentReleaseBackend) UnsetRelease(in *releaseapi.UnsetReleaseIn) (*releaseapi.UnsetReleaseOut, error) {
	var err error
	if in.Metadata != nil {
		err = SetRelease("", in.Metadata.UserAgent, in.Metadata.Locale, in.Metadata.CorrelationId)
	} else {
		err = SetRelease("", nil, nil, nil)
	}
	if err != nil {
		return nil, &ServerError{Message: err.Error()}
	}

	return &releaseapi.UnsetReleaseOut{Success: true}, nil
}
