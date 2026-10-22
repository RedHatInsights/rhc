package rhsmconf

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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

// hostedFixture mirrors subscription-manager's stock, unmodified
// /etc/rhsm/rhsm.conf (etc-conf/rhsm.conf): hosted Red Hat, no proxy, and
// repo_ca_cert relying on %(ca_cert_dir)s interpolation.
const hostedFixture = `
[server]
hostname = subscription.rhsm.redhat.com
prefix = /subscription
port = 443
insecure = 0
proxy_hostname =
proxy_scheme = http
proxy_port =
proxy_user =
proxy_password =
no_proxy =

[rhsm]
baseurl = https://cdn.redhat.com
ca_cert_dir = /etc/rhsm/ca/
repo_ca_cert = %(ca_cert_dir)sredhat-uep.pem
`

// stageFixture is a Red Hat stage registration with a forward proxy
// configured, to prove proxy fields resolve end to end.
const stageFixture = `
[server]
hostname = subscription.rhsm.stage.redhat.com
prefix = /subscription
port = 443
insecure = 0
proxy_hostname = corp-proxy.example.com
proxy_scheme = http
proxy_port = 3128
proxy_user = proxyuser
proxy_password = proxypass
no_proxy = localhost,127.0.0.1,.internal.example.com

[rhsm]
baseurl = https://cdn.redhat.com
ca_cert_dir = /etc/rhsm/ca/
repo_ca_cert = %(ca_cert_dir)sredhat-uep.pem
`

