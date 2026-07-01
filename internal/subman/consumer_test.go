package subman

import (
	"encoding/json"
	"testing"
)

func TestEnvironmentsFromConsumerJSON(t *testing.T) {
	t.Parallel()

	// Subset of Candlepin GET consumers/{uuid} response focusing on environments.
	const consumerJSON = `{
  "uuid": "5e9745d5-624d-4af1-916e-2c17df4eb4e8",
  "environments": [
    {
      "id": "env-id-1",
      "name": "env-name-1",
      "environmentContent": [{"contentId": "5001", "enabled": true}]
    }
  ]
}`

	var parsed struct {
		Environments []Environment `json:"environments"`
	}
	if err := json.Unmarshal([]byte(consumerJSON), &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(parsed.Environments) != 1 {
		t.Fatalf("len(environments) = %d, want 1", len(parsed.Environments))
	}
	if parsed.Environments[0].ID != "env-id-1" {
		t.Errorf("ID = %q, want env-id-1", parsed.Environments[0].ID)
	}
	if parsed.Environments[0].Name == nil || *parsed.Environments[0].Name != "env-name-1" {
		t.Errorf("Name = %v, want env-name-1", parsed.Environments[0].Name)
	}
	if len(parsed.Environments[0].EnvironmentContent) != 1 {
		t.Fatalf("len(EnvironmentContent) = %d, want 1", len(parsed.Environments[0].EnvironmentContent))
	}
	if parsed.Environments[0].EnvironmentContent[0].ContentID != "5001" {
		t.Errorf("ContentID = %q, want 5001", parsed.Environments[0].EnvironmentContent[0].ContentID)
	}
}
