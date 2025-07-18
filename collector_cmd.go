package main

const (
	collectorDirName = "/usr/lib/rhc/collector.d"
)

// CollectorInfo holds information about collector
type CollectorInfo struct {
	configFilePath string // Configuration file path
	id             string // Get from configuration file name
	Meta           struct {
		Name    string `json:"name" toml:"name"`
		Feature string `json:"feature,omitempty" toml:"feature,omitempty"`
	} `json:"meta" toml:"meta"`
	Exec struct {
		VersionCommand string `json:"version_command" toml:"version_command"`
		Collector      struct {
			Command string `json:"command" toml:"command"`
		}
		Archiver struct {
			Command string `json:"command" toml:"command"`
		}
		Uploader struct {
			Command string `json:"command" toml:"command"`
		}
	} `json:"exec" toml:"exec"`
	Systemd struct {
		Service string `json:"service" toml:"service"`
		Timer   string `json:"timer" toml:"timer"`
	} `json:"systemd" toml:"systemd"`
}
