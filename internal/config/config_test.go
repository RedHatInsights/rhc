package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

type fakeFS struct {
	data []byte
	err  error
	path string
}

func (f *fakeFS) Read(path string) ([]byte, error) {
	f.path = path
	if f.err != nil {
		return nil, f.err
	}
	return f.data, nil
}

func TestGetReadsSource(t *testing.T) {
	const path = "/etc/rhc/rhc.conf"
	fs := &fakeFS{data: []byte("[compatibility]\ninterpret-legacy-configurations = false\n")}
	file, err := Get(Source{Filesystem: fs, FilePath: path})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fs.path != path {
		t.Errorf("Read path = %q, want %q", fs.path, path)
	}
	if file.Compatibility.InterpretLegacy {
		t.Error("InterpretLegacy = true, want false")
	}
	if got, want := time.Duration(file.HTTP.Timeout.Connect), 30*time.Second; got != want {
		t.Errorf("Connect = %v, want %v", got, want)
	}
}

func TestGetWrapsReadError(t *testing.T) {
	fs := &fakeFS{err: os.ErrNotExist}
	_, err := Get(Source{Filesystem: fs, FilePath: "/etc/rhc/rhc.conf"})
	if err == nil {
		t.Fatal("Get() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "read main configuration file") {
		t.Errorf("Get() error = %q, want read wrapper", err)
	}
}

func TestDecodeUnknownKeys(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
	}{
		{
			name:    "unknown section",
			input:   "[not-a-section]\nfoo = 1\n",
			wantKey: "not-a-section",
		},
		{
			name:    "unknown key",
			input:   "[http.timeout]\nnot-a-timeout = 1\n",
			wantKey: "http.timeout.not-a-timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decode([]byte(tt.input))
			if err == nil {
				t.Fatal("decode() error = nil, want unknown key")
			}
			if !strings.Contains(err.Error(), tt.wantKey) {
				t.Errorf("decode() error = %q, want key %q", err, tt.wantKey)
			}
		})
	}
}

func TestDecodeUsesDefaults(t *testing.T) {
	config, err := os.ReadFile("../../data/config/rhc.conf")
	if err != nil {
		t.Fatalf("read rhc.conf: %v", err)
	}
	file, err := decode(config)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}

	if !file.Compatibility.InterpretLegacy {
		t.Error("InterpretLegacy = false, want true")
	}
	if got, want := time.Duration(file.HTTP.Timeout.Connect), 30*time.Second; got != want {
		t.Errorf("Connect = %v, want %v", got, want)
	}
	if got, want := time.Duration(file.HTTP.Timeout.Request), 180*time.Second; got != want {
		t.Errorf("Request = %v, want %v", got, want)
	}
	if got, want := time.Duration(file.HTTP.Timeout.Idle), 120*time.Second; got != want {
		t.Errorf("Idle = %v, want %v", got, want)
	}
	if got, want := file.API.Subscriptions.URI, defaultSubscriptionsURI; got != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got, want)
	}
	if got, want := file.API.Content.RPM.URI, defaultRPMContentURI; got != want {
		t.Errorf("Content.RPM.URI = %q, want %q", got, want)
	}
	if got, want := file.API.Insights.Ingress.URI, defaultIngressURI; got != want {
		t.Errorf("Insights.Ingress.URI = %q, want %q", got, want)
	}
	if got, want := file.API.Insights.Inventory.URI, defaultInventoryURI; got != want {
		t.Errorf("Insights.Inventory.URI = %q, want %q", got, want)
	}
	if !file.API.Subscriptions.TLSVerify || !file.API.Insights.Ingress.TLSVerify {
		t.Error("TLSVerify default = false, want true")
	}
}

func TestDecodeKeepsValues(t *testing.T) {
	config := `[compatibility]
interpret-legacy-configurations = false

[http.timeout]
connect = 0
request = 15
idle = 45

[api.subscriptions]
uri = "https://satellite.example/subscription"
tls-verify = false
ca-path = "/etc/rhsm/ca/katello.pem"
`
	file, err := decode([]byte(config))
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}

	if file.Compatibility.InterpretLegacy {
		t.Error("InterpretLegacy = true, want false")
	}
	if got := time.Duration(file.HTTP.Timeout.Connect); got != 0 {
		t.Errorf("Connect = %v, want 0", got)
	}
	if got, want := time.Duration(file.HTTP.Timeout.Request), 15*time.Second; got != want {
		t.Errorf("Request = %v, want %v", got, want)
	}
	if got, want := time.Duration(file.HTTP.Timeout.Idle), 45*time.Second; got != want {
		t.Errorf("Idle = %v, want %v", got, want)
	}
	if got, want := file.API.Subscriptions.URI, "https://satellite.example/subscription"; got != want {
		t.Errorf("Subscriptions.URI = %q, want %q", got, want)
	}
	if file.API.Subscriptions.TLSVerify {
		t.Error("Subscriptions.TLSVerify = true, want false")
	}
	if got, want := file.API.Subscriptions.CAPath, "/etc/rhsm/ca/katello.pem"; got != want {
		t.Errorf("Subscriptions.CAPath = %q, want %q", got, want)
	}
	if got, want := file.API.Insights.Ingress.URI, defaultIngressURI; got != want {
		t.Errorf("Insights.Ingress.URI = %q, want default %q", got, want)
	}
}

func TestDecodeInvalidTimeout(t *testing.T) {
	_, err := decode([]byte("[http.timeout]\nconnect = \"30s\"\n"))
	if err == nil {
		t.Fatal("decode() error = nil, want integer seconds")
	}
}
