package ui

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrintTable(t *testing.T) {
	tests := []struct {
		description string
		input       [][]string
		sep         string
		termWidth   int
		want        string
	}{
		{
			description: "simple table",
			input: [][]string{
				{"FEATURE", "CONFIG", "STATE"},
				{"remote-management", "✓", "✓"},
			},
			sep:       "  ",
			termWidth: 80,
			want:      "FEATURE            CONFIG  STATE\nremote-management  ✓     ✓\n",
		},
		{
			description: "simple table 2",
			input: [][]string{
				{"FEATURE", "CONFIG", "STATE"},
				{"remote", "✓", "✓"},
			},
			sep:       "  ",
			termWidth: 80,
			want:      "FEATURE  CONFIG  STATE\nremote   ✓     ✓\n",
		},
		{
			description: "empty table",
			input:       [][]string{},
			sep:         "  ",
			termWidth:   80,
			want:        "",
		},
		{
			description: "single column",
			input: [][]string{
				{"HEADER"},
				{"value"},
			},
			sep:       "  ",
			termWidth: 80,
			want:      "HEADER\nvalue\n",
		},
		{
			description: "truncated row",
			input: [][]string{
				{"VERY_LONG_COLUMN_NAME", "ANOTHER_LONG_COLUMN"},
				{"this is a very long value that will be truncated", "short"},
			},
			sep:       "  ",
			termWidth: 30,
			want: "VERY_LONG_COLUMN_NAME      ...\n" +
				"this is a very long value t...\n",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Save original stdout
			oldStdout := os.Stdout

			// Create a pipe to capture output
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			os.Stdout = w

			// Call the function
			PrintTable(test.input, test.sep, test.termWidth)

			// Close write end and restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			got := buf.String()

			if !cmp.Equal(got, test.want) {
				t.Errorf("diff got vs want:\n--- got\n+++ want\n%v", cmp.Diff(got, test.want))
			}
		})
	}
}
