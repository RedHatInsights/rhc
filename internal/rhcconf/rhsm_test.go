package rhcconf

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// writeCACert creates a mock CA file in a temp dir and returns its path and the
// directory (with trailing separator) that ca_cert_dir should resolve to.
func writeCACert(t *testing.T) (caPath, caDir string) {
	t.Helper()
	dir := t.TempDir()
	caPath = filepath.Join(dir, "katello-server-ca.pem")
	if err := os.WriteFile(caPath, []byte("mock"), 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return caPath, dir + string(os.PathSeparator)
}

func TestRHSMSettingsFromRHCConfCommentedFileIsEmpty(t *testing.T) {
	shipped, err := os.ReadFile("../../data/config/rhc.conf")
	if err != nil {
		t.Fatalf("read packaged rhc.conf: %v", err)
	}
	file, md := decodeRHCConf(t, string(shipped))

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("RHSMSettingsFromRHCConf() = %+v, want no keys for a commented file", got)
	}
}

func TestRHSMSettingsFromRHCConfSatelliteRegistrationTarget(t *testing.T) {
	caPath, caDir := writeCACert(t)
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
ca-path = "`+caPath+`"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.hostname", Value: "satellite.example.com"},
		{Key: "server.port", Value: "443"},
		{Key: "server.prefix", Value: "/subscription"},
		{Key: "rhsm.repo_ca_cert", Value: caPath},
		{Key: "rhsm.ca_cert_dir", Value: caDir},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfRejectsMissingCAPath(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
ca-path = "/etc/rhsm/ca/does-not-exist.pem"
`)

	if _, err := RHSMSettingsFromRHCConf(file, md); err == nil {
		t.Fatal("RHSMSettingsFromRHCConf() error = nil, want a missing ca-path failure")
	}
}

func TestRHSMSettingsFromRHCConfNonDefaultPortAndPrefix(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://satellite.example.com:8443/rhsm"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.hostname", Value: "satellite.example.com"},
		{Key: "server.port", Value: "8443"},
		{Key: "server.prefix", Value: "/rhsm"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfTLSVerify(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
tls-verify = false
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{{Key: "server.insecure", Value: "1"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}

	file, md = decodeRHCConf(t, `
[api.subscriptions]
tls-verify = true
`)
	got, err = RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want = []RHSMSetting{{Key: "server.insecure", Value: "0"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfSystemCAPathUsesSystemBundle(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://subscription.rhsm.redhat.com/subscription"
ca-path = "system"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.hostname", Value: "subscription.rhsm.redhat.com"},
		{Key: "server.port", Value: "443"},
		{Key: "server.prefix", Value: "/subscription"},
		{Key: "rhsm.repo_ca_cert", Value: systemCABundle},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfProxyURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "http://proxy.example.com:3128"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_scheme", Value: "http"},
		{Key: "server.proxy_hostname", Value: "proxy.example.com"},
		{Key: "server.proxy_port", Value: "3128"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfProxyURIWithoutPortClearsPort(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "http://proxy.example.com"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_scheme", Value: "http"},
		{Key: "server.proxy_hostname", Value: "proxy.example.com"},
		{Key: "server.proxy_port", Value: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfEmptyProxyURIClearsProxy(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = ""
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_scheme", Value: "http"},
		{Key: "server.proxy_hostname", Value: ""},
		{Key: "server.proxy_port", Value: ""},
		{Key: "server.proxy_user", Value: ""},
		{Key: "server.proxy_password", Value: ""},
		{Key: "server.no_proxy", Value: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfEmptyProxyURIKeepsExplicitUser(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = ""
user = "proxyuser"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_scheme", Value: "http"},
		{Key: "server.proxy_hostname", Value: ""},
		{Key: "server.proxy_port", Value: ""},
		{Key: "server.proxy_user", Value: "proxyuser"},
		{Key: "server.proxy_password", Value: ""},
		{Key: "server.no_proxy", Value: ""},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfProxyCredentials(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
user = "proxyuser"
password = "proxypass"
no-proxy = "localhost"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_user", Value: "proxyuser"},
		{Key: "server.proxy_password", Value: "proxypass"},
		{Key: "server.no_proxy", Value: "localhost"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfRPMContentURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.rpm-content]
uri = "https://satellite.example.com/pulp/content"
tls-verify = false
ca-path = "/etc/rhsm/ca/katello-server-ca.pem"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "rhsm.baseurl", Value: "https://satellite.example.com/pulp/content"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}

	file, md = decodeRHCConf(t, `
[api.rpm-content]
uri = "http://cdn.example"
`)
	got, err = RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want = []RHSMSetting{{Key: "rhsm.baseurl", Value: "http://cdn.example"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfIgnoresInsightsAndTimeouts(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.timeout]
connect = 10

[api.insights.ingress]
uri = "https://cert.console.redhat.com/api/ingress/v1/upload"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("RHSMSettingsFromRHCConf() = %+v, want no rhsm.conf keys", got)
	}
}

func TestRHSMSettingsFromRHCConfRejectsUnusableURI(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "http://satellite.example.com/subscription"
`)

	if _, err := RHSMSettingsFromRHCConf(file, md); err == nil {
		t.Fatal("RHSMSettingsFromRHCConf() error = nil, want https required")
	}

	file, md = decodeRHCConf(t, `
[http.proxy]
uri = "proxy.example.com:3128"
`)
	if _, err := RHSMSettingsFromRHCConf(file, md); err == nil {
		t.Fatal("RHSMSettingsFromRHCConf() error = nil, want a proxy scheme and host")
	}

	file, md = decodeRHCConf(t, `
[api.rpm-content]
uri = "cdn.example/content"
`)
	if _, err := RHSMSettingsFromRHCConf(file, md); err == nil {
		t.Fatal("RHSMSettingsFromRHCConf() error = nil, want an absolute http or https URI")
	}
}

