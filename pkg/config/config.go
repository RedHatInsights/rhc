package config

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	conf "github.com/redhatinsights/rhc/internal/config"
	rhcfs "github.com/redhatinsights/rhc/internal/fs"
)

const (
	configDir = "/etc/rhc"
)

// Get reads the configuration file and returns a File plus TOML metadata
// for which keys were explicitly set.
func Get() (conf.File, toml.MetaData, error) {
	source := conf.Source{
		Filesystem: rhcfs.Filesystem{},
		FilePath:   filepath.Join(configDir, "rhc.conf"),
	}
	return conf.Get(source)
}
