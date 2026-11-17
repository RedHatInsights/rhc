package rhcconf

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/redhatinsights/rhc/internal/config"
	"github.com/redhatinsights/rhc/internal/rhsmconf"
)

// The Insights endpoints authenticate with the consumer identity certificate
// managed by subscription-manager.They are fixed constants rather than configurable
// fields.
const (
	ConsumerCertPath = "/etc/pki/consumer/cert.pem"
	ConsumerKeyPath  = "/etc/pki/consumer/key.pem"
)

// resolveInsightsEndpoint derives an unset URI from the effective registration
// host. TLS verification is independent of subscriptions; only Satellite's
// default CA comes from the resolved rhsm.repo_ca_cert.
func resolveInsightsEndpoint(fromFile config.Endpoint, md toml.MetaData, section string, subscriptions Endpoint, rhsm *rhsmconf.Config) (Endpoint, error) {
	ep := Endpoint{
		URI:       fromFile.URI,
		TLSVerify: fromFile.TLSVerify,
		CAPath:    fromFile.CAPath,
		Source:    SourceDefault,
	}
	// An explicit URI skips derivation from the hostname in subscriptions.URI
	// and bypasses Satellite's default CA.
	if md.IsDefined("api", "insights", section, "uri") {
		if err := validateURI(ep.URI); err != nil {
			return Endpoint{}, fmt.Errorf("api.insights.%s.uri: %w", section, err)
		}
		ep.Source = SourceRHCConf
		return ep, nil
	}

	registration, err := url.Parse(subscriptions.URI)
	if err != nil {
		return Endpoint{}, fmt.Errorf("parsing registration URI %q: %w", subscriptions.URI, err)
	}

	if registration.Hostname() == "" {
		return Endpoint{}, fmt.Errorf("registration URI %q is missing a host", subscriptions.URI)
	}
	if err := validatePort(registration.Port()); err != nil {
		return Endpoint{}, fmt.Errorf("registration URI %q: %w", subscriptions.URI, err)
	}
	host := strings.TrimSuffix(strings.ToLower(registration.Hostname()), ".")
	if strings.HasSuffix(host, ".rhsm.redhat.com") {
		return ep, nil
	}
	// Stage changes ingress only; inventory keeps its production default.
	if strings.HasSuffix(host, ".rhsm.stage.redhat.com") {
		if section == "ingress" {
			ep.URI = "https://cert.console.stage.redhat.com/api/ingress/v1/upload"
			ep.Source = subscriptions.Source
		}
		return ep, nil
	}

	path := "/redhat_access/r/insights/platform/ingress/v1/upload"
	if section == "inventory" {
		path = "/redhat_access/r/insights/platform/inventory/v1"
	}
	ep.URI = (&url.URL{Scheme: "https", Host: registration.Host, Path: path}).String()
	ep.Source = subscriptions.Source
	// An explicit ca-path, even empty, takes precedence over rhsm.conf.
	if rhsm != nil && !md.IsDefined("api", "insights", section, "ca-path") {
		ep.CAPath = rhsm.RepoCACert
	}
	return ep, nil
}
