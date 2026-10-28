package operations

import (
	"errors"
	"testing"

	"github.com/redhatinsights/rhc/internal/rhcconf"
	"github.com/redhatinsights/rhc/internal/subman"
)

type fakeRegistrar struct {
	sets        []rhcconf.RHSMSetting
	setAllCalls int
	fail        bool
	password    int
	activation  int
	registerErr error
}

func (f *fakeRegistrar) SetAllConfig(settings map[string]string) error {
	f.setAllCalls++
	if f.fail {
		return errors.New("set failed")
	}
	for key, value := range settings {
		f.sets = append(f.sets, rhcconf.RHSMSetting{Key: key, Value: value})
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

func TestRegisterSetupSetErrorSkipsRegister(t *testing.T) {
	client := &fakeRegistrar{fail: true}
	settings := []rhcconf.RHSMSetting{
		{Key: "server.hostname", Value: "satellite.example.com"},
		{Key: "server.port", Value: "443"},
	}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, settings)
	if err == nil {
		t.Fatal("registerSetup() error = nil, want Config.SetAll failure")
	}
	if client.password != 0 || client.activation != 0 {
		t.Errorf("register calls = password:%d activation:%d, want none", client.password, client.activation)
	}
	if client.setAllCalls != 1 {
		t.Errorf("SetAllConfig calls = %d, want 1 (single atomic write)", client.setAllCalls)
	}
	if len(client.sets) != 0 {
		t.Errorf("applied keys = %+v, want none (atomic write leaves nothing partial)", client.sets)
	}
}

func TestRegisterSetupEmptySettingsStillRegisters(t *testing.T) {
	client := &fakeRegistrar{}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, nil)
	if err != nil {
		t.Fatalf("registerSetup() error = %v", err)
	}
	if client.password != 1 {
		t.Errorf("RegisterWithPassword calls = %d, want 1", client.password)
	}
	if client.activation != 0 {
		t.Errorf("RegisterWithActivationKeys calls = %d, want 0", client.activation)
	}
}

func TestRegisterSetupActivationKeys(t *testing.T) {
	client := &fakeRegistrar{}
	settings := []rhcconf.RHSMSetting{{Key: "server.hostname", Value: "satellite.example.com"}}

	err := registerSetup(client, ConnectOptions{
		Organization:   "org",
		ActivationKeys: []string{"key"},
	}, settings)
	if err != nil {
		t.Fatalf("registerSetup() error = %v", err)
	}
	if client.setAllCalls != 1 {
		t.Errorf("SetAllConfig calls = %d, want 1", client.setAllCalls)
	}
	if len(client.sets) != 1 || client.sets[0].Key != "server.hostname" || client.sets[0].Value != "satellite.example.com" {
		t.Errorf("applied settings = %+v, want server.hostname=satellite.example.com", client.sets)
	}
	if client.activation != 1 || client.password != 0 {
		t.Errorf("register calls = password:%d activation:%d, want activation only", client.password, client.activation)
	}
}

func TestRegisterSetupOrganizationRequired(t *testing.T) {
	client := &fakeRegistrar{registerErr: subman.ErrOrganizationRequired}

	err := registerSetup(client, ConnectOptions{Username: "user", Password: "secret"}, nil)
	if !errors.Is(err, ErrOrganizationRequired) {
		t.Fatalf("registerSetup() error = %v, want ErrOrganizationRequired", err)
	}
}
