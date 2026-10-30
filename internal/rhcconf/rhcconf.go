// Package rhcconf builds rhc's effective configuration by merging, in
// increasing order of precedence: embedded defaults, a translation of
// /etc/rhsm/rhsm.conf, and /etc/rhc/rhc.conf. See ADR-013 (rhc.conf content).
package rhcconf

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/redhatinsights/rhc/internal/config"
	"github.com/redhatinsights/rhc/internal/rhsmconf"
)

// Source names the configuration file a resolved value ultimately came from.
type Source string

const (
	SourceDefault  Source = "default"
	SourceRHSMConf Source = "rhsm.conf"
	SourceRHCConf  Source = "rhc.conf"
)

// Endpoint is the effective configuration for one HTTP API endpoint.
// URI, TLSVerify, and CAPath each resolve independently; Source reflects
// where URI came from.
type Endpoint struct {
	URI       string
	TLSVerify bool
	CAPath    string
	Source    Source
}

// Proxy is the effective forward proxy configuration. Source reflects
// where URI came from.
type Proxy struct {
	URI      string
	User     string
	Password string
	NoProxy  string
	Source   Source
}

// Config is rhc's fully resolved, effective configuration.
type Config struct {
	Proxy         Proxy
	Subscriptions Endpoint
	RPMContent    Endpoint
}

// Load reads rhc.conf and rhsm.conf and returns rhc's effective
// configuration. A missing rhsm.conf (subscription-manager not installed)
// is not an error. rhsm.conf is not read at all when
// interpret-legacy-configurations is explicitly false.
func Load(rhcSource config.Source, rhsmSource rhsmconf.Source) (*Config, error) {
	file, md, err := config.Get(rhcSource)
	if err != nil {
		return nil, fmt.Errorf("reading rhc.conf: %w", err)
	}

	var rhsm *rhsmconf.Config
	if file.Compatibility.InterpretLegacy {
		rhsm, err = rhsmconf.Load(rhsmSource)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("reading rhsm.conf: %w", err)
			}
			rhsm = nil
		}
	}

	cfg, err := Resolve(file, md, rhsm)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Resolve merges file (rhc.conf, already carrying its own embedded
// defaults) with a translation of rhsm (nil when unavailable), using md to
// tell which rhc.conf keys were explicitly set. Explicit rhc.conf wins,
// then the rhsm.conf translation, then rhc.conf's own embedded default.
// It errors if an explicit rhc.conf uri is invalid, or if an rhsm.conf
// value would be used but is invalid.
func Resolve(file config.File, md toml.MetaData, rhsm *rhsmconf.Config) (Config, error) {
	translated := translate(rhsm)

	proxy, err := resolveProxy(file, md, translated.proxy)
	if err != nil {
		return Config{}, err
	}
	subscriptions, err := resolveEndpoint(file.API.Subscriptions, translated.subscriptions, md, "subscriptions")
	if err != nil {
		return Config{}, err
	}
	rpmContent, err := resolveEndpoint(file.API.RPMContent, translated.rpmContent, md, "rpm-content")
	if err != nil {
		return Config{}, err
	}

	return Config{Proxy: proxy, Subscriptions: subscriptions, RPMContent: rpmContent}, nil
}

// translation holds rhsm.conf's fields reshaped into rhc.conf's schema.
// A field is nil when rhsm.conf doesn't define enough to produce it.
type translation struct {
	proxy         *Proxy
	subscriptions *Endpoint
	rpmContent    *Endpoint
}

func translate(rhsm *rhsmconf.Config) translation {
	if rhsm == nil {
		return translation{}
	}
	return translation{
		proxy:         translateProxy(rhsm),
		subscriptions: translateSubscriptions(rhsm),
		rpmContent:    translateRPMContent(rhsm),
	}
}

// validateURI confirms uri is an absolute http/https URI. net/url.Parse
// alone is too lenient to catch common typos: a missing scheme or host
// still parses successfully as a relative reference (e.g.
// "subscription.rhsm.redhat.com/subscription" parses with no error, no
// scheme, no host), so scheme and host are checked explicitly.
func validateURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI %q: %w", uri, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URI %q must use the http or https scheme", uri)
	}
	if u.Host == "" {
		return fmt.Errorf("URI %q is missing a host", uri)
	}
	return nil
}

