// Package rhcconf provides production access to rhc's effective
// configuration: the merge of embedded defaults, a translation of
// /etc/rhsm/rhsm.conf, and /etc/rhc/rhc.conf.
package rhcconf

import (
	"path/filepath"

	conf "github.com/redhatinsights/rhc/internal/config"
	rhcfs "github.com/redhatinsights/rhc/internal/fs"
	"github.com/redhatinsights/rhc/internal/rhcconf"
	"github.com/redhatinsights/rhc/internal/rhsmconf"
)

const rhcConfDir = "/etc/rhc"

// Get reads rhc.conf and rhsm.conf from their standard locations and
// returns rhc's effective configuration.
func Get() (*rhcconf.Config, error) {
	rhcSource := conf.Source{
		Filesystem: rhcfs.Filesystem{},
		FilePath:   filepath.Join(rhcConfDir, "rhc.conf"),
	}
	rhsmSource := rhsmconf.Source{
		Filesystem: rhcfs.Filesystem{},
		FilePath:   rhsmconf.DefaultPath,
	}
	return rhcconf.Load(rhcSource, rhsmSource)
}
