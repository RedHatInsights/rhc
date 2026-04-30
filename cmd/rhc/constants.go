package main

const (
	// ConnectFeaturesPrefsPath is the path to the feature preferences cache file
	ConnectFeaturesPrefsPath = "/var/lib/rhc/rhc-connect-features-prefs.json"
)

const (
	connectCacheKey = "connect-cache"
)

var (
	// LogDir points to the log file directory
	LogDir string
)

func init() {
	if LogDir == "" {
		LogDir = "/var/log/rhc/"
	}
}
