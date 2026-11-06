package main

import (
	"context"
	"encoding/json"

	"github.com/varlink/go/varlink"
)

// varlinkDispatcher is the interface required by varlink.Service for a
// registered interface.
type varlinkDispatcher interface {
	VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error
	VarlinkGetName() string
	VarlinkGetDescription() string
}

// emptyParametersCompat makes method calls without a "parameters" member work.
//
// Newer systemd versions omit the "parameters" member entirely when it is an
// empty object. The generated varlink/go dispatchers call GetParameters() for
// every method that declares parameters, even optional ones, and that method
// returns an error for a missing "parameters" member. Inject an empty object
// before delegating so such calls are treated as calls without parameters.
type emptyParametersCompat struct {
	varlinkDispatcher
}

func (c emptyParametersCompat) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	if call.In != nil && call.In.Parameters == nil {
		empty := json.RawMessage(`{}`)
		call.In.Parameters = &empty
	}
	return c.varlinkDispatcher.VarlinkDispatch(ctx, call, methodname)
}
