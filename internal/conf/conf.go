package conf

import "log/slog"

type Conf struct {
	CertFile string     `toml:"cert-file"`
	KeyFile  string     `toml:"key-file"`
	LogLevel slog.Level `toml:"log-level"`
	CADir    string     `toml:"ca-dir"`
	Features Features   `toml:"features"`
}

type Features struct {
	Content    *bool `toml:"content"`
	Analytics  *bool `toml:"analytics"`
	Management *bool `toml:"remote-management"`
}

var Config = Conf{}
