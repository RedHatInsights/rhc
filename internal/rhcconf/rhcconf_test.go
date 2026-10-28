package rhcconf

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"

	"github.com/redhatinsights/rhc/internal/config"
	"github.com/redhatinsights/rhc/internal/rhsmconf"
)

// fakeFS hands config.Get()/rhsmconf.Load() in-memory content or a forced
// error, while still exercising their real parsing/defaulting logic.
type fakeFS struct {
	data []byte
	err  error
}

func (f fakeFS) Read(string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.data, nil
}

// hostedRHSM mirrors subscription-manager's stock, unmodified rhsm.conf:
// hosted Red Hat, no proxy.
var hostedRHSM = &rhsmconf.Config{
	Hostname:    "subscription.rhsm.redhat.com",
	Port:        "443",
	Prefix:      "/subscription",
	ProxyScheme: "http",
	BaseURL:     "https://cdn.redhat.com",
	RepoCACert:  "/etc/rhsm/ca/redhat-uep.pem",
}

// stageRHSM is a Red Hat stage registration with a forward proxy configured.
var stageRHSM = &rhsmconf.Config{
	Hostname:      "subscription.rhsm.stage.redhat.com",
	Port:          "443",
	Prefix:        "/subscription",
	ProxyHostname: "corp-proxy.example.com",
	ProxyScheme:   "http",
	ProxyPort:     "3128",
	ProxyUser:     "proxyuser",
	ProxyPassword: "proxypass",
	NoProxy:       "localhost,127.0.0.1,.internal.example.com",
	BaseURL:       "https://cdn.redhat.com",
	RepoCACert:    "/etc/rhsm/ca/redhat-uep.pem",
}

// satelliteRHSM has a non-redhat.com hostname and a Katello CA, matching a
// Satellite-registered system's rhsm.conf.
var satelliteRHSM = &rhsmconf.Config{
	Hostname:   "satellite.example.com",
	Port:       "443",
	Prefix:     "/rhsm",
	BaseURL:    "https://satellite.example.com/pulp/content",
	RepoCACert: "/etc/rhsm/ca/katello-server-ca.pem",
}

// decodeRHCConf parses an rhc.conf fragment through config.Get()'s real
// embedded-default logic, not just toml.Decode.
func decodeRHCConf(t *testing.T, content string) (config.File, toml.MetaData) {
	t.Helper()
	file, md, err := config.Get(config.Source{
		Filesystem: fakeFS{data: []byte(content)},
		FilePath:   "rhc.conf",
	})
	if err != nil {
		t.Fatalf("decoding test rhc.conf fragment: %v", err)
	}
	return file, md
}

