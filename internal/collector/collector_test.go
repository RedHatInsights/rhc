package collector

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// stringPtr returns a pointer to the given string value
func stringPtr(s string) *string {
	return &s
}

func TestNewConfig(t *testing.T) {
	tests := []struct {
		description string
		input       *configDto
		id          string
		want        Config
		wantError   string
	}{
		{
			description: "valid config",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test valid config",
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id: "test.valid.config",
			want: Config{
				ID:                 "test.valid.config",
				Name:               "Test valid config",
				IsAnalyticsFeature: true,
				User:               "root",
				Group:              "root",
				ContentType:        "application/vnd.redhat.advisor.collection",
			},
		},
		{
			description: "no user defined",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test no user defined",
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id: "test.no.user.defined",
			want: Config{
				ID:                 "test.no.user.defined",
				Name:               "Test no user defined",
				IsAnalyticsFeature: true,
				User:               "root",
				Group:              "root",
				ContentType:        "application/vnd.redhat.advisor.collection",
			},
		},
		{
			description: "no group defined",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test no group defined",
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id: "test.no.group.defined",
			want: Config{
				ID:                 "test.no.group.defined",
				Name:               "Test no group defined",
				IsAnalyticsFeature: true,
				User:               "root",
				Group:              "root",
				ContentType:        "application/vnd.redhat.advisor.collection",
			},
		},
		{
			description: "nil feature field",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test nil feature",
					Feature: nil,
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id: "test.nil.feature",
			want: Config{
				ID:                 "test.nil.feature",
				Name:               "Test nil feature",
				IsAnalyticsFeature: true,
				User:               "root",
				Group:              "root",
				ContentType:        "application/vnd.redhat.advisor.collection",
			},
		},
		{
			description: "non-analytics feature",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test non-analytics feature",
					Feature: stringPtr("monitoring"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id: "test.non.analytics.feature",
			want: Config{
				ID:                 "test.non.analytics.feature",
				Name:               "Test non-analytics feature",
				IsAnalyticsFeature: false,
				User:               "root",
				Group:              "root",
				ContentType:        "application/vnd.redhat.advisor.collection",
			},
		},
		{
			description: "empty config",
			input:       &configDto{},
			id:          "test.empty.config",
			wantError:   "invalid config: meta section is required",
		},
		{
			description: "missing meta section",
			input: &configDto{
				Meta: nil,
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id:        "test.missing.meta",
			wantError: "invalid config: meta section is required",
		},
		{
			description: "missing meta name",
			input: &configDto{
				Meta: &metaDto{
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id:        "test.missing.meta.name",
			wantError: "invalid config: meta.name is required",
		},
		{
			description: "missing meta type",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test missing meta type",
					Feature: stringPtr("analytics"),
				},
				Ingress: &ingressDto{
					User:        stringPtr("root"),
					Group:       stringPtr("root"),
					ContentType: "application/vnd.redhat.advisor.collection",
				},
			},
			id:        "test.missing.meta.type",
			wantError: "invalid config: meta.type must be 'ingress'",
		},
		{
			description: "missing ingress section",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test missing ingress section",
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
			},
			id:        "test.missing.ingress",
			wantError: "invalid config: ingress section is required",
		},
		{
			description: "missing ingress content_type",
			input: &configDto{
				Meta: &metaDto{
					Name:    "Test missing ingress content_type",
					Feature: stringPtr("analytics"),
					Type:    stringPtr("ingress"),
				},
				Ingress: &ingressDto{
					User:  stringPtr("root"),
					Group: stringPtr("root"),
				},
			},
			id:        "test.missing.ingress.content_type",
			wantError: "invalid config: ingress.content_type is required",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := newConfig(test.id, test.input)

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Errorf("Expected error %q, got %v", test.wantError, err)
				}
			} else {
				if err != nil {
					t.Errorf("newConfig(%q, %v) got unexpected error: %v", test.id, test.input, err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("newConfig(%v) = %v; want %v", test.input, got, test.want)
				}
			}
		})
	}
}

func TestParseConfigFromContent(t *testing.T) {
	tests := []struct {
		description string
		content     string
		id          string
		want        Config
		wantError   string
	}{
		{
			description: "valid TOML content",
			content: `
  [meta]
  name = "Test Config"
  feature = "analytics"
  type = "ingress"

  [ingress]
  user = "root"
  group = "root"
  content_type = "application/test"
  `,
			id: "test.config",
			want: Config{
				ID:                 "test.config",
				Name:               "Test Config",
				IsAnalyticsFeature: true,
				User:               "root",
				Group:              "root",
				ContentType:        "application/test",
			},
		},
		{
			description: "invalid TOML syntax",
			content: `
[meta]
name = "Test Invalid TOML syntax
feature = "analytics"
type = "ingress"

[ingress]
user = "root"
group = "root"
content_type = "application/test"
`,
			id:        "test.invalid.toml",
			wantError: "toml: line 3 (last key \"meta.name\"): strings cannot contain newlines",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := parseConfigFromContent(test.content, test.id)

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Errorf("Expected error %q, got %v", test.wantError, err)
				}
			} else {
				if err != nil {
					t.Errorf("parseConfigFromContent(%q, %q) got unexpected error: %v", test.content, test.id, err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("parseConfigFromContent(%q) = %v; want %v", test.content, got, test.want)
				}
			}
		})
	}
}
