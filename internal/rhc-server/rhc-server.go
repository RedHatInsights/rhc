package server

import (
	"fmt"

	"github.com/redhatinsights/rhc/varlink/internalapi"
)

var (
	// Version is set at build time
	Version = "dev"
)

// Backend implements the internal API backend
type Backend struct{}

// NewBackend creates a new backend instance
func NewBackend() *Backend {
	return &Backend{}
}

// Test implements the Test method of the internal API
// Simply echoes back the input with a prefix
func (b *Backend) Test(in *internalapi.TestIn) (*internalapi.TestOut, error) {
	output := fmt.Sprintf("Echo from rhc-server: %s", in.Input)
	return &internalapi.TestOut{Output: output}, nil
}