func TestResolveSubscriptionsURIOverride(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
`)

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := Endpoint{
		URI:       "https://satellite.example.com/subscription",
		TLSVerify: true,
		CAPath:    "/etc/rhsm/ca/redhat-uep.pem",
		Source:    SourceRHCConf,
	}
	if diff := cmp.Diff(want, got.Subscriptions); diff != "" {
		t.Errorf("Resolve().Subscriptions mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveAdminAddedAPISectionToShippedFile(t *testing.T) {
	// The packaged rhc.conf ships with no [api.*] section; an admin can add
	// one by hand. Only the key actually added should flip to
	// source=rhc.conf — untouched fields keep resolving from rhsm.conf.
	shipped, err := os.ReadFile("../../data/config/rhc.conf")
	if err != nil {
		t.Fatalf("read packaged rhc.conf: %v", err)
	}
	edited := string(shipped) + "\n[api.subscriptions]\nuri = \"https://my-satellite.example.com/subscription\"\n"
	file, md := decodeRHCConf(t, edited)

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := Endpoint{
		URI:       "https://my-satellite.example.com/subscription",
		TLSVerify: true,
		CAPath:    "/etc/rhsm/ca/redhat-uep.pem", // untouched: still from the rhsm.conf translation
		Source:    SourceRHCConf,
	}
	if diff := cmp.Diff(want, got.Subscriptions); diff != "" {
		t.Errorf("Resolve().Subscriptions mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveFullyCommentedRHCConfMatchesRHSMConfAlone(t *testing.T) {
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := "https://subscription.rhsm.redhat.com/subscription"
	if got.Subscriptions.URI != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got.Subscriptions.URI, want)
	}
	if got.Subscriptions.Source != SourceRHSMConf {
		t.Errorf("Subscriptions.Source = %q, want %q", got.Subscriptions.Source, SourceRHSMConf)
	}
}

func TestResolveNoRHSMConfUsesEmbeddedDefault(t *testing.T) {
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := "https://subscription.rhsm.redhat.com/subscription"
	if got.Subscriptions.URI != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got.Subscriptions.URI, want)
	}
	if got.Subscriptions.Source != SourceDefault {
		t.Errorf("Subscriptions.Source = %q, want %q", got.Subscriptions.Source, SourceDefault)
	}
	if got.Proxy.Source != SourceDefault {
		t.Errorf("Proxy.Source = %q, want %q", got.Proxy.Source, SourceDefault)
	}
	if got.RPMContent.Source != SourceDefault {
		t.Errorf("RPMContent.Source = %q, want %q", got.RPMContent.Source, SourceDefault)
	}
}

func TestResolveProxyFromRHSMConf(t *testing.T) {
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, stageRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := Proxy{
		URI:      "http://corp-proxy.example.com:3128",
		User:     "proxyuser",
		Password: "proxypass",
		NoProxy:  "localhost,127.0.0.1,.internal.example.com",
		Source:   SourceRHSMConf,
	}
	if diff := cmp.Diff(want, got.Proxy); diff != "" {
		t.Errorf("Resolve().Proxy mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveProxyExplicitRHCConfWinsOverRHSMConf(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "http://other-proxy.example.com:8080"
`)

	got, err := Resolve(file, md, stageRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Proxy.URI != "http://other-proxy.example.com:8080" {
		t.Errorf("Proxy.URI = %q, want the rhc.conf override", got.Proxy.URI)
	}
	if got.Proxy.Source != SourceRHCConf {
		t.Errorf("Proxy.Source = %q, want %q", got.Proxy.Source, SourceRHCConf)
	}
	// Not set in rhc.conf, so still from the rhsm.conf translation.
	if got.Proxy.User != "proxyuser" {
		t.Errorf("Proxy.User = %q, want the rhsm.conf translation to still apply", got.Proxy.User)
	}
}

func TestResolveNoProxyConfiguredAnywhere(t *testing.T) {
	// hostedRHSM has no proxy_hostname; must not synthesize a proxy from
	// the leftover default proxy_scheme.
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Proxy.URI != "" {
		t.Errorf("Proxy.URI = %q, want empty when no proxy is configured anywhere", got.Proxy.URI)
	}
	if got.Proxy.Source != SourceDefault {
		t.Errorf("Proxy.Source = %q, want %q", got.Proxy.Source, SourceDefault)
	}
}

func TestResolveRPMContentFromBaseURL(t *testing.T) {
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, satelliteRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := Endpoint{
		URI:       "https://satellite.example.com/pulp/content",
		TLSVerify: true,
		CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
		Source:    SourceRHSMConf,
	}
	if diff := cmp.Diff(want, got.RPMContent); diff != "" {
		t.Errorf("Resolve().RPMContent mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveSatelliteSubscriptionsURIUsesNonDefaultPrefix(t *testing.T) {
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, satelliteRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := "https://satellite.example.com/rhsm"
	if got.Subscriptions.URI != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got.Subscriptions.URI, want)
	}
}

func TestResolveNonDefaultPortIsIncludedInURI(t *testing.T) {
	rhsm := &rhsmconf.Config{Hostname: "satellite.example.com", Port: "8443", Prefix: "/rhsm"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := "https://satellite.example.com:8443/rhsm"
	if got.Subscriptions.URI != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got.Subscriptions.URI, want)
	}
}

func TestResolveInsecureSetsTLSVerifyFalse(t *testing.T) {
	rhsm := &rhsmconf.Config{Hostname: "satellite.example.com", Port: "443", Prefix: "/rhsm", Insecure: true}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	got, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Subscriptions.TLSVerify {
		t.Error("Subscriptions.TLSVerify = true, want false when rhsm.conf has insecure = 1")
	}
}

