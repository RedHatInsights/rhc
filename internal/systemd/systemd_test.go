package systemd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestUnit(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	tests := []struct {
		description string
		input       struct {
			unitFile string
			now      bool
		}
		want      string
		wantError error
	}{
		{
			description: "activate simple.service",
			input: struct {
				unitFile string
				now      bool
			}{
				unitFile: "testdata/simple.service",
				now:      true,
			},
			want: "active",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			conn, err := NewConnectionContext(context.Background(), ConnectionTypeUser)
			if err != nil {
				t.Fatal(err)
			}

			abs, err := filepath.Abs(test.input.unitFile)
			if err != nil {
				t.Fatal(err)
			}

			// Link the testdata unit files into the search paths so they can be
			// referenced by name. This simulates the context in which this
			// package is more likely to be run.
			_, err = conn.conn.LinkUnitFilesContext(conn.ctx, []string{abs}, true, false)
			if err != nil {
				t.Fatal(err)
			}

			err = conn.EnableUnit(filepath.Base(test.input.unitFile), test.input.now, true)
			defer func(t *testing.T) {
				if err := conn.DisableUnit(filepath.Base(test.input.unitFile), test.input.now, true); err != nil {
					t.Fatal(err)
				}
			}(t)
			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				got, err := conn.GetUnitState(filepath.Base(test.input.unitFile))
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("%v", cmp.Diff(got, test.want))
				}
			}
		})
	}
}
