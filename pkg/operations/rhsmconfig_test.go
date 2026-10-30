package operations

import (
	"errors"
	"testing"

	"github.com/redhatinsights/rhc/internal/rhcconf"
	"github.com/redhatinsights/rhc/internal/subman"
)

type fakeRegistrar struct {
	sets        []rhcconf.RHSMSetting
	failOn      int
	password    int
	activation  int
	registerErr error
}

func (f *fakeRegistrar) SetConfig(key, value string) error {
	f.sets = append(f.sets, rhcconf.RHSMSetting{Key: key, Value: value})
	if f.failOn > 0 && len(f.sets) == f.failOn {
		return errors.New("set failed")
	}
	return nil
}

func (f *fakeRegistrar) RegisterWithPassword(string, string, string, subman.RegisterOptions) error {
	f.password++
	return f.registerErr
}

func (f *fakeRegistrar) RegisterWithActivationKeys(string, []string, subman.RegisterOptions) error {
	f.activation++
	return f.registerErr
}

func TestRegisterPreparedSetErrorSkipsRegister(t *testing.T) {
	client := &fakeRegistrar{failOn: 2}
	settings := []rhcconf.RHSMSetting{
		{Key: "server.hostname", Value: "satellite.example.com"},
		{Key: "server.port", Value: "443"},
	}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, settings)
	if err == nil {
		t.Fatal("registerSetup() error = nil, want Config.Set failure")
	}
	if client.password != 0 || client.activation != 0 {
		t.Errorf("register calls = password:%d activation:%d, want none", client.password, client.activation)
	}
	if len(client.sets) != 2 {
		t.Errorf("SetConfig calls = %d, want 2 (stop after the failing key)", len(client.sets))
	}
}

func TestRegisterPreparedEmptySettingsStillRegisters(t *testing.T) {
	client := &fakeRegistrar{}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, nil)
	if err != nil {
		t.Fatalf("registerSetup() error = %v", err)
	}
	if len(client.sets) != 0 {
		t.Errorf("SetConfig calls = %+v, want none", client.sets)
	}
	if client.password != 1 {
		t.Errorf("RegisterWithPassword calls = %d, want 1", client.password)
	}
	if client.activation != 0 {
		t.Errorf("RegisterWithActivationKeys calls = %d, want 0", client.activation)
	}
}

func TestRegisterPreparedActivationKeys(t *testing.T) {
	client := &fakeRegistrar{}
	settings := []rhcconf.RHSMSetting{{Key: "server.hostname", Value: "satellite.example.com"}}

	err := registerSetup(client, ConnectOptions{
		Organization:   "org",
		ActivationKeys: []string{"key"},
	}, settings)
	if err != nil {
		t.Fatalf("registerSetup() error = %v", err)
	}
	if len(client.sets) != 1 || client.sets[0].Key != "server.hostname" {
		t.Errorf("SetConfig calls = %+v, want server.hostname", client.sets)
	}
	if client.activation != 1 || client.password != 0 {
		t.Errorf("register calls = password:%d activation:%d, want activation only", client.password, client.activation)
	}
}

func TestRegisterPreparedOrganizationRequired(t *testing.T) {
	client := &fakeRegistrar{registerErr: subman.ErrOrganizationRequired}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, nil)
	if !errors.Is(err, ErrOrganizationRequired) {
		t.Fatalf("registerSetup() error = %v, want ErrOrganizationRequired", err)
	}
}
