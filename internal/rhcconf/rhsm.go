package rhcconf

import (
	"fmt"
	"net/url"

	"github.com/BurntSushi/toml"

	"github.com/redhatinsights/rhc/internal/config"
)

// systemCAPath is the rhc.conf sentinel that selects the system trust store.
// It is not a certificate bundle file, so it is not written to rhsm.repo_ca_cert.
const systemCAPath = "system"

// RHSMSetting is one com.redhat.RHSM1.Config.Set argument pair. Key uses
// subscription-manager's "section.name" form, for example "server.hostname".
type RHSMSetting struct {
	Key   string
	Value string
}

// RHSMSettingsFromRHCConf returns the rhsm.conf keys rhc.conf explicitly sets.
// Commented keys and embedded defaults are omitted, so a shipped rhc.conf
// produces an empty list. api.insights and http.timeout are never translated.
// api.rpm-content tls-verify and ca-path are not written either:
// server.insecure and rhsm.repo_ca_cert already come from api.subscriptions.
// The order is stable: subscriptions uri, ca-path, tls-verify, rpm-content
// uri, then proxy uri, user, password, and no-proxy.
func RHSMSettingsFromRHCConf(file config.File, md toml.MetaData) ([]RHSMSetting, error) {
	var settings []RHSMSetting

	if md.IsDefined("api", "subscriptions", "uri") {
		hostname, port, prefix, err := splitSubscriptionsURI(file.API.Subscriptions.URI)
		if err != nil {
			return nil, err
		}
		settings = append(settings,
			RHSMSetting{Key: "server.hostname", Value: hostname},
			RHSMSetting{Key: "server.port", Value: port},
			RHSMSetting{Key: "server.prefix", Value: prefix},
		)
	}
	if md.IsDefined("api", "subscriptions", "ca-path") && file.API.Subscriptions.CAPath != systemCAPath {
		settings = append(settings, RHSMSetting{
			Key:   "rhsm.repo_ca_cert",
			Value: file.API.Subscriptions.CAPath,
		})
	}
	if md.IsDefined("api", "subscriptions", "tls-verify") {
		insecure := "0"
		if !file.API.Subscriptions.TLSVerify {
			insecure = "1"
		}
		settings = append(settings, RHSMSetting{Key: "server.insecure", Value: insecure})
	}
	if md.IsDefined("api", "rpm-content", "uri") {
		if err := validateURI(file.API.RPMContent.URI); err != nil {
			return nil, fmt.Errorf("api.rpm-content.uri: %w", err)
		}
		settings = append(settings, RHSMSetting{
			Key:   "rhsm.baseurl",
			Value: file.API.RPMContent.URI,
		})
	}

	if md.IsDefined("http", "proxy", "uri") {
		scheme, hostname, port, err := splitProxyURI(file.HTTP.Proxy.URI)
		if err != nil {
			return nil, err
		}
		settings = append(settings,
			RHSMSetting{Key: "server.proxy_scheme", Value: scheme},
			RHSMSetting{Key: "server.proxy_hostname", Value: hostname},
			RHSMSetting{Key: "server.proxy_port", Value: port},
		)
	}
	if md.IsDefined("http", "proxy", "user") {
		settings = append(settings, RHSMSetting{Key: "server.proxy_user", Value: file.HTTP.Proxy.User})
	}
	if md.IsDefined("http", "proxy", "password") {
		settings = append(settings, RHSMSetting{Key: "server.proxy_password", Value: file.HTTP.Proxy.Password})
	}
	if md.IsDefined("http", "proxy", "no-proxy") {
		settings = append(settings, RHSMSetting{Key: "server.no_proxy", Value: file.HTTP.Proxy.NoProxy})
	}

	return settings, nil
}

func splitSubscriptionsURI(raw string) (hostname, port, prefix string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("parse api.subscriptions.uri: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", fmt.Errorf("api.subscriptions.uri %q: want https://host[:port]/prefix", raw)
	}
	port = u.Port()
	if port == "" {
		port = "443"
	}
	prefix = u.Path
	if prefix == "" {
		prefix = "/"
	}
	return u.Hostname(), port, prefix, nil
}

func splitProxyURI(raw string) (scheme, hostname, port string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("parse http.proxy.uri: %w", err)
	}
	if u.Scheme == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", fmt.Errorf("http.proxy.uri %q: want scheme://host[:port]", raw)
	}
	return u.Scheme, u.Hostname(), u.Port(), nil
}
