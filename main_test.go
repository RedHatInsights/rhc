package main

import (
	"reflect"
	"slices"
	"testing"
)

func featureIDs(feature []*Feature) []string {
	result := make([]string, len(feature))
	for i, f := range feature {
		result[i] = f.ID
	}
	return result
}

// TestResolveFeatureInput tests that feature dependencies are correctly resolved.
// When feature is added, all its dependencies are added.
// When feature is removed, all its dependents are removed.
// When feature is requested to be both added and removed, error is returned.
func TestResolveFeatureInput(t *testing.T) {
	shouldWord := []struct {
		description string
		inEnable    []string
		inDisable   []string
		wantEnable  []string
		wantDisable []string
	}{
		{
			description: "+identity",
			inEnable:    []string{"identity"},
			inDisable:   []string{},
			wantEnable:  []string{"identity"},
			wantDisable: []string{},
		},
		{
			description: "+content",
			inEnable:    []string{"content"},
			inDisable:   []string{},
			wantEnable:  []string{"content", "identity"},
			wantDisable: []string{},
		},
		{
			description: "+management",
			inEnable:    []string{"management"},
			inDisable:   []string{},
			wantEnable:  []string{"analytics", "content", "identity", "management"},
			wantDisable: []string{},
		},
		{
			description: "-analytics",
			inEnable:    []string{},
			inDisable:   []string{"analytics"},
			wantEnable:  []string{},
			wantDisable: []string{"analytics", "compliance", "malware", "management"},
		},
		{
			description: "-content",
			inEnable:    []string{},
			inDisable:   []string{"content"},
			wantEnable:  []string{},
			wantDisable: []string{"content", "management"},
		},
		{
			description: "-management",
			inEnable:    []string{},
			inDisable:   []string{"management"},
			wantEnable:  []string{},
			wantDisable: []string{"management"},
		},
	}

	for _, test := range shouldWord {
		t.Run(test.description, func(t *testing.T) {
			gotEnableF, gotDisableF, err := resolveFeatureInput(test.inEnable, test.inDisable)
			if err != nil {
				t.Fatal(err)
			}
			gotEnable := featureIDs(gotEnableF)
			gotDisable := featureIDs(gotDisableF)
			slices.Sort(gotEnable)
			slices.Sort(gotDisable)

			if !reflect.DeepEqual(gotEnable, test.wantEnable) {
				t.Errorf("want <%s>, got <%s>", test.wantEnable, gotEnable)
			}
			if !reflect.DeepEqual(gotDisable, test.wantDisable) {
				t.Errorf("want <%s>, got <%s>", test.wantDisable, gotDisable)
			}
		})
	}

	shouldFail := []struct {
		description string
		inEnable    []string
		inDisable   []string
		error       string
	}{
		{
			description: "identity cannot be disabled",
			inEnable:    []string{},
			inDisable:   []string{"identity"},
			error:       "feature 'identity' cannot be disabled",
		},
		{
			description: "management needs analytics",
			inEnable:    []string{"management"},
			inDisable:   []string{"analytics"},
			error:       "features can't be enabled and disabled at the same time: analytics, management",
		},
		{
			description: "malware needs analytics",
			inEnable:    []string{"malware"},
			inDisable:   []string{"analytics"},
			error:       "features can't be enabled and disabled at the same time: analytics, malware",
		},
	}

	for _, test := range shouldFail {
		t.Run(test.description, func(t *testing.T) {
			_, _, err := resolveFeatureInput(test.inEnable, test.inDisable)

			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != test.error {
				t.Errorf("want <%v>, got <%v>", test.error, err)
			}
		})
	}
}
