package operations

import (
	"errors"
	"fmt"

	"github.com/redhatinsights/rhc/internal/rhcconf"
	"github.com/redhatinsights/rhc/internal/subman"
	pkgconfig "github.com/redhatinsights/rhc/pkg/config"
)

// rhsmConfigWriter writes individual rhsm.conf keys.
type rhsmConfigWriter interface {
	SetConfig(key, value string) error
}

// rhsmRegistrar writes rhsm.conf and then registers the system.
type rhsmRegistrar interface {
	rhsmConfigWriter
	RegisterWithPassword(username, password, organization string, opts subman.RegisterOptions) error
	RegisterWithActivationKeys(organization string, activationKeys []string, opts subman.RegisterOptions) error
}

// ApplyDefinedRHSMConfig writes the registration target and proxy keys that
// rhc.conf explicitly sets into rhsm.conf. A file with no such keys returns
// without calling com.redhat.RHSM1.Config.Set.
func ApplyRHSMSettingsFromRHCConf() error {
	settings, err := loadRHSMSettingsFromRHCConf()
	if err != nil {
		return err
	}
	if len(settings) == 0 {
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

func applyRHSMSettings(client rhsmConfigWriter, settings []rhcconf.RHSMSetting) error {
	for _, setting := range settings {
		if err := client.SetConfig(setting.Key, setting.Value); err != nil {
			return fmt.Errorf("writing %s to rhsm.conf: %w", setting.Key, err)
		}
	}
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
