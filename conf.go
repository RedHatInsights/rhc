package main

const (
	cliLogLevel  = "log-level"
	cliCertFile  = "cert-file"
	cliKeyFile   = "key-file"
	cliAPIServer = "base-url"
)

type Conf struct {
	CertFile string
	KeyFile  string
	LogLevel string
	CADir    string
}

var config = Conf{}
