package systemd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseTimer(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantTimer   TimerInfo
		errContains string
	}{
		{
			name:        "empty input",
			input:       "",
			wantErr:     true,
			errContains: "empty input",
		},
		{
			name:        "whitespace only input",
			input:       "   \n\t  ",
			wantErr:     true,
			errContains: "empty input",
		},
		{
			name:        "invalid JSON",
			input:       `{"invalid json`,
			wantErr:     true,
			errContains: "failed to unmarshal JSON",
		},
		{
			name:        "empty array",
			input:       `[]`,
			wantErr:     true,
			errContains: "no timers found",
		},
		{
			name: "single timer with single activate unit",
			input: `[{
				"next": 1234567890000000,
				"left": 3600000000,
				"last": 1234567880000000,
				"passed": 10000000,
				"unit": "dnf-makecache.timer",
				"activates": "dnf-makecache.service"
			}]`,
			wantErr: false,
			wantTimer: TimerInfo{
				Next:      1234567890000000,
				Left:      3600000000,
				Last:      1234567880000000,
				Passed:    10000000,
				Unit:      "dnf-makecache.timer",
				Activates: []string{"dnf-makecache.service"},
			},
		},
		{
			name: "single timer with multiple activate units",
			input: `[{
				"next": 1234567890000000,
				"left": 3600000000,
				"last": 1234567880000000,
				"passed": 10000000,
				"unit": "multi.timer",
				"activates": ["service1.service", "service2.service"]
			}]`,
			wantErr: false,
			wantTimer: TimerInfo{
				Next:      1234567890000000,
				Left:      3600000000,
				Last:      1234567880000000,
				Passed:    10000000,
				Unit:      "multi.timer",
				Activates: []string{"service1.service", "service2.service"},
			},
		},
		{
			name: "timer with zero timestamps",
			input: `[{
				"next": 0,
				"left": 0,
				"last": 0,
				"passed": 0,
				"unit": "inactive.timer",
				"activates": "inactive.service"
			}]`,
			wantErr: false,
			wantTimer: TimerInfo{
				Next:      0,
				Left:      0,
				Last:      0,
				Passed:    0,
				Unit:      "inactive.timer",
				Activates: []string{"inactive.service"},
			},
		},
		{
			name: "timer with empty activates array",
			input: `[{
				"next": 1234567890000000,
				"left": 3600000000,
				"last": 1234567880000000,
				"passed": 10000000,
				"unit": "noactivate.timer",
				"activates": []
			}]`,
			wantErr: false,
			wantTimer: TimerInfo{
				Next:      1234567890000000,
				Left:      3600000000,
				Last:      1234567880000000,
				Passed:    10000000,
				Unit:      "noactivate.timer",
				Activates: []string{},
			},
		},
		{
			name: "multiple timers in array - returns first one",
			input: `[
				{
					"next": 1111111111111111,
					"left": 1000000000,
					"last": 1111111100000000,
					"passed": 11111111,
					"unit": "first.timer",
					"activates": "first.service"
				},
				{
					"next": 2222222222222222,
					"left": 2000000000,
					"last": 2222222200000000,
					"passed": 22222222,
					"unit": "second.timer",
					"activates": "second.service"
				}
			]`,
			wantErr: false,
			wantTimer: TimerInfo{
				Next:      1111111111111111,
				Left:      1000000000,
				Last:      1111111100000000,
				Passed:    11111111,
				Unit:      "first.timer",
				Activates: []string{"first.service"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimer(tt.input)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTimer() expected error, got nil")
					return
				}
				if tt.errContains != "" {
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("parseTimer() error = %v, want error containing %q", err, tt.errContains)
					}
				}
				return
			}

			// No error expected
			if err != nil {
				t.Errorf("parseTimer() unexpected error = %v", err)
				return
			}

			// Compare results
			if got.Next != tt.wantTimer.Next {
				t.Errorf("parseTimer() Next = %v, want %v", got.Next, tt.wantTimer.Next)
			}
			if got.Left != tt.wantTimer.Left {
				t.Errorf("parseTimer() Left = %v, want %v", got.Left, tt.wantTimer.Left)
			}
			if got.Last != tt.wantTimer.Last {
				t.Errorf("parseTimer() Last = %v, want %v", got.Last, tt.wantTimer.Last)
			}
			if got.Passed != tt.wantTimer.Passed {
				t.Errorf("parseTimer() Passed = %v, want %v", got.Passed, tt.wantTimer.Passed)
			}
			if got.Unit != tt.wantTimer.Unit {
				t.Errorf("parseTimer() Unit = %v, want %v", got.Unit, tt.wantTimer.Unit)
			}

			// Compare Activates slices
			if diff := cmp.Diff(tt.wantTimer.Activates, got.Activates); diff != "" {
				t.Errorf("parseTimer() Activates mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsValidTimerName(t *testing.T) {
	tests := []struct {
		name      string
		timerName string
		want      bool
	}{
		// Valid timer names
		{
			name:      "simple timer name",
			timerName: "rhc-collector.timer",
			want:      true,
		},
		{
			name:      "timer with instance",
			timerName: "rhc-collector@instance.timer",
			want:      true,
		},
		{
			name:      "timer with numbers",
			timerName: "timer-123.timer",
			want:      true,
		},
		{
			name:      "timer with underscores",
			timerName: "my_timer_name.timer",
			want:      true,
		},
		{
			name:      "timer with colons",
			timerName: "some:timer:name.timer",
			want:      true,
		},
		{
			name:      "timer with dots in name",
			timerName: "some.timer.name.timer",
			want:      true,
		},
		{
			name:      "timer with backslashes",
			timerName: "some\\timer.timer",
			want:      true,
		},
		{
			name:      "timer with empty instance",
			timerName: "rhc-collector@.timer",
			want:      true,
		},
		{
			name:      "timer with complex instance",
			timerName: "rhc@inst-123_test.timer",
			want:      true,
		},
		// Invalid timer names
		{
			name:      "missing .timer suffix",
			timerName: "rhc-collector",
			want:      false,
		},
		{
			name:      "timer with @ in instance",
			timerName: "rhc@inst@test.timer",
			want:      false,
		},
		{
			name:      "wrong suffix",
			timerName: "rhc-collector.service",
			want:      false,
		},
		{
			name:      "empty string",
			timerName: "",
			want:      false,
		},
		{
			name:      "only .timer",
			timerName: ".timer",
			want:      false,
		},
		{
			name:      "starts with @",
			timerName: "@instance.timer",
			want:      false,
		},
		{
			name:      "spaces in name",
			timerName: "rhc collector.timer",
			want:      false,
		},
		{
			name:      "special characters",
			timerName: "rhc-collector!.timer",
			want:      false,
		},
		{
			name:      "multiple @ symbols before instance",
			timerName: "rhc@@instance.timer",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTimerName(tt.timerName)
			if got != tt.want {
				t.Errorf("isValidTimerName(%q) = %v, want %v", tt.timerName, got, tt.want)
			}
		})
	}
}
