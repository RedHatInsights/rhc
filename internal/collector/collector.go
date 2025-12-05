package collector

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ConfigDir is the default directory path where collector configuration files are stored.
const ConfigDir = "/usr/lib/rhc/collector/"
const defaultMetaType = "ingress"
const defaultUser = "root"
const defaultGroup = "root"

// Config represents the configuration for a collector instance.
type Config struct {
	// ID is the unique identifier for the collector.
	ID string
	// Name is the human-readable name of the collector.
	Name string
	// IsAnalyticsFeature indicates whether collector provides analytics functionality.
	IsAnalyticsFeature bool
	// User specifies the system user under which the collector should run.
	User string
	// Group specifies the system group under which the collector should run.
	Group string
	// ContentType is used by rhc when it uploads the data archive to Ingress.
	ContentType string
}

// configDto represents the structure of a TOML configuration file for parsing.
type configDto struct {
	Meta    *metaDto    `toml:"meta"`
	Ingress *ingressDto `toml:"ingress"`
}

// metaDto represents the metadata section of a TOML configuration file.
type metaDto struct {
	Name    string  `toml:"name"`
	Feature *string `toml:"feature,omitempty"`
	Type    *string `toml:"type"`
}

// ingressDto represents the ingress section of a TOML configuration file.
type ingressDto struct {
	User        *string `toml:"user,omitempty"`
	Group       *string `toml:"group,omitempty"`
	ContentType string  `toml:"content_type"`
}

// GetCollectors returns list of available collectors from valid TOML files in ConfigDir.
func GetCollectors() ([]string, error) {
	configFiles, err := os.ReadDir(ConfigDir)
	if err != nil {
		return nil, err
	}

	var collectors []string
	for _, configFile := range configFiles {
		configName, err := getConfigFilename(configFile)
		if err != nil {
			log.Printf("Warning: %v", err)
		} else {
			collectorId := strings.TrimSuffix(configName, ".toml")
			if _, err = loadConfigFromFile(collectorId); err != nil {
				log.Printf("Warning: failed to load config from %s: %v", configName, err)
			} else {
				collectors = append(collectors, collectorId)
			}
		}
	}

	return collectors, nil
}

// GetConfig retrieves a collector configuration by its ID.
func GetConfig(id string) (Config, error) {
	config, err := loadConfigFromFile(id)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

// getConfigFilename returns the filename if the file entry is a valid TOML configuration file.
// Returns an error if the entry is not a regular file with a .toml extension.
func getConfigFilename(configFile os.DirEntry) (string, error) {
	if isFileToml(configFile) {
		return configFile.Name(), nil
	}
	return "", fmt.Errorf("invalid config file %v", filepath.Join(ConfigDir, configFile.Name()))
}

// isFileToml returns true if the file entry is a regular file with a .toml extension.
func isFileToml(file os.DirEntry) bool {
	return !file.IsDir() && strings.HasSuffix(file.Name(), ".toml")
}

// parseConfigFromContent parses TOML content directly from a string into a Config.
func parseConfigFromContent(content string, id string) (Config, error) {
	var c *configDto
	_, err := toml.Decode(content, &c)
	if err != nil {
		return Config{}, err
	}
	return newConfig(id, c)
}

// loadConfigFromFile loads a collector configuration file from the ConfigDir directory.
// Returns an error if the file cannot be decoded.
func loadConfigFromFile(id string) (Config, error) {
	var c *configDto
	_, err := toml.DecodeFile(ConfigDir+id+".toml", &c)
	if err != nil {
		return Config{}, err
	}
	config, err := newConfig(id, c)
	return config, err
}

// newConfig creates a Config instance from a configDto and validates required fields.
// Returns an error if any required field is missing or is invalid.
func newConfig(id string, dto *configDto) (Config, error) {
	if dto.Meta == nil {
		return Config{}, fmt.Errorf("invalid config: meta section is required")
	}
	if dto.Meta.Name == "" {
		return Config{}, fmt.Errorf("invalid config: meta.name is required")
	}
	if dto.Meta.Type == nil || *dto.Meta.Type != defaultMetaType {
		return Config{}, fmt.Errorf("invalid config: meta.type must be '%s'", defaultMetaType)
	}

	if dto.Ingress == nil {
		return Config{}, fmt.Errorf("invalid config: ingress section is required")
	}
	var user string
	if dto.Ingress.User != nil {
		user = *dto.Ingress.User
	} else {
		user = defaultUser
	}
	var group string
	if dto.Ingress.Group != nil {
		group = *dto.Ingress.Group
	} else {
		group = defaultGroup
	}
	if dto.Ingress.ContentType == "" {
		return Config{}, fmt.Errorf("invalid config: ingress.content_type is required")
	}

	// Emit warning if meta.feature is present but not 'analytics'
	if dto.Meta.Feature != nil && *dto.Meta.Feature != "analytics" {
		log.Printf("Warning: meta.feature is '%s' but expected 'analytics'", *dto.Meta.Feature)
	}

	return Config{
		ID:                 id,
		Name:               dto.Meta.Name,
		IsAnalyticsFeature: dto.Meta.Feature == nil || *dto.Meta.Feature == "analytics",
		User:               user,
		Group:              group,
		ContentType:        dto.Ingress.ContentType,
	}, nil
}
