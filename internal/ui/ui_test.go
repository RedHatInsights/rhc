package ui

import "testing"

func TestConfigureOutput(t *testing.T) {
	plainIcons := icons{
		Ok:      "✓",
		Info:    "●",
		Error:   "𐄂",
		Warning: "!",
	}
	coloredIcons := icons{
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
		wantIcons             icons
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
		ConfigureOutput(true, true, false)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigureOutput(tt.animated, tt.colored, tt.machineReadable)

			if got := AreAnimationsEnabled(); got != tt.wantAnimationsEnabled {
				t.Errorf("AreAnimationsEnabled() = %v, want %v", got, tt.wantAnimationsEnabled)
			}
			if got := IsOutputMachineReadable(); got != tt.wantMachineReadable {
				t.Errorf("IsOutputMachineReadable() = %v, want %v", got, tt.wantMachineReadable)
			}
			if Icons != tt.wantIcons {
				t.Errorf("Icons = %#v, want %#v", Icons, tt.wantIcons)
			}
		})
	}
}
