// Package rhsmconf reads the rhc-relevant fields out of subscription-manager's
// /etc/rhsm/rhsm.conf directly from disk.
package rhsmconf

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/ini.v1"

	rhcfs "github.com/redhatinsights/rhc/internal/fs"
)

// DefaultPath is the standard location of subscription-manager's configuration file.
const DefaultPath = "/etc/rhsm/rhsm.conf"

type Source struct {
	Filesystem rhcfs.FS
	FilePath   string
}

// Config holds the subset of rhsm.conf fields rhc needs to resolve its effective configuration.
type Config struct {
	// From [server].
	Hostname      string
	Port          string
	Prefix        string
	Insecure      bool
	ProxyHostname string
	ProxyScheme   string
	ProxyPort     string
	ProxyUser     string
	ProxyPassword string
	NoProxy       string

	// From [rhsm].
	BaseURL    string
	RepoCACert string
}

// interpolationPattern matches Python ConfigParser's %(name)s substitution syntax.
var interpolationPattern = regexp.MustCompile(`%\(([^)]+)\)s`)

// Load reads and parses the rhsm.conf file identified by source, resolving %(name)s interpolation.
func Load(source Source) (*Config, error) {
	data, err := source.Filesystem.Read(source.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", source.FilePath, err)
	}
	return parseConfig(data)
}

var loadOptions = ini.LoadOptions{IgnoreInlineComment: true, PreserveSurroundedQuote: true}

func parseConfig(data []byte) (*Config, error) {
	f, err := ini.LoadSources(loadOptions, data)
	if err != nil {
		return nil, fmt.Errorf("parsing rhsm.conf: %w", err)
	}

	server := f.Section("server")
	rhsm := f.Section("rhsm")

	insecure, err := parseBool(server, "insecure")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Hostname:      server.Key("hostname").Value(),
		Port:          server.Key("port").Value(),
		Prefix:        server.Key("prefix").Value(),
		Insecure:      insecure,
		ProxyHostname: server.Key("proxy_hostname").Value(),
		ProxyScheme:   server.Key("proxy_scheme").Value(),
		ProxyPort:     server.Key("proxy_port").Value(),
		ProxyUser:     server.Key("proxy_user").Value(),
		ProxyPassword: server.Key("proxy_password").Value(),
		NoProxy:       server.Key("no_proxy").Value(),
		BaseURL:       rhsm.Key("baseurl").Value(),
		RepoCACert:    rhsm.Key("repo_ca_cert").Value(),
	}

	cfg.RepoCACert, err = interpolate(cfg.RepoCACert, rhsm)
	if err != nil {
		return nil, fmt.Errorf("resolving rhsm.repo_ca_cert: %w", err)
	}

	return cfg, nil
}

const maxSubstitutions = 99

// interpolate resolves %(name)s references in value against raw key values in section
func interpolate(value string, section *ini.Section) (string, error) {
	count := 0
	return substitute(value, section, map[string]bool{}, &count)
}

// substitute recursively resolves %(name)s references and detects cycles
func substitute(value string, section *ini.Section, chain map[string]bool, count *int) (string, error) {
	for {
		match := interpolationPattern.FindStringSubmatch(value)
		if match == nil {
			return value, nil
		}

		name := match[1]
		if chain[name] {
			return "", fmt.Errorf("interpolation cycle referencing %%(%s)s", name)
		}
		if !section.HasKey(name) {
			return "", fmt.Errorf("undefined reference %%(%s)s", name)
		}

		*count++
		if *count > maxSubstitutions {
			return "", fmt.Errorf("exceeded %d interpolation substitutions resolving %%(%s)s", maxSubstitutions, name)
		}

		chain[name] = true
		resolved, err := substitute(section.Key(name).Value(), section, chain, count)
		delete(chain, name)
		if err != nil {
			return "", err
		}

		value = strings.Replace(value, match[0], resolved, 1)
	}
}

func parseBool(section *ini.Section, name string) (bool, error) {
	if !section.HasKey(name) {
		return false, nil
	}
	v, err := section.Key(name).Bool()
	if err != nil {
		return false, fmt.Errorf("%s.%s: %w", section.Name(), name, err)
	}
	return v, nil
}
