package main

import "path/filepath"

var (
	// Version is the version as described by git.
	Version string

	// ShortName is used as a prefix to binary file names.
	ShortName string

	// LongName is used in file and directory names.
	LongName string

	// TopicPrefix is used as a prefix to all MQTT topics in the client.
	TopicPrefix string

	// Provider is used when constructing user-facing string output to identify
	// the agency providing the connection broker.
	Provider string

	// ServiceName us used for manipulating of yggdrasil service
	// It can be branded to rhcd on RHEL
	ServiceName string
)

// Installation directory prefix and paths. Values are specified by compile-time
// substitution values, and are then set to sane defaults at runtime if the
// value is a zero-value string.
var (
	PrefixDir         string
	BinDir            string
	SbinDir           string
	LibexecDir        string
	DataDir           string
	DatarootDir       string
	ManDir            string
	DocDir            string
	SysconfDir        string
	LocalstateDir     string
	DbusInterfacesDir string
)

func init() {
	if PrefixDir == "" {
		PrefixDir = "/usr/local"
	}
	if BinDir == "" {
		BinDir = filepath.Join(PrefixDir, "bin")
	}
	if SbinDir == "" {
		SbinDir = filepath.Join(PrefixDir, "sbin")
	}
	if LibexecDir == "" {
		LibexecDir = filepath.Join(PrefixDir, "libexec")
	}
	if DataDir == "" {
		DataDir = filepath.Join(PrefixDir, "share")
	}
	if DatarootDir == "" {
		DatarootDir = filepath.Join(PrefixDir, "share")
	}
	if ManDir == "" {
		ManDir = filepath.Join(PrefixDir, "man")
	}
	if DocDir == "" {
		DocDir = filepath.Join(PrefixDir, "doc")
	}
	if SysconfDir == "" {
		SysconfDir = filepath.Join(PrefixDir, "etc")
	}
	if LocalstateDir == "" {
		LocalstateDir = filepath.Join(PrefixDir, "var")
	}
	if DbusInterfacesDir == "" {
		DbusInterfacesDir = filepath.Join(DataDir, "dbus-1", "interfaces")
	}

	if ShortName == "" {
		ShortName = "rhc"
	}
	if LongName == "" {
		LongName = "rhc"
	}
	if TopicPrefix == "" {
		TopicPrefix = "rhc"
	}
	if Provider == "" {
		Provider = "Red Hat"
	}
	if ServiceName == "" {
		ServiceName = "yggdrasil"
	}
}