// validatePort confirms port is empty (no port) or a valid 16-bit TCP port
// number. Neither net.JoinHostPort nor url.Parse check this: both accept
// "host:999999" and "host:abc" without complaint, so a typo'd rhsm.conf
// port would otherwise reach a built URI unnoticed.
func validatePort(port string) error {
	if port == "" {
		return nil
	}
	if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
		return fmt.Errorf("%q is not a valid port", port)
	}
	return nil
}

// bracketHost wraps an IPv6 literal (e.g. "::1") in brackets so it's valid
// in a URL authority component on its own, with no port. net.JoinHostPort
// does the equivalent for a host *and* port together, but requires both
// arguments: passing it an empty port leaves a dangling "host:" instead of
// omitting the port, so it can't be used for this no-port case.
func bracketHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

// joinHost returns the URL authority component for host and an optional
// port: net.JoinHostPort when port is set (which also brackets an IPv6
// host), or bracketHost alone when there's no port to join.
func joinHost(host, port string) string {
	if port == "" {
		return bracketHost(host)
	}
	return net.JoinHostPort(host, port)
}

// translateProxy combines rhsm.conf's discrete proxy_* fields into a single
// URI. An empty proxy_hostname means no proxy is configured: proxy_scheme
// defaults to "http" in the packaged file even with no proxy set, so
// hostname is the real signal, not scheme. The built URI is NOT validated
// here; validation happens in resolveProxy, and only if this translated
// value would actually be used (no rhc.conf override).
func translateProxy(rhsm *rhsmconf.Config) *Proxy {
	if rhsm.ProxyHostname == "" {
		return nil
	}
	scheme := rhsm.ProxyScheme
	if scheme == "" {
		scheme = "http"
	}
	uri := scheme + "://" + joinHost(rhsm.ProxyHostname, rhsm.ProxyPort)
	return &Proxy{
		URI:      uri,
		User:     rhsm.ProxyUser,
		Password: rhsm.ProxyPassword,
		NoProxy:  rhsm.NoProxy,
		Source:   SourceRHSMConf,
	}
}

// translateSubscriptions builds the registration server URI from rhsm.conf's
// hostname, port, and prefix. Port 443 (the https default) is omitted, same
// as ADR-013's default URI. The prefix is checked here (before building the
// URI) because a bad prefix creates a syntactically-valid-but-broken URI that
// validateURI can't detect. Port and hostname validation happens in
// resolveEndpoint, only if this translated value would actually be used.
func translateSubscriptions(rhsm *rhsmconf.Config) *Endpoint {
	if rhsm.Hostname == "" {
		return nil
	}
	// Prefix must start with "/" to avoid concatenating onto hostname.
	// Check here because the bad URI would pass url.Parse.
	if rhsm.Prefix != "" && !strings.HasPrefix(rhsm.Prefix, "/") {
		// Return a marked-invalid endpoint so Resolve can error with context.
		// TLSVerify and CAPath must still be populated so they're available
		// if rhc.conf overrides the URI (precedence: bad rhsm.conf URI doesn't
		// block rhc.conf override, but rhsm.conf's TLS settings should still apply).
		return &Endpoint{
			URI:       fmt.Sprintf("INVALID_PREFIX:%s", rhsm.Prefix),
			TLSVerify: !rhsm.Insecure,
			CAPath:    rhsm.RepoCACert,
			Source:    SourceRHSMConf,
		}
	}
	port := rhsm.Port
	if port == "443" {
		port = ""
	}
	uri := "https://" + joinHost(rhsm.Hostname, port) + rhsm.Prefix
	return &Endpoint{
		URI:       uri,
		TLSVerify: !rhsm.Insecure,
		CAPath:    rhsm.RepoCACert,
		Source:    SourceRHSMConf,
	}
}

