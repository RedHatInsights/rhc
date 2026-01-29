package main

import "path/filepath"

const (
	ExitCodeOK          = 0  // successful termination
	ExitCodeErr         = 1  // generic error
	ExitCodeUsage       = 64 // command line usage error
	ExitCodeDataErr     = 65 // data format error
	ExitCodeNoInput     = 66 // cannot open input
	ExitCodeNoUser      = 67 // addressee unknown
	ExitCodeNoHost      = 68 // host name unknown
	ExitCodeUnavailable = 69 // service unavailable
	ExitCodeSoftware    = 70 // internal software error
	ExitCodeOSErr       = 71 // system error (e.g., can't fork)
	ExitCodeOSFile      = 72 // critical OS file missing
	ExitCodeCantCreat   = 73 // can't create (user) output file
	ExitCodeIOErr       = 74 // input/output error
	ExitCodeTempFail    = 75 // temp failure; user is invited to retry
	ExitCodeProtocol    = 76 // remote error in protocol
	ExitCodeNoPerm      = 77 // permission denied
	ExitCodeConfig      = 78 // configuration error
)

var (
	// Version is the version as described by git.
	Version string

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
	LogDir            string
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
	if LogDir == "" {
		LogDir = filepath.Join(LocalstateDir, "log")
	}

	if ServiceName == "" {
		ServiceName = "yggdrasil"
	}
}
