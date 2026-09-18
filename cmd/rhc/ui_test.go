package main

import "testing"

func TestConfigureOutput(t *testing.T) {
	plainIcons := iconSet{
		Ok:      "✓",
		Info:    "●",
		Error:   "𐄂",
		Warning: "!",
	}
	coloredIcons := iconSet{
		Ok:      colorGreen + "✓" + colorReset,
		Info:    colorYellow + "●" + colorReset,
		Error:   colorRed + "𐄂" + colorReset,
		Warning: colorRed + "!" + colorReset,
	}
	tests := []struct {
		name                  string
		animated              bool
		colored               bool
		machineReadable       bool
		wantAnimationsEnabled bool
		wantMachineReadable   bool
		wantIcons             iconSet
	}{
		{
			name:                  "animated and colored",
			animated:              true,
			colored:               true,
			machineReadable:       false,
			wantAnimationsEnabled: true,
			wantMachineReadable:   false,
			wantIcons:             coloredIcons,
		},
		{
			name:                  "colored without animations",
			animated:              false,
			colored:               true,
			machineReadable:       false,
			wantAnimationsEnabled: false,
			wantMachineReadable:   false,
			wantIcons:             coloredIcons,
		},
		{
			name:                  "animated without color",
			animated:              true,
			colored:               false,
			machineReadable:       false,
			wantAnimationsEnabled: true,
			wantMachineReadable:   false,
			wantIcons:             plainIcons,
		},
		{
			name:                  "machine-readable output",
			animated:              false,
			colored:               true,
			machineReadable:       true,
			wantAnimationsEnabled: false,
			wantMachineReadable:   true,
			wantIcons:             coloredIcons,
		},
		{
			name:                  "plain without animations",
			animated:              false,
			colored:               false,
			machineReadable:       false,
			wantAnimationsEnabled: false,
			wantMachineReadable:   false,
			wantIcons:             plainIcons,
		},
	}

	t.Cleanup(func() {
		configureOutput(true, true, false)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configureOutput(tt.animated, tt.colored, tt.machineReadable)

			if got := areAnimationsEnabled(); got != tt.wantAnimationsEnabled {
				t.Errorf("areAnimationsEnabled() = %v, want %v", got, tt.wantAnimationsEnabled)
			}
			if got := isOutputMachineReadable(); got != tt.wantMachineReadable {
				t.Errorf("isOutputMachineReadable() = %v, want %v", got, tt.wantMachineReadable)
			}
			if icons != tt.wantIcons {
				t.Errorf("icons = %#v, want %#v", icons, tt.wantIcons)
			}
		})
	}
}
