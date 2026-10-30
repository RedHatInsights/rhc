package rhcconf

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
ca-path = "/etc/rhsm/ca/katello-server-ca.pem"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	want := []RHSMSetting{
		{Key: "server.hostname", Value: "satellite.example.com"},
		{Key: "server.port", Value: "443"},
		{Key: "server.prefix", Value: "/subscription"},
		{Key: "rhsm.repo_ca_cert", Value: "/etc/rhsm/ca/katello-server-ca.pem"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RHSMSettingsFromRHCConf() mismatch (-want +got):\n%s", diff)
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

func TestRHSMSettingsFromRHCConfSystemCAPathIsNotWritten(t *testing.T) {
	file, md := decodeRHCConf(t, `
[api.subscriptions]
uri = "https://subscription.rhsm.redhat.com/subscription"
ca-path = "system"
`)

	got, err := RHSMSettingsFromRHCConf(file, md)
	if err != nil {
		t.Fatalf("RHSMSettingsFromRHCConf() error = %v", err)
	}
	for _, setting := range got {
		if setting.Key == "rhsm.repo_ca_cert" {
			t.Errorf("rhsm.repo_ca_cert = %q, want the system sentinel omitted", setting.Value)
		}
	}
	if len(got) != 3 {
		t.Errorf("RHSMSettingsFromRHCConf() = %+v, want hostname, port, and prefix only", got)
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
