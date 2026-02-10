package main

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

	// LogDir points to the log file directory
	LogDir string
)

func init() {
	if Version == "" {
		Version = "dev"
	}
	if LogDir == "" {
		LogDir = "/var/log/rhc/"
	}
}