func TestResolveProxyUserPasswordAndNoProxyExplicitOverrides(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
user = "rhc-conf-user"
password = "rhc-conf-password"
no-proxy = "rhc-conf-no-proxy"
`)

	got, err := Resolve(file, md, stageRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Proxy.User != "rhc-conf-user" {
		t.Errorf("Proxy.User = %q, want the rhc.conf override", got.Proxy.User)
	}
	if got.Proxy.Password != "rhc-conf-password" {
		t.Errorf("Proxy.Password = %q, want the rhc.conf override", got.Proxy.Password)
	}
	if got.Proxy.NoProxy != "rhc-conf-no-proxy" {
		t.Errorf("Proxy.NoProxy = %q, want the rhc.conf override", got.Proxy.NoProxy)
	}
	// URI untouched by rhc.conf, so it still comes from rhsm.conf.
	if got.Proxy.Source != SourceRHSMConf {
		t.Errorf("Proxy.Source = %q, want %q (URI untouched by the other overrides)", got.Proxy.Source, SourceRHSMConf)
	}
}

func TestResolveTLSVerifyExplicitOverride(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
tls-verify = false
`)

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Subscriptions.TLSVerify {
		t.Error("Subscriptions.TLSVerify = true, want the rhc.conf override false")
	}
	// URI still comes from the rhsm.conf translation.
	if got.Subscriptions.Source != SourceRHSMConf {
		t.Errorf("Subscriptions.Source = %q, want %q (tls-verify override shouldn't affect it)", got.Subscriptions.Source, SourceRHSMConf)
	}
}

func TestResolveRHCConfCAPathOverridesRHSMConfTranslation(t *testing.T) {
	// uri from rhsm.conf, ca-path pinned in rhc.conf: fields resolve independently.
	file, md := decodeRHCConf(t, `
[api.subscriptions]
ca-path = "system"
`)

	got, err := Resolve(file, md, hostedRHSM)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Subscriptions.URI != "https://subscription.rhsm.redhat.com/subscription" {
		t.Errorf("Subscriptions.URI = %q, want the rhsm.conf translation (ca-path override shouldn't affect it)", got.Subscriptions.URI)
	}
	if got.Subscriptions.CAPath != "system" {
		t.Errorf("Subscriptions.CAPath = %q, want the rhc.conf override %q", got.Subscriptions.CAPath, "system")
	}
	// Source tracks the URI's provenance, which still came from rhsm.conf.
	if got.Subscriptions.Source != SourceRHSMConf {
		t.Errorf("Subscriptions.Source = %q, want %q", got.Subscriptions.Source, SourceRHSMConf)
	}
}

func TestLoadMergesBothFiles(t *testing.T) {
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte(`
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
`)}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{data: []byte(`
[server]
hostname = subscription.rhsm.redhat.com
port = 443
prefix = /subscription

[rhsm]
baseurl = https://cdn.redhat.com
`)}, FilePath: "rhsm.conf"}

	got, err := Load(rhcSource, rhsmSource)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Subscriptions.URI != "https://satellite.example.com/subscription" {
		t.Errorf("Subscriptions.URI = %q, want the rhc.conf override", got.Subscriptions.URI)
	}
	if got.RPMContent.URI != "https://cdn.redhat.com" {
		t.Errorf("RPMContent.URI = %q, want the rhsm.conf translation", got.RPMContent.URI)
	}
}

func TestLoadTreatsMissingRHSMConfAsNotAnError(t *testing.T) {
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte("[compatibility]\n")}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{err: os.ErrNotExist}, FilePath: "rhsm.conf"}

	got, err := Load(rhcSource, rhsmSource)
	if err != nil {
		t.Fatalf("Load() error = %v, want rhsm.conf absence tolerated", err)
	}
	if got.Subscriptions.Source != SourceDefault {
		t.Errorf("Subscriptions.Source = %q, want %q when rhsm.conf is absent", got.Subscriptions.Source, SourceDefault)
	}
}

func TestLoadPropagatesRHCConfReadError(t *testing.T) {
	rhcSource := config.Source{Filesystem: fakeFS{err: os.ErrPermission}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{err: os.ErrNotExist}, FilePath: "rhsm.conf"}

	_, err := Load(rhcSource, rhsmSource)
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("Load() error = %v, want it to wrap os.ErrPermission", err)
	}
}

func TestLoadPropagatesNonMissingRHSMConfError(t *testing.T) {
	// Unlike a missing rhsm.conf, a real read failure must not be swallowed.
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte("[compatibility]\n")}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{err: os.ErrPermission}, FilePath: "rhsm.conf"}

	_, err := Load(rhcSource, rhsmSource)
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("Load() error = %v, want it to wrap os.ErrPermission", err)
	}
}

