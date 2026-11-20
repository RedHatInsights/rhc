package canonical_facts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetCanonicalFactsResilient(t *testing.T) {
	facts, err := GetCanonicalFacts()
	if err != nil {
		t.Fatalf("GetCanonicalFacts should not fail when some facts are unavailable: %v", err)
	}
	if facts == nil {
		t.Fatal("expected non-nil CanonicalFacts")
	}
	if facts.IPAddresses == nil {
		t.Error("IPAddresses should be a non-nil slice")
	}
	if facts.MACAddresses == nil {
		t.Error("MACAddresses should be a non-nil slice")
	}
}

func TestGetCanonicalFactsPartialSources(t *testing.T) {
	dir := t.TempDir()

	machineIDPath := filepath.Join(dir, "machine-id")
	if err := os.WriteFile(machineIDPath, []byte("a8e0af25e2e7468797c52da4effc4db9\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sources := factSources{
		insightsIDPath: filepath.Join(dir, "missing-insights-id"),
		machineIDPath:  machineIDPath,
		// Missing or unreadable BIOS UUID / consumer cert must not abort collection.
		biosUUIDPath: filepath.Join(dir, "missing-product_uuid"),
		consumerCert: filepath.Join(dir, "missing-cert.pem"),
	}

	facts, err := getCanonicalFacts(sources)
	if err != nil {
		t.Fatalf("getCanonicalFacts should succeed with partial sources: %v", err)
	}

	if facts.MachineID != "a8e0af25-e2e7-4687-97c5-2da4effc4db9" {
		t.Errorf("MachineID = %q, want dashed UUID from machine-id", facts.MachineID)
	}
	if facts.InsightsID != "" {
		t.Errorf("InsightsID = %q, want empty when file is missing", facts.InsightsID)
	}
	if facts.BIOSUUID != "" {
		t.Errorf("BIOSUUID = %q, want empty when file is unavailable", facts.BIOSUUID)
	}
	if facts.SubscriptionManagerID != "" {
		t.Errorf("SubscriptionManagerID = %q, want empty when cert is missing", facts.SubscriptionManagerID)
	}
	if facts.IPAddresses == nil || facts.MACAddresses == nil {
		t.Error("address slices should be non-nil even when other facts are missing")
	}
}

func TestGetCanonicalFactsInvalidMachineID(t *testing.T) {
	dir := t.TempDir()
	machineIDPath := filepath.Join(dir, "machine-id")
	if err := os.WriteFile(machineIDPath, []byte("not-a-uuid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sources := factSources{
		insightsIDPath: filepath.Join(dir, "missing-insights-id"),
		machineIDPath:  machineIDPath,
		biosUUIDPath:   filepath.Join(dir, "missing-bios-uuid"),
		consumerCert:   filepath.Join(dir, "missing-cert.pem"),
	}

	facts, err := getCanonicalFacts(sources)
	if err != nil {
		t.Fatalf("invalid machine-id must not fail collection: %v", err)
	}
	if facts.MachineID != "" {
		t.Errorf("MachineID = %q, want empty for invalid UUID", facts.MachineID)
	}
}

func TestCanonicalFactsFromMap(t *testing.T) {
	tests := []struct {
		description string
		input       map[string]interface{}
		want        *CanonicalFacts
		wantError   error
	}{
		{
			description: "valid",
			input: map[string]interface{}{
				"insights_id":             "bb69cd34-263f-444c-9278-5935b61d7f60",
				"machine_id":              "acc046d0-0add-4550-ac7c-5a833b1b6470",
				"bios_uuid":               "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				"subscription_manager_id": "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				"ip_addresses":            []string{"1.2.3.4", "5.6.7.8"},
				"fqdn":                    "foo.bar.com",
				"mac_addresses":           []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
			want: &CanonicalFacts{
				InsightsID:            "bb69cd34-263f-444c-9278-5935b61d7f60",
				MachineID:             "acc046d0-0add-4550-ac7c-5a833b1b6470",
				BIOSUUID:              "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				SubscriptionManagerID: "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				IPAddresses:           []string{"1.2.3.4", "5.6.7.8"},
				FQDN:                  "foo.bar.com",
				MACAddresses:          []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
		},
		{
			description: "error",
			input: map[string]interface{}{
				"insights_id":             1,
				"machine_id":              "acc046d0-0add-4550-ac7c-5a833b1b6470",
				"bios_uuid":               "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				"subscription_manager_id": "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				"ip_addresses":            []string{"1.2.3.4", "5.6.7.8"},
				"fqdn":                    "foo.bar.com",
				"mac_addresses":           []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
			wantError: &InvalidValueTypeError{key: "insights_id", val: 1},
		},
		{
			description: "valid with absent insights_id",
			input: map[string]interface{}{
				"machine_id":              "acc046d0-0add-4550-ac7c-5a833b1b6470",
				"bios_uuid":               "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				"subscription_manager_id": "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				"ip_addresses":            []string{"1.2.3.4", "5.6.7.8"},
				"fqdn":                    "foo.bar.com",
				"mac_addresses":           []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
			want: &CanonicalFacts{
				InsightsID:            "",
				MachineID:             "acc046d0-0add-4550-ac7c-5a833b1b6470",
				BIOSUUID:              "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				SubscriptionManagerID: "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				IPAddresses:           []string{"1.2.3.4", "5.6.7.8"},
				FQDN:                  "foo.bar.com",
				MACAddresses:          []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := CanonicalFactsFromMap(test.input)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmp.AllowUnexported(InvalidValueTypeError{})) {
					t.Errorf("%v != %v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("diff got test.want\n--- got\n+++ test.want\n%v", cmp.Diff(got, test.want))
				}
			}
		})
	}
}

func TestCanonicalFactsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		description string
		input       []byte
		want        CanonicalFacts
		wantError   error
	}{
		{
			description: "empty",
			input:       []byte(`{}`),
			want:        CanonicalFacts{},
		},
		{
			description: "valid",
			input:       []byte(`{"insights_id":"bb69cd34-263f-444c-9278-5935b61d7f60","machine_id":"acc046d0-0add-4550-ac7c-5a833b1b6470","bios_uuid":"d8ec3cd5-a6bc-4742-bd2f-32940da182b0","subscription_manager_id":"bc452b83-c4ee-4b80-91d8-98ff816b2440","ip_addresses":["1.2.3.4", "5.6.7.8"],"fqdn":"foo.bar.com","mac_addresses":["CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"]}`),
			want: CanonicalFacts{
				InsightsID:            "bb69cd34-263f-444c-9278-5935b61d7f60",
				MachineID:             "acc046d0-0add-4550-ac7c-5a833b1b6470",
				BIOSUUID:              "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
				SubscriptionManagerID: "bc452b83-c4ee-4b80-91d8-98ff816b2440",
				IPAddresses:           []string{"1.2.3.4", "5.6.7.8"},
				FQDN:                  "foo.bar.com",
				MACAddresses:          []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var got CanonicalFacts
			err := json.Unmarshal(test.input, &got)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError) {
					t.Errorf("%v != %v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("%v != %v", got, test.want)
				}
			}
		})
	}
}

func BenchmarkCanonicalFactsFromMap(b *testing.B) {
	input := map[string]interface{}{
		"insights_id":             "bb69cd34-263f-444c-9278-5935b61d7f60",
		"machine_id":              "acc046d0-0add-4550-ac7c-5a833b1b6470",
		"bios_uuid":               "d8ec3cd5-a6bc-4742-bd2f-32940da182b0",
		"subscription_manager_id": "bc452b83-c4ee-4b80-91d8-98ff816b2440",
		"ip_addresses":            []string{"1.2.3.4", "5.6.7.8"},
		"fqdn":                    "foo.bar.com",
		"mac_addresses":           []string{"CC:D1:7A:44:6D:1B", "A7:03:90:D0:05:A7"},
	}
	for i := 0; i < b.N; i++ {
		_, err := CanonicalFactsFromMap(input)
		if err != nil {
			b.Error(err)
		}
	}
}
