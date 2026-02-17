package conf

import "log/slog"

// Conf is a structure holding configuration values from /etc/rhc/config.toml file
type Conf struct {
	CertFile string     `toml:"cert-file"`
	KeyFile  string     `toml:"key-file"`
	LogLevel slog.Level `toml:"log-level"`
	CADir    string     `toml:"ca-dir"`
}

var Config = Conf{}

// ConnectFeaturesPrefs represents only preferences during "rhc connect". The file
// should exist in /var/lib/rhc/rhc-connect-features-prefs.json upon calling "rhc connect".
// Then the file should be deleted after the connection is established, because the
// preference file contains the "connect" preference. If such preference was part of
// some configuration file in /etc, then it could be easily in conflict in the configuration
// file /etc/rhsm/rhsm.conf, because the manage_repos option in the section [rhsm] in
// /etc/rhsm/rhsm.conf is the source of truth of content management. When the sub-man
// will stop to exist and all the business logic will be moved to "rhc.next", then the preference
// can become part of some configuration file in /etc/rhc/*
type ConnectFeaturesPrefs struct {
	Content          *bool `json:"content"`
	Analytics        *bool `json:"analytics"`
	RemoteManagement *bool `json:"remote-management"`
}

var ConnectFeaturesPreferences ConnectFeaturesPrefs