func TestLoadIgnoresRHSMConfWhenLegacyInterpretationDisabled(t *testing.T) {
	// If this Satellite-shaped rhsm.conf were consulted, every group below
	// would resolve to its values instead of the embedded default — except
	// http.proxy.user, which rhc.conf sets explicitly.
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{data: []byte(`
[server]
hostname = satellite.example.com
port = 443
prefix = /rhsm
proxy_hostname = corp-proxy.example.com
proxy_scheme = http
proxy_port = 3128

[rhsm]
baseurl = https://satellite.example.com/pulp/content
`)}, FilePath: "rhsm.conf"}
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte(`
[compatibility]
interpret-legacy-configurations = false

[http.proxy]
user = "rhc-conf-user"
`)}, FilePath: "rhc.conf"}

	got, err := Load(rhcSource, rhsmSource)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Proxy.URI != "" || got.Proxy.Source != SourceDefault {
		t.Errorf("Proxy = {URI: %q, Source: %q}, want the rhsm.conf proxy ignored (empty, source=default)", got.Proxy.URI, got.Proxy.Source)
	}
	if got.Proxy.User != "rhc-conf-user" {
		t.Errorf("Proxy.User = %q, want the explicit rhc.conf value to still apply", got.Proxy.User)
	}
	if got.Subscriptions.URI != "https://subscription.rhsm.redhat.com/subscription" || got.Subscriptions.Source != SourceDefault {
		t.Errorf("Subscriptions = {URI: %q, Source: %q}, want rhc.conf's own embedded default, not the Satellite translation", got.Subscriptions.URI, got.Subscriptions.Source)
	}
	if got.RPMContent.URI != "https://cdn.redhat.com" || got.RPMContent.Source != SourceDefault {
		t.Errorf("RPMContent = {URI: %q, Source: %q}, want rhc.conf's own embedded default, not the Satellite translation", got.RPMContent.URI, got.RPMContent.Source)
	}
}

func TestLoadDoesNotReadRHSMConfAtAllWhenLegacyInterpretationDisabled(t *testing.T) {
	// Any read attempt surfaces an error distinct from os.ErrNotExist, so
	// Load() fails loudly if the skip isn't honored.
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte("[compatibility]\ninterpret-legacy-configurations = false\n")}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{err: errors.New("rhsm.conf should not have been read")}, FilePath: "rhsm.conf"}

	if _, err := Load(rhcSource, rhsmSource); err != nil {
		t.Fatalf("Load() error = %v, want rhsm.conf never to be read", err)
	}
}

func TestResolveRejectsInvalidExplicitProxyURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "proxy.example.com:3128"
`)

	_, err := Resolve(file, md, nil)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid http.proxy.uri")
	}
	if !strings.Contains(err.Error(), "http.proxy.uri") {
		t.Errorf("Resolve() error = %v, want it to name http.proxy.uri", err)
	}
}

func TestResolveRejectsInvalidExplicitSubscriptionsURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "ftp://satellite.example.com/subscription"
`)

	_, err := Resolve(file, md, nil)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid api.subscriptions.uri")
	}
	if !strings.Contains(err.Error(), "api.subscriptions.uri") {
		t.Errorf("Resolve() error = %v, want it to name api.subscriptions.uri", err)
	}
}

func TestResolveRejectsInvalidExplicitRPMContentURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.rpm-content]
uri = "not a uri"
`)

	_, err := Resolve(file, md, nil)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid api.rpm-content.uri")
	}
	if !strings.Contains(err.Error(), "api.rpm-content.uri") {
		t.Errorf("Resolve() error = %v, want it to name api.rpm-content.uri", err)
	}
}

func TestResolveProxyEmptyURIDisablesLegacyProxy(t *testing.T) {
	// rhsm.conf configures a full proxy (URI, user, password, no-proxy), but
	// rhc.conf explicitly sets uri = "". This is how an admin opts out of
	// the legacy proxy entirely. It must not fail validation, and it must
	// clear ALL proxy fields, not just the URI.
	rhsm := &rhsmconf.Config{
		ProxyHostname: "proxy.stage.redhat.com",
		ProxyPort:     "3128",
		ProxyScheme:   "http",
		ProxyUser:     "proxyuser",
		ProxyPassword: "proxypass",
		NoProxy:       "localhost,127.0.0.1",
	}
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = ""
`)

	got, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want an explicit empty http.proxy.uri to be accepted", err)
	}
	// Empty URI = complete opt-out; ALL proxy fields should be empty.
	if got.Proxy.URI != "" {
		t.Errorf("Proxy.URI = %q, want empty (legacy proxy disabled)", got.Proxy.URI)
	}
	if got.Proxy.User != "" {
		t.Errorf("Proxy.User = %q, want empty (should not inherit from rhsm.conf)", got.Proxy.User)
	}
	if got.Proxy.Password != "" {
		t.Errorf("Proxy.Password = %q, want empty (should not inherit from rhsm.conf)", got.Proxy.Password)
	}
	if got.Proxy.NoProxy != "" {
		t.Errorf("Proxy.NoProxy = %q, want empty (should not inherit from rhsm.conf)", got.Proxy.NoProxy)
	}
	if got.Proxy.Source != SourceRHCConf {
		t.Errorf("Proxy.Source = %q, want %q", got.Proxy.Source, SourceRHCConf)
	}
}