func TestRHSMSettingsFromRHCConfRejectsInvalidSubscriptionsURIs(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "missing host", uri: "https:///subscription"},
		{name: "userinfo", uri: "https://user:password@satellite.example.com/subscription"},
		{name: "query", uri: "https://satellite.example.com/subscription?debug=true"},
		{name: "fragment", uri: "https://satellite.example.com/subscription#fragment"},
		{name: "invalid port", uri: "https://satellite.example.com:70000/subscription"},
		{name: "unparseable URI", uri: "https://satellite.example.com:bad port/subscription"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, md := decodeRHCConf(t, "[api.subscriptions]\nuri = "+strconv.Quote(tt.uri)+"\n")
			_, err := RHSMSettingsFromRHCConf(file, md)
			if err == nil {
				t.Fatalf("RHSMSettingsFromRHCConf(%q) error = nil, want rejection", tt.uri)
			}
		})
	}
}

func TestRHSMSettingsFromRHCConfRejectsDirectoryAsCAPath(t *testing.T) {
	dir := t.TempDir()
	file, md := decodeRHCConf(t, "[api.subscriptions]\nca-path = "+strconv.Quote(dir)+"\n")

	_, err := RHSMSettingsFromRHCConf(file, md)
	if err == nil {
		t.Fatal("RHSMSettingsFromRHCConf() error = nil, want a directory ca-path failure")
	}
}

func TestRHSMSettingsFromRHCConfCombinesExplicitProxySettings(t *testing.T) {
	file, md := decodeRHCConf(t, `
[http.proxy]
uri = "https://proxy.example.com:8443"
user = "proxyuser"
password = "proxypass"
no-proxy = "localhost,127.0.0.1"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.proxy_scheme", Value: "https"},
		{Key: "server.proxy_hostname", Value: "proxy.example.com"},
		{Key: "server.proxy_port", Value: "8443"},
		{Key: "server.proxy_user", Value: "proxyuser"},
		{Key: "server.proxy_password", Value: "proxypass"},
		{Key: "server.no_proxy", Value: "localhost,127.0.0.1"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
	}
}

func TestRHSMSettingsFromRHCConfRejectsInvalidProxyURIs(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "unsupported scheme", uri: "ftp://proxy.example.com"},
		{name: "out-of-range port", uri: "http://proxy.example.com:70000"},
		{name: "unparseable port", uri: "http://proxy.example.com:bad port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, md := decodeRHCConf(t, "[http.proxy]\nuri = "+strconv.Quote(tt.uri)+"\n")
			_, err := RHSMSettingsFromRHCConf(file, md)
			if err == nil {
				t.Fatalf("RHSMSettingsFromRHCConf(%q) error = nil, want rejection", tt.uri)
			}
		})
	}
}

func TestRHSMSettingsFromRHCConfRedactsProxyCredentials(t *testing.T) {
	const secret = "s3cret"
	// A proxy URI with embedded credentials is rejected (userinfo is not
	// allowed); the error must not echo the password. Both a parseable URI and
	// one that fails url.Parse are covered.
	for _, uri := range []string{
		"http://admin:" + secret + "@proxy.example.com:3128",
		"http://admin:" + secret + "@proxy.example.com:31 28",
	} {
		file, md := decodeRHCConf(t, "[http.proxy]\nuri = \""+uri+"\"\n")
		_, err := RHSMSettingsFromRHCConf(file, md)
		if err == nil {
			t.Fatalf("RHSMSettingsFromRHCConf(%q) error = nil, want rejection", uri)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error leaks password: %v", err)
		}
	}
}
