package operations

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/redhatinsights/rhc/internal/rhcconf"
	"github.com/redhatinsights/rhc/internal/subman"
	pkgconfig "github.com/redhatinsights/rhc/pkg/config"
)

// rhsmConfigWriter writes rhsm.conf keys atomically in a single call.
type rhsmConfigWriter interface {
	SetAllConfig(settings map[string]string) error
}

// rhsmRegistrar writes rhsm.conf and then registers the system.
type rhsmRegistrar interface {
	rhsmConfigWriter
	RegisterWithPassword(username, password, organization string, opts subman.RegisterOptions) error
	RegisterWithActivationKeys(organization string, activationKeys []string, opts subman.RegisterOptions) error
}

// ApplyRHSMSettingsFromRHCConf writes the registration target and proxy keys that
// rhc.conf explicitly sets into rhsm.conf. A file with no such keys returns
// without calling com.redhat.RHSM1.Config.Set.
func ApplyRHSMSettingsFromRHCConf() error {
	settings, err := loadRHSMSettingsFromRHCConf()
	if err != nil {
		return err
	}
	if len(settings) == 0 {
		slog.Debug("rhc.conf sets no rhsm.conf keys; leaving rhsm.conf unchanged")
		return nil
	}
	client, err := subman.NewRHSMClient()
	if err != nil {
		return fmt.Errorf("cannot connect to subscription-manager: %w", err)
	}
	return applyRHSMSettings(client, settings)
}

func loadRHSMSettingsFromRHCConf() ([]rhcconf.RHSMSetting, error) {
	file, md, err := pkgconfig.Get()
	if err != nil {
		return nil, fmt.Errorf("reading rhc.conf: %w", err)
	}
	return rhcconf.RHSMSettingsFromRHCConf(file, md)
}

// applyRHSMSettings writes all settings to rhsm.conf in a single SetAll call so
// a failure cannot leave rhsm.conf with only some of the keys applied. An empty
// slice is a no-op.
func applyRHSMSettings(client rhsmConfigWriter, settings []rhcconf.RHSMSetting) error {
	if len(settings) == 0 {
		return nil
	}
	values := make(map[string]string, len(settings))
	keys := make([]string, 0, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
		keys = append(keys, setting.Key)
	}
	// Log key names only: values may include server.proxy_password and
	// credential-bearing URIs, which must never reach the logs.
	slog.Info("Applying rhc.conf settings to rhsm.conf", "count", len(keys), "keys", keys)
	if err := client.SetAllConfig(values); err != nil {
		return fmt.Errorf("writing rhsm.conf: %w", err)
	}
	slog.Debug("Wrote rhc.conf settings to rhsm.conf", "count", len(keys))
	return nil
}

func registerClient(client rhsmRegistrar, opts ConnectOptions) error {
	regOpts := subman.RegisterOptions{
		EnvironmentNames: opts.ContentTemplates,
		EnableContent:    opts.EnableContent,
	}
	var err error
	if len(opts.ActivationKeys) > 0 {
		err = client.RegisterWithActivationKeys(opts.Organization, opts.ActivationKeys, regOpts)
	} else {
		err = client.RegisterWithPassword(opts.Username, opts.Password, opts.Organization, regOpts)
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrOrganizationRequired) {
		return err
	}
	return fmt.Errorf("cannot connect to Red Hat Subscription Management: %s", err)
}

// registerSetup writes settings, then registers. A write error is returned
// before either Register method is called.
func registerSetup(client rhsmRegistrar, opts ConnectOptions, settings []rhcconf.RHSMSetting) error {
	if err := applyRHSMSettings(client, settings); err != nil {
		return err
	}
	return registerClient(client, opts)
}