func TestResolveRejectsInvalidRHSMProxyPortWhenUsed(t *testing.T) {
	// Invalid rhsm.conf proxy_port causes an error ONLY if it would be used
	// (rhc.conf doesn't override).
	rhsm := &rhsmconf.Config{ProxyHostname: "proxy.example.com", ProxyPort: "not-a-port"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid proxy_port from rhsm.conf")
	}
	// url.Parse may catch invalid port syntax first, so just check for "port" in error.
	if !strings.Contains(strings.ToLower(err.Error()), "port") {
		t.Errorf("Resolve() error = %v, want it to mention port", err)
	}
}

func TestResolveAcceptsInvalidRHSMProxyPortWhenOverridden(t *testing.T) {
	// Invalid rhsm.conf proxy_port is ignored if rhc.conf overrides the URI.
	rhsm := &rhsmconf.Config{ProxyHostname: "proxy.example.com", ProxyPort: "not-a-port"}
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "http://good-proxy.example.com:3128"
`)

	got, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil (rhc.conf override should win)", err)
	}
	if got.Proxy.URI != "http://good-proxy.example.com:3128" {
		t.Errorf("Resolve().Proxy.URI = %q, want rhc.conf value", got.Proxy.URI)
	}
	if got.Proxy.Source != SourceRHCConf {
		t.Errorf("Resolve().Proxy.Source = %v, want SourceRHCConf", got.Proxy.Source)
	}
}

func TestResolveRejectsInvalidRHSMSubscriptionsPortWhenUsed(t *testing.T) {
	rhsm := &rhsmconf.Config{Hostname: "satellite.example.com", Port: "not-a-port", Prefix: "/rhsm"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid server.port from rhsm.conf")
	}
	// url.Parse may catch invalid port syntax first, so just check for "port" in error.
	if !strings.Contains(strings.ToLower(err.Error()), "port") {
		t.Errorf("Resolve() error = %v, want it to mention port", err)
	}
}

func TestResolveRejectsInvalidRHSMPrefixWhenUsed(t *testing.T) {
	// Missing leading slash would concatenate directly onto the host,
	// producing a broken URI that parses but routes wrong.
	rhsm := &rhsmconf.Config{Hostname: "satellite.example.com", Port: "443", Prefix: "subscription"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for a prefix missing its leading slash")
	}
	if !strings.Contains(err.Error(), "server.prefix") {
		t.Errorf("Resolve() error = %v, want it to name server.prefix", err)
	}
}

func TestResolveInvalidRHSMPrefixPreservesTLSSettings(t *testing.T) {
	// When rhsm.conf has an invalid prefix but valid TLS settings, and
	// rhc.conf overrides the URI, the TLS settings from rhsm.conf should
	// still be applied (invalid prefix doesn't block the override, and
	// TLS settings shouldn't be lost).
	rhsm := &rhsmconf.Config{
		Hostname:   "satellite.example.com",
		Port:       "443",
		Prefix:     "subscription", // Missing leading slash
		Insecure:   false,
		RepoCACert: "/etc/rhsm/ca/katello-server-ca.pem",
	}
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://my.example/subscription"
`)

	cfg, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want no error when rhc.conf overrides invalid rhsm.conf prefix", err)
	}

	if cfg.Subscriptions.URI != "https://my.example/subscription" {
		t.Errorf("Subscriptions.URI = %q, want rhc.conf URI", cfg.Subscriptions.URI)
	}
	if cfg.Subscriptions.Source != SourceRHCConf {
		t.Errorf("Subscriptions.Source = %q, want %q", cfg.Subscriptions.Source, SourceRHCConf)
	}
	// TLS settings should come from rhsm.conf since rhc.conf didn't explicitly set them
	if !cfg.Subscriptions.TLSVerify {
		t.Errorf("Subscriptions.TLSVerify = false, want true from rhsm.conf (insecure=0)")
	}
	if cfg.Subscriptions.CAPath != "/etc/rhsm/ca/katello-server-ca.pem" {
		t.Errorf("Subscriptions.CAPath = %q, want %q from rhsm.conf", cfg.Subscriptions.CAPath, "/etc/rhsm/ca/katello-server-ca.pem")
	}
}

