package main

import "log/slog"

const (
	cliLogLevel  = "log-level"
	cliCertFile  = "cert-file"
	cliKeyFile   = "key-file"
	cliAPIServer = "base-url"
)

type Conf struct {
	CertFile string
	KeyFile  string
	LogLevel slog.Level
	CADir    string
}

var config = Conf{}