// translateRPMContent maps rhsm.conf's baseurl directly: it is already a
// full content delivery URI, not a host/port/prefix triple. It is NOT
// validated here; validation happens in resolveEndpoint, and only if this
// translated value would actually be used (no rhc.conf override).
func translateRPMContent(rhsm *rhsmconf.Config) *Endpoint {
	if rhsm.BaseURL == "" {
		return nil
	}
	return &Endpoint{
		URI:       rhsm.BaseURL,
		TLSVerify: !rhsm.Insecure,
		CAPath:    rhsm.RepoCACert,
		Source:    SourceRHSMConf,
	}
}

// validateProxyFromRHSM validates a proxy URI built from rhsm.conf's discrete
// fields. Called only when that proxy will actually be used.
func validateProxyFromRHSM(p *Proxy) error {
	// Extract components from the built URI for targeted error messages.
	u, err := url.Parse(p.URI)
	if err != nil {
		return fmt.Errorf("rhsm.conf proxy: malformed URI %q: %w", p.URI, err)
	}
	
	// Validate scheme (proxy_scheme in rhsm.conf)
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("server.proxy_scheme: must be 'http' or 'https' (got %q)", u.Scheme)
	}
	
	// Validate host (proxy_hostname in rhsm.conf)
	if u.Host == "" {
		return fmt.Errorf("server.proxy_hostname: missing or invalid")
	}
	
	// Validate port if present (proxy_port in rhsm.conf)
	if u.Port() != "" {
		if err := validatePort(u.Port()); err != nil {
			return fmt.Errorf("server.proxy_port: %w", err)
		}
	}
	
	return nil
}

// resolveProxy applies the three-tier precedence to the proxy group. rhsm.conf
// values are validated only if they'll be used (rhc.conf doesn't override).
// rhc.conf explicitly setting an invalid http.proxy.uri is an error, but an
// explicit empty uri is accepted without validation: it's how an admin
// opts out of a proxy configured by rhsm.conf.
func resolveProxy(file config.File, md toml.MetaData, fromRHSM *Proxy) (Proxy, error) {
	p := Proxy{
		URI:      file.HTTP.Proxy.URI,
		User:     file.HTTP.Proxy.User,
		Password: file.HTTP.Proxy.Password,
		NoProxy:  file.HTTP.Proxy.NoProxy,
		Source:   SourceDefault,
	}

	// Apply rhsm.conf translation (if available) before rhc.conf overrides.
	// Each field is merged independently.
	if fromRHSM != nil {
		p.URI, p.Source = fromRHSM.URI, SourceRHSMConf
		p.User = fromRHSM.User
		p.Password = fromRHSM.Password
		p.NoProxy = fromRHSM.NoProxy
	}

	// rhc.conf explicit overrides win; URI gets validated, empty URI is
	// accepted (disables legacy proxy). An explicit empty URI means "no proxy",
	// so we clear ALL proxy fields, not just the URI.
	if md.IsDefined("http", "proxy", "uri") {
		if file.HTTP.Proxy.URI != "" {
			if err := validateURI(file.HTTP.Proxy.URI); err != nil {
				return Proxy{}, fmt.Errorf("http.proxy.uri: %w", err)
			}
		} else {
			// Empty URI = complete opt-out; don't keep rhsm.conf's user/password/no-proxy.
			p.User = ""
			p.Password = ""
			p.NoProxy = ""
		}
		p.URI, p.Source = file.HTTP.Proxy.URI, SourceRHCConf
	}
	if md.IsDefined("http", "proxy", "user") {
		p.User = file.HTTP.Proxy.User
	}
	if md.IsDefined("http", "proxy", "password") {
		p.Password = file.HTTP.Proxy.Password
	}
	if md.IsDefined("http", "proxy", "no-proxy") {
		p.NoProxy = file.HTTP.Proxy.NoProxy
	}

	// Validate rhsm-derived URI only if it's actually being used (rhc.conf
	// didn't override).
	if p.Source == SourceRHSMConf && fromRHSM != nil {
		if err := validateProxyFromRHSM(fromRHSM); err != nil {
			return Proxy{}, err
		}
	}

	return p, nil
}

