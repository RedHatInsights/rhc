package config

import (
	"path/filepath"

	conf "github.com/redhatinsights/rhc/internal/config"
	rhcfs "github.com/redhatinsights/rhc/internal/fs"
)

const (
	configDir = "/etc/rhc"
)

// Get reads the configuration file and returns a File structure.
func Get() (conf.File, error) {
	source := conf.Source{
		Filesystem: rhcfs.Filesystem{},
		FilePath:   filepath.Join(configDir, "rhc.conf"),
	}
	return conf.Get(source)
}
