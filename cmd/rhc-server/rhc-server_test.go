package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/redhatinsights/rhc/varlink/internalapi"
)

func TestNewBackend(t *testing.T) {
	backend := NewBackend()

	if backend == nil {
		t.Fatal("NewBackend() returned nil")
	}
}

func TestBackend_Test(t *testing.T) {
	tests := []struct {
		description string
		input       *internalapi.TestIn
		want        *internalapi.TestOut
	}{
		{
			description: "simple message",
			input: &internalapi.TestIn{
				Input: "hello",
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: hello",
			},
		},
		{
			description: "empty string",
			input: &internalapi.TestIn{
				Input: "",
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: ",
			},
		},
		{
			description: "message with special characters",
			input: &internalapi.TestIn{
				Input: "hello!@#$%^&*()",
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: hello!@#$%^&*()",
			},
		},
		{
			description: "message with newlines",
			input: &internalapi.TestIn{
				Input: "line1\nline2\nline3",
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: line1\nline2\nline3",
			},
		},
		{
			description: "message with unicode",
			input: &internalapi.TestIn{
				Input: "Hello 世界 🌍",
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: Hello 世界 🌍",
			},
		},
		{
			description: "very long message",
			input: &internalapi.TestIn{
				Input: string(make([]byte, 10000)),
			},
			want: &internalapi.TestOut{
				Output: "Echo from rhc-server: " + string(make([]byte, 10000)),
			},
		},
	}

	backend := NewBackend()

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := backend.Test(test.input)

			if err != nil {
				t.Errorf("Backend.Test(%v) unexpected error: %v", test.input, err)
			}

			if !cmp.Equal(got, test.want) {
				t.Errorf("Backend.Test(%v) = %v; want %v", test.input, got, test.want)
			}
		})
	}
}
