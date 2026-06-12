package support

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Credentials holds the authentication material used by setup steps to connect
// or register the system under test.  Load them with LoadCredentials.
//
// The credentials file lives at $CONF/credentials.toml.  Supply exactly one of
// the two supported authentication methods:
//
//	# Method 1 — username + password
//	username = "user@example.com"
//	password = "secret"
//
//	# Method 2 — organisation + activation key
//	org            = "123456"
//	activation_key = "my-key"
type Credentials struct {
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Org           string `toml:"org"`
	ActivationKey string `toml:"activation_key"`
}

// LoadCredentials reads $CONF/credentials.toml and returns the parsed
// credentials.  It returns an error when the file is missing, malformed, or
// contains neither of the two valid authentication combinations.
func LoadCredentials() (*Credentials, error) {
	path := filepath.Join(TestConfig, "credentials.toml")
	var creds Credentials
	if _, err := toml.DecodeFile(path, &creds); err != nil {
		return nil, fmt.Errorf("loading credentials from %s: %w", path, err)
	}

	hasUserPass := creds.Username != "" && creds.Password != ""
	hasOrgKey := creds.Org != "" && creds.ActivationKey != ""

	if !hasUserPass && !hasOrgKey {
		return nil, fmt.Errorf(
			"%s: must contain either (username + password) or (org + activation_key)",
			path,
		)
	}
	return &creds, nil
}

// RHCConnectArgs returns the rhc-connect(1) flags for these credentials.
// Example output: ["--username", "alice", "--password", "s3cr3t"]
func (c *Credentials) RHCConnectArgs() []string {
	if c.Org != "" {
		return []string{"--organization", c.Org, "--activation-key", c.ActivationKey}
	}
	return []string{"--username", c.Username, "--password", c.Password}
}

// RHSMRegisterArgs returns the subscription-manager-register(8) flags for
// these credentials.
// Example output: ["--username", "alice", "--password", "s3cr3t"]
func (c *Credentials) RHSMRegisterArgs() []string {
	if c.Org != "" {
		return []string{"--org", c.Org, "--activationkey", c.ActivationKey}
	}
	return []string{"--username", c.Username, "--password", c.Password}
}

// RHCConnectCommand builds the full rhc connect command string.
func (c *Credentials) RHCConnectCommand() string {
	return "rhc connect " + strings.Join(c.RHCConnectArgs(), " ")
}

// RHSMRegisterCommand builds the full subscription-manager register command.
func (c *Credentials) RHSMRegisterCommand() string {
	return "subscription-manager register " + strings.Join(c.RHSMRegisterArgs(), " ")
}
