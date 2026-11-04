package rhcconf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolveInsightsWithoutRHSM(t *testing.T) {
	file, md := decodeRHCConf(t, "")
	got, err := Resolve(file, md, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	wantIngress := Endpoint{URI: "https://cert.console.redhat.com/api/ingress/v1/upload", TLSVerify: true, Source: SourceDefault}
	if diff := cmp.Diff(wantIngress, got.InsightsIngress); diff != "" {
		t.Errorf("InsightsIngress mismatch (-want +got):\n%s", diff)
	}
	wantInventory := Endpoint{URI: "https://cert.console.redhat.com/api/inventory/v1/hosts", TLSVerify: true, Source: SourceDefault}
	if diff := cmp.Diff(wantInventory, got.InsightsInventory); diff != "" {
		t.Errorf("InsightsInventory mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveInsightsExplicitURI(t *testing.T) {
	for _, section := range []string{"ingress", "inventory"} {
		t.Run(section, func(t *testing.T) {
			const uri = "https://collector.example.com:8443/custom?key=value"
			file, md := decodeRHCConf(t, fmt.Sprintf("[api.insights.%s]\nuri = %q\n", section, uri))
			got, err := Resolve(file, md, satelliteRHSM)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			explicit, derived := got.InsightsIngress, got.InsightsInventory
			if section == "inventory" {
				explicit, derived = derived, explicit
			}
			want := Endpoint{URI: uri, TLSVerify: true, Source: SourceRHCConf}
			if diff := cmp.Diff(want, explicit); diff != "" {
				t.Errorf("explicit endpoint mismatch (-want +got):\n%s", diff)
			}
			wantDerived := Endpoint{
				URI:       "https://satellite.example.com/redhat_access/r/insights/platform/inventory/v1",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHSMConf,
			}
			if section == "inventory" {
				wantDerived.URI = "https://satellite.example.com/redhat_access/r/insights/platform/ingress/v1/upload"
			}
			if diff := cmp.Diff(wantDerived, derived); diff != "" {
				t.Errorf("derived endpoint mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveInsightsExplicitTLSAndCA(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		tlsVerify bool
		caPath    string
	}{
		{
			name:      "TLS only",
			input:     "tls-verify = false\n",
			tlsVerify: false,
			caPath:    "/etc/rhsm/ca/katello-server-ca.pem",
		},
		{
			name:      "CA only",
			input:     "ca-path = \"/etc/pki/custom.pem\"\n",
			tlsVerify: true,
			caPath:    "/etc/pki/custom.pem",
		},
		{
			name:      "empty CA",
			input:     "ca-path = \"\"\n",
			tlsVerify: true,
			caPath:    "",
		},
	}
	for _, section := range []string{"ingress", "inventory"} {
		for _, tt := range tests {
			t.Run(section+"/"+tt.name, func(t *testing.T) {
				file, md := decodeRHCConf(t, fmt.Sprintf("[api.insights.%s]\n%s", section, tt.input))
				got, err := Resolve(file, md, satelliteRHSM)
				if err != nil {
					t.Fatalf("Resolve() error = %v", err)
				}
				wantIngress := Endpoint{
					URI:       "https://satellite.example.com/redhat_access/r/insights/platform/ingress/v1/upload",
					TLSVerify: true,
					CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
					Source:    SourceRHSMConf,
				}
				wantInventory := wantIngress
				wantInventory.URI = "https://satellite.example.com/redhat_access/r/insights/platform/inventory/v1"
				if section == "ingress" {
					wantIngress.TLSVerify, wantIngress.CAPath = tt.tlsVerify, tt.caPath
				} else {
					wantInventory.TLSVerify, wantInventory.CAPath = tt.tlsVerify, tt.caPath
				}
				if diff := cmp.Diff(wantIngress, got.InsightsIngress); diff != "" {
					t.Errorf("InsightsIngress mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(wantInventory, got.InsightsInventory); diff != "" {
					t.Errorf("InsightsInventory mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

func TestResolveInsightsTLSAndCAIndependentOfSubscriptions(t *testing.T) {
	tests := []struct {
		name               string
		insecure           bool
		input              string
		wantSubscriptionCA string
	}{
		{
			name:               "RHSM insecure",
			insecure:           true,
			wantSubscriptionCA: "/etc/rhsm/ca/katello-server-ca.pem",
		},
		{
			name: "subscription TLS and CA overrides",
			input: `[api.subscriptions]
tls-verify = false
ca-path = "/etc/pki/subscription-ca.pem"
`,
			wantSubscriptionCA: "/etc/pki/subscription-ca.pem",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rhsm := *satelliteRHSM
			rhsm.Insecure = tt.insecure
			file, md := decodeRHCConf(t, tt.input)
			got, err := Resolve(file, md, &rhsm)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Subscriptions.TLSVerify || got.Subscriptions.CAPath != tt.wantSubscriptionCA {
				t.Fatalf("Subscriptions = %+v, want TLSVerify=false and CAPath=%q", got.Subscriptions, tt.wantSubscriptionCA)
			}
			wantIngress := Endpoint{
				URI:       "https://satellite.example.com/redhat_access/r/insights/platform/ingress/v1/upload",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHSMConf,
			}
			wantInventory := wantIngress
			wantInventory.URI = "https://satellite.example.com/redhat_access/r/insights/platform/inventory/v1"
			if diff := cmp.Diff(wantIngress, got.InsightsIngress); diff != "" {
				t.Errorf("InsightsIngress mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(wantInventory, got.InsightsInventory); diff != "" {
				t.Errorf("InsightsInventory mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveInsightsUsesEffectiveRegistrationHostname(t *testing.T) {
	tests := []struct {
		host          string
		wantIngress   Endpoint
		wantInventory Endpoint
	}{
		{
			host:          "subscription.rhsm.stage.redhat.com",
			wantIngress:   Endpoint{URI: "https://cert.console.stage.redhat.com/api/ingress/v1/upload", TLSVerify: true, Source: SourceRHCConf},
			wantInventory: Endpoint{URI: "https://cert.console.redhat.com/api/inventory/v1/hosts", TLSVerify: true, Source: SourceDefault},
		},
		{
			host: "other-satellite.example.com",
			wantIngress: Endpoint{
				URI:       "https://other-satellite.example.com:8443/redhat_access/r/insights/platform/ingress/v1/upload",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHCConf,
			},
			wantInventory: Endpoint{
				URI:       "https://other-satellite.example.com:8443/redhat_access/r/insights/platform/inventory/v1",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHCConf,
			},
		},
		{
			host: "[2001:db8::1]",
			wantIngress: Endpoint{
				URI:       "https://[2001:db8::1]:8443/redhat_access/r/insights/platform/ingress/v1/upload",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHCConf,
			},
			wantInventory: Endpoint{
				URI:       "https://[2001:db8::1]:8443/redhat_access/r/insights/platform/inventory/v1",
				TLSVerify: true,
				CAPath:    "/etc/rhsm/ca/katello-server-ca.pem",
				Source:    SourceRHCConf,
			},
		},
		{
			host:          "subscription.rhsm.redhat.com",
			wantIngress:   Endpoint{URI: "https://cert.console.redhat.com/api/ingress/v1/upload", TLSVerify: true, Source: SourceDefault},
			wantInventory: Endpoint{URI: "https://cert.console.redhat.com/api/inventory/v1/hosts", TLSVerify: true, Source: SourceDefault},
		},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			file, md := decodeRHCConf(t, fmt.Sprintf("[api.subscriptions]\nuri = %q\n", "https://"+tt.host+":8443/custom-prefix"))
			got, err := Resolve(file, md, satelliteRHSM)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if diff := cmp.Diff(tt.wantIngress, got.InsightsIngress); diff != "" {
				t.Errorf("InsightsIngress mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantInventory, got.InsightsInventory); diff != "" {
				t.Errorf("InsightsInventory mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveInsightsRejectsUnusableRegistrationURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr string
	}{
		{
			name:    "missing host",
			uri:     "https://:8443/subscription",
			wantErr: `registration URI "https://:8443/subscription" is missing a host`,
		},
		{
			name:    "port out of range",
			uri:     "https://satellite.example.com:99999/rhsm",
			wantErr: `registration URI "https://satellite.example.com:99999/rhsm": "99999" is not a valid port`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, md := decodeRHCConf(t, fmt.Sprintf("[api.subscriptions]\nuri = %q\n", tt.uri))
			_, err := Resolve(file, md, satelliteRHSM)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Resolve() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveInsightsRejectsInvalidExplicitURI(t *testing.T) {
	for _, section := range []string{"ingress", "inventory"} {
		for _, uri := range []string{"", "collector.example.com/upload"} {
			t.Run(section+"/"+uri, func(t *testing.T) {
				file, md := decodeRHCConf(t, fmt.Sprintf("[api.insights.%s]\nuri = %q\n", section, uri))
				_, err := Resolve(file, md, satelliteRHSM)
				key := "api.insights." + section + ".uri"
				if err == nil || !strings.Contains(err.Error(), key) {
					t.Errorf("Resolve() error = %v, want validation error naming %s", err, key)
				}
			})
		}
	}
}