func TestResolveRejectsInvalidRHSMProxyHostnameWhenUsed(t *testing.T) {
	// validatePort passes (no port set), so this only fails at the
	// constructed-URI validation step.
	rhsm := &rhsmconf.Config{ProxyHostname: "proxy example.com"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid proxy_hostname from rhsm.conf")
	}
}

func TestResolveRejectsInvalidRHSMSubscriptionsHostnameWhenUsed(t *testing.T) {
	rhsm := &rhsmconf.Config{Hostname: "satellite example.com", Prefix: "/rhsm"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid hostname from rhsm.conf")
	}
}

func TestResolvePropagatesProxyTranslationError(t *testing.T) {
	rhsm := &rhsmconf.Config{ProxyHostname: "proxy.example.com", ProxyPort: "not-a-port"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid rhsm.conf proxy_port")
	}
}

func TestResolvePropagatesSubscriptionsTranslationError(t *testing.T) {
	rhsm := &rhsmconf.Config{Hostname: "satellite.example.com", Port: "not-a-port", Prefix: "/rhsm"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid rhsm.conf server.port")
	}
}

func TestResolvePropagatesInvalidRHSMBaseURLError(t *testing.T) {
	// rhsm.conf's baseurl is ingested as a complete URI, not built from
	// discrete parts, so it needs the same validation as rhc.conf's own
	// explicit uri fields.
	rhsm := &rhsmconf.Config{BaseURL: "not-a-valid-baseurl"}
	file, md := decodeRHCConf(t, "[compatibility]\n")

	_, err := Resolve(file, md, rhsm)
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error for an invalid rhsm.conf baseurl")
	}
	if !strings.Contains(err.Error(), "rhsm.baseurl") {
		t.Errorf("Resolve() error = %v, want it to name rhsm.baseurl", err)
	}
}

func TestLoadPropagatesResolveValidationError(t *testing.T) {
	rhcSource := config.Source{Filesystem: fakeFS{data: []byte(`
[api.subscriptions]
uri = "not a uri"
`)}, FilePath: "rhc.conf"}
	rhsmSource := rhsmconf.Source{Filesystem: fakeFS{err: os.ErrNotExist}, FilePath: "rhsm.conf"}

	_, err := Load(rhcSource, rhsmSource)
	if err == nil {
		t.Fatal("Load() error = nil, want an error for an invalid api.subscriptions.uri")
	}
}

func TestSanitizeURIRemovesCredentials(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"user and password", "http://user:pass@proxy.example.com:3128", "http://proxy.example.com:3128"},
		{"user only", "http://user@proxy.example.com:3128", "http://proxy.example.com:3128"},
		{"no credentials", "http://proxy.example.com:3128", "http://proxy.example.com:3128"},
		{"https with credentials", "https://admin:secret@satellite.example.com/rhsm", "https://satellite.example.com/rhsm"},
		{"malformed URI", "not a uri", "not a uri"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeURI(tt.uri)
			if got != tt.want {
				t.Errorf("sanitizeURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestLog(t *testing.T) {
	var buf bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prior)

	cfg := Config{
		Proxy:         Proxy{URI: "http://proxy.example.com:3128", Source: SourceRHSMConf},
		Subscriptions: Endpoint{URI: "https://subscription.rhsm.redhat.com/subscription", Source: SourceDefault},
		RPMContent:    Endpoint{URI: "https://cdn.redhat.com", Source: SourceRHCConf},
	}
	cfg.Log()

	out := buf.String()
	for _, want := range []string{
		"effective proxy config", "source=rhsm.conf", "uri=http://proxy.example.com:3128",
		"effective api.subscriptions", "source=default",
		"effective api.rpm-content", "source=rhc.conf", "uri=https://cdn.redhat.com",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("Log() output missing %q, got:\n%s", want, out)
		}
	}
}

