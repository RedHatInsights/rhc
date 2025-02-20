package main

import "path/filepath"

const (
	// ExitCodeOK for successful termination
	ExitCodeOK = 0

	// ExitCodeErr for generic error
	ExitCodeErr = 1

	// ExitCodeUsage for command line usage error
	ExitCodeUsage = 64

	// ExitCodeDataErr for data format error
	ExitCodeDataErr = 65

	// ExitCodeNoInput for cannot open input
	ExitCodeNoInput = 66

	// ExitCodeNoUser for addressee unknown
	ExitCodeNoUser = 67

	// ExitCodeNoHost for host name unknown
	ExitCodeNoHost = 68

	// ExitCodeUnavailable for service unavailable
	ExitCodeUnavailable = 69

	// ExitCodeSoftware for internal software error
	ExitCodeSoftware = 70

	// ExitCodeOSErr system error (e.g., can't fork)
	ExitCodeOSErr = 71

	// ExitCodeOSFile critical OS file missing
	ExitCodeOSFile = 72

	// ExitCodeCantCreat for can't create (user) output file
	ExitCodeCantCreat = 73

	// ExitCodeIOErr for input/output error
	ExitCodeIOErr = 74

	// ExitCodeTempFail for temp failure; user is invited to retry
	ExitCodeTempFail = 75

	// ExitCodeProtocol for remote error in protocol
	ExitCodeProtocol = 76

	// ExitCodeNoPerm for permission denied
	ExitCodeNoPerm = 77

	// ExitCodeConfig for configuration error
	ExitCodeConfig = 78
)

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
