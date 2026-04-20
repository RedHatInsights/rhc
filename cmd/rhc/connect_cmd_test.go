package main

import (
	"testing"
)

func TestCheckFeatureFlags(t *testing.T) {
	tests := []struct {
		name      string
		toEnable  []string
		toDisable []string
		wantErr   string
	}{
		{
			name:      "empty lists",
			toEnable:  []string{},
			toDisable: []string{},
			wantErr:   "",
		},
		{
			name:      "valid non-conflicting features",
			toEnable:  []string{"content"},
			toDisable: []string{"analytics"},
			wantErr:   "",
		},
		{
			name:      "same feature in both lists",
			toEnable:  []string{"content"},
			toDisable: []string{"content"},
			wantErr:   "invalid combination: enable 'content', disable 'content'",
		},
		{
			name:      "enable feature while disabling its dependency",
			toEnable:  []string{"remote-management"},
			toDisable: []string{"analytics"},
			wantErr:   "invalid combination: enable 'remote-management', disable 'analytics'",
		},
		{
			name:      "disable feature while enabling dependent",
			toEnable:  []string{"remote-management"},
			toDisable: []string{"analytics"},
			wantErr:   "invalid combination: enable 'remote-management', disable 'analytics'",
		},
		{
			name:      "invalid feature in toEnable",
			toEnable:  []string{"invalid-feature"},
			toDisable: []string{},
			wantErr:   "feature \"invalid-feature\" not found",
		},
		{
			name:      "invalid feature in toDisable",
			toEnable:  []string{},
			toDisable: []string{"invalid-feature"},
			wantErr:   "feature \"invalid-feature\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFeatureFlags(tt.toEnable, tt.toDisable)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("checkFeatureFlags() error = %v, wantErr nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("checkFeatureFlags() error = nil, wantErr %v", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("checkFeatureFlags() error = %v, wantErr %v", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
