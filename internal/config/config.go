package config

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	rhcfs "github.com/redhatinsights/rhc/internal/fs"
)

const (
	defaultSubscriptionsURI = "https://subscription.rhsm.redhat.com/subscription"
	defaultRPMContentURI    = "https://cdn.redhat.com"
	defaultIngressURI       = "https://cert.console.redhat.com/api/ingress/v1/upload"
	defaultInventoryURI     = "https://cert.console.redhat.com/api/inventory/v1/hosts"
)

type Source struct {
	Filesystem rhcfs.FS
	FilePath   string
}

type File struct {
	Compatibility Compatibility `toml:"compatibility"`
	HTTP          HTTP          `toml:"http"`
	API           API           `toml:"api"`
}

type Compatibility struct {
	InterpretLegacy bool `toml:"interpret-legacy-configurations"`
}

type HTTP struct {
	Timeout Timeout `toml:"timeout"`
	Proxy   Proxy   `toml:"proxy"`
}

type Timeout struct {
	Connect DurationSeconds `toml:"connect"`
	Request DurationSeconds `toml:"request"`
	Idle    DurationSeconds `toml:"idle"`
}

type Proxy struct {
	URI      string `toml:"uri"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	NoProxy  string `toml:"no-proxy"`
}

type API struct {
	Subscriptions Endpoint `toml:"subscriptions"`
	Content       Content  `toml:"content"`
	Insights      Insights `toml:"insights"`
}

type Endpoint struct {
	URI       string `toml:"uri"`
	TLSVerify bool   `toml:"tls-verify"`
	CAPath    string `toml:"ca-path"`
}

type Content struct {
	RPM Endpoint `toml:"rpm"`
}

type Insights struct {
	Ingress   Endpoint `toml:"ingress"`
	Inventory Endpoint `toml:"inventory"`
}

type DurationSeconds time.Duration

func (d *DurationSeconds) UnmarshalText(text []byte) error {
	n, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return fmt.Errorf("expected a whole number of seconds: %q", text)
	}
	if n < 0 {
		return fmt.Errorf("seconds must not be negative: %d", n)
	}
	if n > int64(math.MaxInt64/time.Second) {
		return fmt.Errorf("seconds out of range: %d", n)
	}
	*d = DurationSeconds(time.Duration(n) * time.Second)
	return nil
}

func (d DurationSeconds) String() string {
	return time.Duration(d).String()
}

// Get reads the configuration file and returns a File structure.
func Get(source Source) (File, error) {
	data, err := source.Filesystem.Read(source.FilePath)
	if err != nil {
		return File{}, fmt.Errorf("read main configuration file: %w", err)
	}
	return decode(data)
}

func decode(data []byte) (File, error) {
	var file File
	md, err := toml.Decode(string(data), &file)
	if err != nil {
		return File{}, err
	}

	if err := unknownKeys(md); err != nil {
		return File{}, err
	}

	applyEndpointDefaults(md, &file.API.Subscriptions, []string{"api", "subscriptions"}, defaultSubscriptionsURI)
	applyEndpointDefaults(md, &file.API.Content.RPM, []string{"api", "content", "rpm"}, defaultRPMContentURI)
	applyEndpointDefaults(md, &file.API.Insights.Ingress, []string{"api", "insights", "ingress"}, defaultIngressURI)
	applyEndpointDefaults(md, &file.API.Insights.Inventory, []string{"api", "insights", "inventory"}, defaultInventoryURI)

	if !md.IsDefined("compatibility", "interpret-legacy-configurations") {
		file.Compatibility.InterpretLegacy = true
	}
	if !md.IsDefined("http", "timeout", "connect") {
		file.HTTP.Timeout.Connect = DurationSeconds(30 * time.Second)
	}
	if !md.IsDefined("http", "timeout", "request") {
		file.HTTP.Timeout.Request = DurationSeconds(180 * time.Second)
	}
	if !md.IsDefined("http", "timeout", "idle") {
		file.HTTP.Timeout.Idle = DurationSeconds(120 * time.Second)
	}

	return file, nil
}

func unknownKeys(md toml.MetaData) error {
	allowed := map[string]struct{}{
		"compatibility":          {},
		"http":                   {},
		"http.timeout":           {},
		"http.proxy":             {},
		"api":                    {},
		"api.subscriptions":      {},
		"api.content":            {},
		"api.content.rpm":        {},
		"api.insights":           {},
		"api.insights.ingress":   {},
		"api.insights.inventory": {},
	}
	for _, key := range md.Undecoded() {
		name := key.String()
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("unknown configuration key %q", name)
		}
	}
	return nil
}

func applyEndpointDefaults(md toml.MetaData, endpoint *Endpoint, key []string, uri string) {
	if !md.IsDefined(append(append([]string{}, key...), "uri")...) {
		endpoint.URI = uri
	}
	if !md.IsDefined(append(append([]string{}, key...), "tls-verify")...) {
		endpoint.TLSVerify = true
	}
}
