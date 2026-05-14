// Package exitcode defines exit codes for rhc CLI tools.
//
// These codes follow the sysexits.h convention from BSD systems,
// providing standardized exit codes for command-line programs.
// See: https://man.openbsd.org/sysexits.3
package exitcode

const (
	OK          = 0  // successful termination
	Err         = 1  // generic error
	Usage       = 64 // command line usage error
	DataErr     = 65 // data format error
	NoInput     = 66 // cannot open input
	NoUser      = 67 // addressee unknown
	NoHost      = 68 // host name unknown
	Unavailable = 69 // service unavailable
	Software    = 70 // internal software error
	OSErr       = 71 // system error (e.g., can't fork)
	OSFile      = 72 // critical OS file missing
	CantCreat   = 73 // can't create (user) output file
	IOErr       = 74 // input/output error
	TempFail    = 75 // temp failure; user is invited to retry
	Protocol    = 76 // remote error in protocol
	NoPerm      = 77 // permission denied
	Config      = 78 // configuration error
)
