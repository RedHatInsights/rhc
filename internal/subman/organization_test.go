package subman

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const sampleOrganizationJSON = `{
   "created": "2022-11-02T16:00:23+0000",
   "updated": "2022-11-02T16:00:48+0000",
   "id": "4028face84391264018439127db10004",
   "displayName": "Donald Duck",
   "key": "donaldduck",
   "contentPrefix": null,
   "defaultServiceLevel": null,
   "logLevel": null,
   "contentAccessMode": "org_environment",
   "contentAccessModeList": "entitlement,org_environment",
   "autobindHypervisorDisabled": false,
   "autobindDisabled": false,
   "lastRefreshed": "2022-11-02T16:00:48+0000",
   "parentOwner": null,
   "upstreamConsumer": null
}`

func TestOrganizationUnmarshal(t *testing.T) {
	t.Parallel()

	var org Organization
	if err := json.Unmarshal([]byte(sampleOrganizationJSON), &org); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if org.Key != "donaldduck" {
		t.Errorf("Key = %q, want donaldduck", org.Key)
	}
	if org.ID != "4028face84391264018439127db10004" {
		t.Errorf("ID = %q, want 4028face84391264018439127db10004", org.ID)
	}
	if org.DisplayName == nil || *org.DisplayName != "Donald Duck" {
		t.Errorf("DisplayName = %v, want Donald Duck", org.DisplayName)
	}
	if org.ContentAccessMode == nil || *org.ContentAccessMode != "org_environment" {
		t.Errorf("ContentAccessMode = %v, want org_environment", org.ContentAccessMode)
	}
}

const sampleEnvironmentsJSON = `[
  {
    "id": "env-id-1",
    "name": "env-name-1",
    "description": "Testing environment #1",
    "contentPrefix": "/content/dist/rhel9",
    "owner": {
      "id": "4028face84391264018439127db10004",
      "key": "donaldduck",
      "displayName": "Donald Duck",
      "contentAccessMode": "org_environment"
    },
    "environmentContent": [
      {"contentId": "5001", "enabled": true}
    ]
  },
  {
    "id": "env-id-2",
    "name": "env-name-2",
    "environmentContent": [
      {"contentId": "5002", "enabled": false}
    ]
  }
]`

func TestEnvironmentUnmarshal(t *testing.T) {
	t.Parallel()

	var environments []Environment
	if err := json.Unmarshal([]byte(sampleEnvironmentsJSON), &environments); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(environments) != 2 {
		t.Fatalf("len(environments) = %d, want 2", len(environments))
	}

	first := environments[0]
	if first.ID != "env-id-1" {
		t.Errorf("environments[0].ID = %q, want env-id-1", first.ID)
	}
	if first.Name == nil || *first.Name != "env-name-1" {
		t.Errorf("environments[0].Name = %v, want env-name-1", first.Name)
	}
	if first.ContentPrefix == nil || *first.ContentPrefix != "/content/dist/rhel9" {
		t.Errorf("environments[0].ContentPrefix = %v, want /content/dist/rhel9", first.ContentPrefix)
	}
	if first.Owner == nil || first.Owner.Key == nil || *first.Owner.Key != "donaldduck" {
		t.Errorf("environments[0].Owner.Key = %v, want donaldduck", first.Owner)
	}
	if len(first.EnvironmentContent) != 1 {
		t.Fatalf("len(environments[0].EnvironmentContent) = %d, want 1", len(first.EnvironmentContent))
	}
	if first.EnvironmentContent[0].ContentID != "5001" || !first.EnvironmentContent[0].Enabled {
		t.Errorf("environments[0].EnvironmentContent[0] = %+v, want contentId=5001 enabled=true",
			first.EnvironmentContent[0])
	}

	second := environments[1]
	if second.EnvironmentContent[0].ContentID != "5002" || second.EnvironmentContent[0].Enabled {
		t.Errorf("environments[1].EnvironmentContent[0] = %+v, want contentId=5002 enabled=false",
			second.EnvironmentContent[0])
	}
}

func TestEnvironmentRoundTripFromRhsm2Shape(t *testing.T) {
	t.Parallel()

	// Simulates json.Marshal output from rhsm2.Environment slice.
	const rhsm2EnvironmentsJSON = `[{"id":"env-id-1","name":"env-name-1","type":"content","environmentContent":[{"contentId":"5001","enabled":true}]}]`

	var environments []Environment
	if err := json.Unmarshal([]byte(rhsm2EnvironmentsJSON), &environments); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := []Environment{{
		ID:   "env-id-1",
		Name: strPtr("env-name-1"),
		Type: strPtr("content"),
		EnvironmentContent: []EnvironmentContent{{
			ContentID: "5001",
			Enabled:   true,
		}},
	}}

	if diff := cmp.Diff(want, environments); diff != "" {
		t.Errorf("environments mismatch (-want +got):\n%s", diff)
	}
}

func strPtr(s string) *string {
	return &s
}