// satelliteFixture has a hostname that is not under redhat.com and an
// explicit Katello CA path with no interpolation, matching how a
// Satellite-registered system's rhsm.conf looks.
const satelliteFixture = `
[server]
hostname = satellite.example.com
prefix = /rhsm
port = 443
insecure = 0
proxy_hostname =
proxy_scheme = http
proxy_port =
proxy_user =
proxy_password =
no_proxy =

[rhsm]
baseurl = https://satellite.example.com/pulp/content
ca_cert_dir = /etc/rhsm/ca/
repo_ca_cert = /etc/rhsm/ca/katello-server-ca.pem
`

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Config
	}{
		{
			name:    "hosted fixture resolves hostname, port, prefix, empty proxy, and interpolated CA path",
			content: hostedFixture,
			want: Config{
				Hostname:      "subscription.rhsm.redhat.com",
				Port:          "443",
				Prefix:        "/subscription",
				Insecure:      false,
				ProxyHostname: "",
				ProxyScheme:   "http",
				ProxyPort:     "",
				ProxyUser:     "",
				ProxyPassword: "",
				NoProxy:       "",
				BaseURL:       "https://cdn.redhat.com",
				RepoCACert:    "/etc/rhsm/ca/redhat-uep.pem",
			},
		},
		{
			name:    "stage fixture resolves hostname, port, prefix, a configured proxy, and interpolated CA path",
			content: stageFixture,
			want: Config{
				Hostname:      "subscription.rhsm.stage.redhat.com",
				Port:          "443",
				Prefix:        "/subscription",
				Insecure:      false,
				ProxyHostname: "corp-proxy.example.com",
				ProxyScheme:   "http",
				ProxyPort:     "3128",
				ProxyUser:     "proxyuser",
				ProxyPassword: "proxypass",
				NoProxy:       "localhost,127.0.0.1,.internal.example.com",
				BaseURL:       "https://cdn.redhat.com",
				RepoCACert:    "/etc/rhsm/ca/redhat-uep.pem",
			},
		},
		{
			name:    "Satellite-shaped fixture resolves hostname, port, prefix, empty proxy, and the literal Katello CA path",
			content: satelliteFixture,
			want: Config{
				Hostname:      "satellite.example.com",
				Port:          "443",
				Prefix:        "/rhsm",
				Insecure:      false,
				ProxyHostname: "",
				ProxyScheme:   "http",
				ProxyPort:     "",
				ProxyUser:     "",
				ProxyPassword: "",
				NoProxy:       "",
				BaseURL:       "https://satellite.example.com/pulp/content",
				RepoCACert:    "/etc/rhsm/ca/katello-server-ca.pem",
			},
		},
		{
			name: "insecure = 1 resolves to true",
			content: `
[server]
hostname = satellite.example.com
insecure = 1
`,
			want: Config{
				Hostname: "satellite.example.com",
				Insecure: true,
			},
		},
		{
			name: "a non-default ca_cert_dir still resolves generically through interpolation",
			content: `
[rhsm]
ca_cert_dir = /custom/ca/
repo_ca_cert = %(ca_cert_dir)sredhat-uep.pem
`,
			want: Config{
				RepoCACert: "/custom/ca/redhat-uep.pem",
			},
		},
		{
			name: "the same key referenced twice in one value is not a cycle",
			content: `
[rhsm]
dir = /etc/pki/
repo_ca_cert = %(dir)s%(dir)s
`,
			want: Config{
				RepoCACert: "/etc/pki//etc/pki/",
			},
		},
		{
			name: "an explicitly empty baseurl stays empty and is not an error",
			content: `
[rhsm]
baseurl =
repo_ca_cert = /etc/rhsm/ca/redhat-uep.pem
`,
			want: Config{
				RepoCACert: "/etc/rhsm/ca/redhat-uep.pem",
			},
		},
		{
			name: "a missing baseurl key stays empty and is not an error",
			content: `
[rhsm]
repo_ca_cert = /etc/rhsm/ca/redhat-uep.pem
`,
			want: Config{
				RepoCACert: "/etc/rhsm/ca/redhat-uep.pem",
			},
		},
		{
			name: "a # character in proxy_password is kept as literal value content, not truncated as a comment",
			content: `
[server]
proxy_password = cor#rect
`,
			want: Config{
				ProxyPassword: "cor#rect",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConfig([]byte(test.content))
			if err != nil {
				t.Fatalf("parseConfig() unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.want, *got); diff != "" {
				t.Errorf("parseConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseConfigInvalidSyntax(t *testing.T) {
	_, err := parseConfig([]byte("[server\nhostname = broken"))
	if err == nil {
		t.Fatal("parseConfig() expected an error for malformed INI syntax, got none")
	}
}

func TestParseConfigInvalidInsecure(t *testing.T) {
	_, err := parseConfig([]byte("[server]\ninsecure = tru\n"))
	if err == nil {
		t.Fatal("parseConfig() expected an error for an invalid insecure value, got none")
	}
}

func TestParseConfigUndefinedInterpolationReference(t *testing.T) {
	_, err := parseConfig([]byte("[rhsm]\nrepo_ca_cert = %(missing)sredhat-uep.pem\n"))
	if err == nil {
		t.Fatal("parseConfig() expected an error for an undefined interpolation reference, got none")
	}
}

func TestParseConfigInterpolationCycle(t *testing.T) {
	_, err := parseConfig([]byte("[rhsm]\na = %(b)s\nb = %(a)s\nrepo_ca_cert = %(a)s\n"))
	if err == nil {
		t.Fatal("parseConfig() expected an error for an interpolation cycle, got none")
	}
}

// A value whose own expansion ends in an unterminated '%' can splice against surrounding
// literal text to form a new matchable %(name)s reference that the chain-based cycle check
// (which only tracks genuine key references) does not see coming, since no key actually
// references itself or another key in a loop. Stacking enough trailing (a)s fragments drives
// the substitution count past maxSubstitutions, which is the independent backstop for this case.
func TestParseConfigInterpolationExcessiveSubstitutions(t *testing.T) {
	tail := strings.Repeat("(a)s", maxSubstitutions+1)
	content := []byte("[rhsm]\na = some-string%\nrepo_ca_cert = %(a)s" + tail + "\n")

	_, err := parseConfig(content)
	if err == nil {
		t.Fatal("parseConfig() expected an error when the substitution limit is exceeded, got none")
	}
}

func TestLoad(t *testing.T) {
	t.Run("reads and parses the file at the given path", func(t *testing.T) {
		fsys := &fakeFS{data: []byte(hostedFixture)}

		got, err := Load(Source{Filesystem: fsys, FilePath: DefaultPath})
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if fsys.path != DefaultPath {
			t.Errorf("Read() path = %q, want %q", fsys.path, DefaultPath)
		}
		if got.Hostname != "subscription.rhsm.redhat.com" {
			t.Errorf("Load().Hostname = %q, want %q", got.Hostname, "subscription.rhsm.redhat.com")
		}
		if got.RepoCACert != "/etc/rhsm/ca/redhat-uep.pem" {
			t.Errorf("Load().RepoCACert = %q, want %q", got.RepoCACert, "/etc/rhsm/ca/redhat-uep.pem")
		}
	})

	t.Run("propagates a filesystem read error, not a panic", func(t *testing.T) {
		fsys := &fakeFS{err: os.ErrNotExist}

		_, err := Load(Source{Filesystem: fsys, FilePath: DefaultPath})
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Load() error = %v, want wrapped os.ErrNotExist", err)
		}
	})
}
