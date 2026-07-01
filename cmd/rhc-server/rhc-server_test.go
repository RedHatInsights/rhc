package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/rhc/internal/subman"
	"github.com/redhatinsights/rhc/varlink/consumerapi"
)

func TestToConsumerAPIOrganization(t *testing.T) {
	t.Parallel()

	displayName := "Donald Duck"
	contentAccessMode := "org_environment"
	org := &subman.Organization{
		ID:                  "4028face84391264018439127db10004",
		Key:                 "donaldduck",
		DisplayName:         &displayName,
		ContentAccessMode:   &contentAccessMode,
		AutobindDisabled:    boolPtr(false),
	}

	apiOrg, err := toConsumerAPIOrganization(org)
	if err != nil {
		t.Fatalf("toConsumerAPIOrganization() error = %v", err)
	}

	if apiOrg.Id != org.ID {
		t.Errorf("Id = %q, want %q", apiOrg.Id, org.ID)
	}
	if apiOrg.Key != org.Key {
		t.Errorf("Key = %q, want %q", apiOrg.Key, org.Key)
	}
	if apiOrg.DisplayName == nil || *apiOrg.DisplayName != displayName {
		t.Errorf("DisplayName = %v, want %q", apiOrg.DisplayName, displayName)
	}
	if apiOrg.ContentAccessMode == nil || *apiOrg.ContentAccessMode != contentAccessMode {
		t.Errorf("ContentAccessMode = %v, want %q", apiOrg.ContentAccessMode, contentAccessMode)
	}
}

func TestToConsumerAPIOrganizationNullOptionalFields(t *testing.T) {
	t.Parallel()

	org := &subman.Organization{
		ID:  "4028face84391264018439127db10004",
		Key: "donaldduck",
	}

	apiOrg, err := toConsumerAPIOrganization(org)
	if err != nil {
		t.Fatalf("toConsumerAPIOrganization() error = %v", err)
	}

	if apiOrg.ContentPrefix != nil {
		t.Errorf("ContentPrefix = %v, want nil (omitted optional field)", apiOrg.ContentPrefix)
	}
	if apiOrg.DisplayName != nil {
		t.Errorf("DisplayName = %v, want nil (omitted optional field)", apiOrg.DisplayName)
	}
}

func TestToConsumerAPIEnvironments(t *testing.T) {
	t.Parallel()

	environments := []subman.Environment{{
		ID:   "env-id-1",
		Name: strPtr("env-name-1"),
		Type: strPtr("content"),
		EnvironmentContent: []subman.EnvironmentContent{{
			ContentID: "5001",
			Enabled:   true,
		}},
	}}

	apiEnvironments, err := toConsumerAPIEnvironments(environments)
	if err != nil {
		t.Fatalf("toConsumerAPIEnvironments() error = %v", err)
	}

	if len(apiEnvironments) != 1 {
		t.Fatalf("len(apiEnvironments) = %d, want 1", len(apiEnvironments))
	}

	first := apiEnvironments[0]
	if first.Id != "env-id-1" {
		t.Errorf("Id = %q, want env-id-1", first.Id)
	}
	if first.EnvironmentContent == nil {
		t.Fatal("EnvironmentContent = nil, want non-nil pointer to slice")
	}
	if len(*first.EnvironmentContent) != 1 {
		t.Fatalf("len(EnvironmentContent) = %d, want 1", len(*first.EnvironmentContent))
	}
	if (*first.EnvironmentContent)[0].ContentId != "5001" || !(*first.EnvironmentContent)[0].Enabled {
		t.Errorf("EnvironmentContent[0] = %+v, want contentId=5001 enabled=true", (*first.EnvironmentContent)[0])
	}
}

func TestToConsumerAPIEnvironmentsEmpty(t *testing.T) {
	t.Parallel()

	apiEnvironments, err := toConsumerAPIEnvironments([]subman.Environment{})
	if err != nil {
		t.Fatalf("toConsumerAPIEnvironments() error = %v", err)
	}

	if apiEnvironments == nil {
		t.Fatal("apiEnvironments = nil, want non-nil empty slice")
	}
	if len(apiEnvironments) != 0 {
		t.Errorf("len(apiEnvironments) = %d, want 0", len(apiEnvironments))
	}
}

func TestToConsumerAPIEnvironmentsNilEnvironmentContent(t *testing.T) {
	t.Parallel()

	environments := []subman.Environment{{
		ID:   "env-id-1",
		Name: strPtr("env-name-1"),
	}}

	apiEnvironments, err := toConsumerAPIEnvironments(environments)
	if err != nil {
		t.Fatalf("toConsumerAPIEnvironments() error = %v", err)
	}

	want := []consumerapi.Environment{{
		Id:   "env-id-1",
		Name: strPtr("env-name-1"),
	}}

	if diff := cmp.Diff(want, apiEnvironments); diff != "" {
		t.Errorf("apiEnvironments mismatch (-want +got):\n%s", diff)
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