func TestLogDoesNotLeakCredentials(t *testing.T) {
	var buf bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prior)

	cfg := Config{
		Proxy: Proxy{
			URI:      "http://proxyuser:proxypass@proxy.example.com:3128",
			User:     "proxyuser",
			Password: "proxypass",
			Source:   SourceRHSMConf,
		},
		Subscriptions: Endpoint{URI: "https://admin:secret@satellite.example.com/rhsm", Source: SourceRHCConf},
		RPMContent:    Endpoint{URI: "https://cdn.redhat.com", Source: SourceDefault},
	}
	cfg.Log()

	out := buf.String()
	// Verify credentials are NOT in the output
	for _, forbidden := range []string{"proxyuser", "proxypass", "admin", "secret"} {
		if bytes.Contains(buf.Bytes(), []byte(forbidden)) {
			t.Errorf("Log() output contains credential %q, got:\n%s", forbidden, out)
		}
	}
	// Verify sanitized URIs ARE in the output
	for _, want := range []string{
		"uri=http://proxy.example.com:3128",
		"uri=https://satellite.example.com/rhsm",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("Log() output missing %q, got:\n%s", want, out)
		}
	}
}

func TestResolveIndependentFieldPrecedenceURIAndTLS(t *testing.T) {
	// Each field (URI, TLSVerify, CAPath) has independent precedence.
	// rhc.conf can override URI while keeping rhsm.conf's TLS settings,
	// or override TLS while keeping rhsm.conf's URI.
	rhsm := &rhsmconf.Config{
		Hostname:   "satellite.example.com",
		Port:       "443",
		Prefix:     "/rhsm",
		Insecure:   false,
		RepoCACert: "/etc/rhsm/ca/katello.pem",
	}
	file, md := decodeRHCConf(t, `
[api.subscriptions]
tls-verify = false
`)

	cfg, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// URI and CAPath from rhsm.conf (not overridden)
	if cfg.Subscriptions.URI != "https://satellite.example.com/rhsm" {
		t.Errorf("Subscriptions.URI = %q, want URI from rhsm.conf", cfg.Subscriptions.URI)
	}
	if cfg.Subscriptions.CAPath != "/etc/rhsm/ca/katello.pem" {
		t.Errorf("Subscriptions.CAPath = %q, want CAPath from rhsm.conf", cfg.Subscriptions.CAPath)
	}
	// TLSVerify from rhc.conf (explicitly overridden)
	if cfg.Subscriptions.TLSVerify {
		t.Error("Subscriptions.TLSVerify = true, want false from rhc.conf explicit override")
	}
	// Source tracks URI origin
	if cfg.Subscriptions.Source != SourceRHSMConf {
		t.Errorf("Subscriptions.Source = %q, want %q (URI not overridden)", cfg.Subscriptions.Source, SourceRHSMConf)
	}
}

func TestResolveInvalidRHSMPrefixWithExplicitRHCConfTLS(t *testing.T) {
	// When rhsm.conf has an invalid prefix AND rhc.conf overrides both
	// URI and TLS settings, all values should come from rhc.conf.
	rhsm := &rhsmconf.Config{
		Hostname:   "satellite.example.com",
		Port:       "443",
		Prefix:     "subscription", // Missing leading slash
		Insecure:   false,
		RepoCACert: "/etc/rhsm/ca/katello.pem",
	}
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://my.example/subscription"
tls-verify = false
ca-path = "/etc/pki/my-ca.pem"
`)

	cfg, err := Resolve(file, md, rhsm)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// All values from rhc.conf (all explicitly overridden)
	if cfg.Subscriptions.URI != "https://my.example/subscription" {
		t.Errorf("Subscriptions.URI = %q, want rhc.conf URI", cfg.Subscriptions.URI)
	}
	if cfg.Subscriptions.TLSVerify {
		t.Error("Subscriptions.TLSVerify = true, want false from rhc.conf")
	}
	if cfg.Subscriptions.CAPath != "/etc/pki/my-ca.pem" {
		t.Errorf("Subscriptions.CAPath = %q, want rhc.conf CAPath", cfg.Subscriptions.CAPath)
	}
	if cfg.Subscriptions.Source != SourceRHCConf {
		t.Errorf("Subscriptions.Source = %q, want %q", cfg.Subscriptions.Source, SourceRHCConf)
	}
}
