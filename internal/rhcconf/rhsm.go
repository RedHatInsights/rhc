package rhcconf

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/redhatinsights/rhc/internal/config"
)

// systemCAPath is the rhc.conf sentinel that selects the system trust store.
const systemCAPath = "system"

// systemCABundle is that trust store on RHEL, CentOS Stream, and Fedora.
// subscription-manager stores a certificate file in rhsm.repo_ca_cert, so the
// sentinel is written as this path and dnf uses it as sslcacert.
const systemCABundle = "/etc/pki/tls/certs/ca-bundle.crt"

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
// An explicit (non-"system") ca-path must point at an existing file, and its
// directory is also written to rhsm.ca_cert_dir so subscription-manager trusts
// the CA when it opens the registration connection, not only when dnf later
// reads repo files.  Note that ca_cert_dir trust is directory-wide: every
// *.pem in that directory becomes trusted for the connection.  The "system"
// sentinel and the hosted default are left on the packaged ca_cert_dir.
// An explicit empty http.proxy.uri opts out of the proxy, matching Resolve:
// scheme is reset to http, and hostname, port, user, password, and no-proxy
// are cleared. An explicit user, password, or no-proxy replaces that clear.
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
	if md.IsDefined("api", "subscriptions", "ca-path") {
		caPath := file.API.Subscriptions.CAPath
		if caPath == systemCAPath {
			caPath = systemCABundle
			slog.Debug("api.subscriptions.ca-path sentinel maps to the system trust bundle", "repo_ca_cert", caPath)
		} else {
			if err := validateCACertFile(caPath); err != nil {
				return nil, fmt.Errorf("api.subscriptions.ca-path: %w", err)
			}
		}
		settings = append(settings, RHSMSetting{
			Key:   "rhsm.repo_ca_cert",
			Value: caPath,
		})
		// An explicit CA must also be trusted when subscription-manager opens
		// the registration connection, which reads rhsm.ca_cert_dir rather than
		// rhsm.repo_ca_cert. The hosted "system" bundle keeps the packaged dir.
		if caPath != systemCABundle {
			dir := caCertDir(caPath)
			slog.Debug("trusting CA directory for the registration connection", "ca_cert_dir", dir)
			settings = append(settings, RHSMSetting{
				Key:   "rhsm.ca_cert_dir",
				Value: dir,
			})
		}
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

	// An empty URI is the opt-out, not a URI to parse. Hostname is what
	// subscription-manager treats as "no proxy"; scheme http is the packaged
	// value for that disabled state. User, password, and no-proxy are cleared
	// too, unless rhc.conf sets them explicitly.
	proxyCleared := false
	if md.IsDefined("http", "proxy", "uri") {
		if file.HTTP.Proxy.URI == "" {
			proxyCleared = true
			slog.Debug("http.proxy.uri is empty: clearing rhsm proxy settings")
			settings = append(settings,
				RHSMSetting{Key: "server.proxy_scheme", Value: "http"},
				RHSMSetting{Key: "server.proxy_hostname", Value: ""},
				RHSMSetting{Key: "server.proxy_port", Value: ""},
			)
		} else {
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
	}
	if md.IsDefined("http", "proxy", "user") || proxyCleared {
		settings = append(settings, RHSMSetting{Key: "server.proxy_user", Value: file.HTTP.Proxy.User})
	}
	if md.IsDefined("http", "proxy", "password") || proxyCleared {
		settings = append(settings, RHSMSetting{Key: "server.proxy_password", Value: file.HTTP.Proxy.Password})
	}
	if md.IsDefined("http", "proxy", "no-proxy") || proxyCleared {
		settings = append(settings, RHSMSetting{Key: "server.no_proxy", Value: file.HTTP.Proxy.NoProxy})
	}

	// Log only key names: values may include server.proxy_password and
	// credential-bearing URIs, which must never reach the logs.
	slog.Debug("translated rhc.conf into rhsm.conf settings",
		"count", len(settings), "keys", settingKeys(settings))
	return settings, nil
}

// settingKeys returns the ordered key names of settings for logging, omitting
// the values, which may be secrets (server.proxy_password) or carry credentials.
func settingKeys(settings []RHSMSetting) []string {
	keys := make([]string, len(settings))
	for i, s := range settings {
		keys[i] = s.Key
	}
	return keys
}

// validateCACertFile rejects an explicit ca-path that does not resolve to an
// existing, readable regular file. Catching a typo'd or missing path here turns
// a silent dnf TLS failure later into an immediate, actionable connect error.
func validateCACertFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file, want a certificate file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%q: %w", path, err)
	}
	return f.Close()
}

// caCertDir returns the directory holding an explicit CA file, with a trailing
// separator to match subscription-manager's packaged ca_cert_dir convention.
func caCertDir(caPath string) string {
	return filepath.Dir(caPath) + string(os.PathSeparator)
}

func splitSubscriptionsURI(raw string) (hostname, port, prefix string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("parse api.subscriptions.uri: %w", redactURLError(err))
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", fmt.Errorf("api.subscriptions.uri %q: want https://host[:port]/prefix", u.Redacted())
	}
	if err = validatePort(u.Port()); err != nil {
		return "", "", "", fmt.Errorf("api.subscriptions.uri %q: %w", u.Redacted(), err)
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
		return "", "", "", fmt.Errorf("parse http.proxy.uri: %w", redactURLError(err))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", "", fmt.Errorf("http.proxy.uri %q: scheme must be http or https", u.Redacted())
	}
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", fmt.Errorf("http.proxy.uri %q: want scheme://host[:port]", u.Redacted())
	}
	port = u.Port()
	if err := validatePort(port); err != nil {
		return "", "", "", fmt.Errorf("http.proxy.uri %q: %w", u.Redacted(), err)
	}
	return u.Scheme, u.Hostname(), port, nil
}
