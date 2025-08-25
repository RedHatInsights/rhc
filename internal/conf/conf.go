package conf

import "log/slog"

type Conf struct {
	CertFile string
	KeyFile  string
	LogLevel slog.Level
	CADir    string
}

var Config = Conf{}