// validateEndpointFromRHSM validates an endpoint URI built from rhsm.conf.
// Called only when that endpoint will actually be used. section is
// "subscriptions" or "rpm-content" for targeted error messages.
func validateEndpointFromRHSM(ep *Endpoint, section string) error {
	// Check for prefix validation failure from translateSubscriptions.
	if strings.HasPrefix(ep.URI, "INVALID_PREFIX:") {
		prefix := strings.TrimPrefix(ep.URI, "INVALID_PREFIX:")
		return fmt.Errorf("server.prefix: must start with '/' (got %q)", prefix)
	}
	
	u, err := url.Parse(ep.URI)
	if err != nil {
		return fmt.Errorf("rhsm.conf %s: malformed URI %q: %w", section, ep.URI, err)
	}
	
	if section == "subscriptions" {
		// Built from hostname/port/prefix
		if u.Scheme != "https" {
			return fmt.Errorf("server.hostname/port/prefix: URI must use https scheme (got %q)", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("server.hostname: missing or invalid")
		}
		if u.Port() != "" {
			if err := validatePort(u.Port()); err != nil {
				return fmt.Errorf("server.port: %w", err)
			}
		}
	} else {
		// rpm-content: baseurl passthrough
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("rhsm.baseurl: URI must use http or https scheme (got %q)", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("rhsm.baseurl: missing host")
		}
	}
	
	return nil
}

// resolveEndpoint applies the three-tier precedence to one [api.*] endpoint.
// section is the name under [api.*] ("subscriptions" or "rpm-content"). rhsm.conf
// values are validated only if they'll be used (rhc.conf doesn't override).
func resolveEndpoint(fromFile config.Endpoint, fromRHSM *Endpoint, md toml.MetaData, section string) (Endpoint, error) {
	ep := Endpoint{
		URI:       fromFile.URI,
		TLSVerify: fromFile.TLSVerify,
		CAPath:    fromFile.CAPath,
		Source:    SourceDefault,
	}

	// Apply rhsm.conf translation (if available) before rhc.conf overrides.
	// Each field is merged independently.
	if fromRHSM != nil {
		ep.URI, ep.Source = fromRHSM.URI, SourceRHSMConf
		ep.TLSVerify = fromRHSM.TLSVerify
		ep.CAPath = fromRHSM.CAPath
	}

	// rhc.conf explicit overrides win.
	if md.IsDefined("api", section, "uri") {
		if err := validateURI(fromFile.URI); err != nil {
			return Endpoint{}, fmt.Errorf("api.%s.uri: %w", section, err)
		}
		ep.URI, ep.Source = fromFile.URI, SourceRHCConf
	}
	if md.IsDefined("api", section, "tls-verify") {
		ep.TLSVerify = fromFile.TLSVerify
	}
	if md.IsDefined("api", section, "ca-path") {
		ep.CAPath = fromFile.CAPath
	}

	// Validate rhsm-derived URI only if it's actually being used (rhc.conf
	// didn't override).
	if ep.Source == SourceRHSMConf && fromRHSM != nil {
		if err := validateEndpointFromRHSM(fromRHSM, section); err != nil {
			return Endpoint{}, err
		}
	}

	return ep, nil
}

// Log emits one DEBUG line for the proxy and one per API endpoint, naming
// the source that produced the printed URI: default, rhsm.conf, or rhc.conf.
// sanitizeURI removes credentials (user:pass@) from a URI for safe logging.
// If the URI is unparseable, it's returned as-is (better to log a malformed
// value than to panic or suppress it entirely).
func sanitizeURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.User == nil {
		return uri
	}
	u.User = nil
	return u.String()
}

func (c Config) Log() {
	slog.Debug("effective proxy config", "source", c.Proxy.Source, "uri", sanitizeURI(c.Proxy.URI))
	slog.Debug("effective api.subscriptions", "source", c.Subscriptions.Source, "uri", sanitizeURI(c.Subscriptions.URI))
	slog.Debug("effective api.rpm-content", "source", c.RPMContent.Source, "uri", sanitizeURI(c.RPMContent.URI))
}
