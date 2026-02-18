package collector

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() fs.FileMode          { return 0 }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestGetCollectorConfigName(t *testing.T) {
	tests := []struct {
		name       string
		configFile mockDirEntry
		want       string
		wantError  string
	}{
		{
			name:       "valid toml file",
			configFile: mockDirEntry{name: "com.redhat.advisor.toml", isDir: false},
			want:       "com.redhat.advisor.toml",
			wantError:  "",
		},
		{
			name:       "directory with toml extension",
			configFile: mockDirEntry{name: "com.directory.toml", isDir: true},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/com.directory.toml",
		},
		{
			name:       "file without json extension",
			configFile: mockDirEntry{name: "com.config.json", isDir: false},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/com.config.json",
		},
		{
			name:       "file with toml in name but different extension",
			configFile: mockDirEntry{name: "com.config.toml.bak", isDir: false},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/com.config.toml.bak",
		},
		{
			name:       "file ending with toml but no dot",
			configFile: mockDirEntry{name: "configtoml", isDir: false},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/configtoml",
		},
		{
			name:       "directory without extension",
			configFile: mockDirEntry{name: "config.directory", isDir: true},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/config.directory",
		},
		{
			name:       "file starting with uppercase character",
			configFile: mockDirEntry{name: "Config.toml", isDir: false},
			want:       "Config.toml",
			wantError:  "",
		},
		{
			name:       "case sensitivity - uppercase extension",
			configFile: mockDirEntry{name: "config.TOML", isDir: false},
			want:       "",
			wantError:  "invalid config file /usr/lib/rhc/collectors/config.TOML",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := getConfigFilename(test.configFile)

			if test.wantError != "" {
				if err == nil {
					t.Errorf("getCollectorConfigName() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), test.wantError) {
					t.Errorf("getCollectorConfigName() error = %v, want error containing %q", err, test.wantError)
				}
				if got != test.want {
					t.Errorf("getCollectorConfigName() = %q, want %q", got, test.want)
				}
			} else {
				if err != nil {
					t.Errorf("getCollectorConfigName() unexpected error: %v", err)
				}
				if got != test.want {
					t.Errorf("getCollectorConfigName() = %q, want %q", got, test.want)
				}
			}
		})
	}
}

func createTestDirWithFiles(t *testing.T, files map[string]string) string {
	testDir := filepath.Join(t.TempDir(), "testdata")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	for filePath, content := range files {
		fullPath := filepath.Join(testDir, filePath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", fullPath, err)
		}
	}

	return testDir
}

func TestCreateArchive(t *testing.T) {
	t.Run("successful archive creation", func(t *testing.T) {
		testDir := createTestDirWithFiles(t, map[string]string{
			"file1.txt":        "content of file 1",
			"file2.txt":        "content of file 2",
			"subdir/file3.txt": "content of file 3",
		})
		outputDir := t.TempDir()
		archiveName := "test.tar.xz"

		archivePath, err := createArchive(archiveName, testDir, outputDir)

		if err != nil {
			t.Errorf("createArchive() unexpected error: %v", err)
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("createArchive() archive file does not exist at path: %s", archivePath)
		}
	})

	t.Run("nonexistent source directory", func(t *testing.T) {
		tempDir := t.TempDir()
		archiveName := "test.tar.xz"
		nonexistentDir := filepath.Join(tempDir, "nonexistent")

		_, err := createArchive(archiveName, nonexistentDir, tempDir)

		if err == nil {
			t.Error("createArchive() expected error for nonexistent directory")
		}
	})

	t.Run("invalid archive path", func(t *testing.T) {
		testDir := createTestDirWithFiles(t, map[string]string{"test.txt": "test"})
		// Use a nonexistent directory as an output dir to cause failure
		nonexistentOutputDir := "/nonexistent/path"
		archiveName := "test.tar.xz"

		_, err := createArchive(archiveName, testDir, nonexistentOutputDir)

		if err == nil {
			t.Error("createArchive() expected error for nonexistent output directory")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		testDir := createTestDirWithFiles(t, nil)
		outputDir := t.TempDir()
		archiveName := "empty.tar.xz"

		archivePath, err := createArchive(archiveName, testDir, outputDir)

		if err != nil {
			t.Errorf("createArchive() unexpected error for empty directory: %v", err)
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("createArchive() archive file does not exist at path: %s", archivePath)
		}
	})
}

func TestNewTimer(t *testing.T) {
	tests := []struct {
		description string
		id          string
		timerData   string
		want        Timer
		wantError   string
	}{
		{
			description: "valid timer data",
			id:          "test.collector",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.collector",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     0,
			},
		},
		{
			description: "valid timer with non-zero exit code",
			id:          "test.collector.error",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 1
				}
			}`,
			want: Timer{
				ID:           "test.collector.error",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     1,
			},
		},
		{
			description: "invalid JSON",
			id:          "test.invalid",
			timerData:   `{"last_started": {"timestamp": 1609459200`,
			wantError:   "unexpected end of JSON input",
		},
		{
			description: "missing last_started",
			id:          "test.missing.started",
			timerData: `{
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.missing.started",
				LastStarted:  time.Time{}, // Zero value
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     0,
			},
		},
		{
			description: "missing last_finished",
			id:          "test.missing.finished",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				}
			}`,
			want: Timer{
				ID:           "test.missing.finished",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Time{}, // Zero value
				ExitCode:     0,
			},
		},
		{
			description: "empty JSON object",
			id:          "test.empty",
			timerData:   "{}",
			want: Timer{
				ID:           "test.empty",
				LastStarted:  time.Time{}, // Zero value
				LastFinished: time.Time{}, // Zero value
				ExitCode:     0,
			},
		},
		{
			description: "missing timestamp in last_started",
			id:          "test.missing.started.timestamp",
			timerData: `{
				"last_started": {
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.missing.started.timestamp",
				LastStarted:  time.Unix(0, 0),
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     0,
			},
		},
		{
			description: "missing timestamp in last_finished",
			id:          "test.missing.finished.timestamp",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"exit_code": 1
				}
			}`,
			want: Timer{
				ID:           "test.missing.finished.timestamp",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Unix(0, 0),
				ExitCode:     1,
			},
		},
		{
			description: "negative timestamp in last_started",
			id:          "test.negative.started.timestamp",
			timerData: `{
				"last_started": {
					"timestamp": -1000
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.negative.started.timestamp",
				LastStarted:  time.Unix(-1000, 0),
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     0,
			},
		},
		{
			description: "negative timestamp in last_finished",
			id:          "test.negative.finished.timestamp",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"timestamp": -5000,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.negative.finished.timestamp",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Unix(-5000, 0),
				ExitCode:     0,
			},
		},
		{
			description: "invalid timestamp type in last_started",
			id:          "test.invalid.started.timestamp.type",
			timerData: `{
				"last_started": {
					"timestamp": "invalid"
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			wantError: "json: cannot unmarshal string into Go struct field startedEventDto.last_started.timestamp of type int64",
		},
		{
			description: "invalid timestamp type in last_finished",
			id:          "test.invalid.finished.timestamp.type",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"timestamp": "invalid",
					"exit_code": 0
				}
			}`,
			wantError: "json: cannot unmarshal string into Go struct field finishedEventDto.last_finished.timestamp of type int64",
		},
		{
			description: "invalid exit_code type in last_finished",
			id:          "test.invalid.finished.exit_code.type",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": "invalid"
				}
			}`,
			wantError: "json: cannot unmarshal string into Go struct field finishedEventDto.last_finished.exit_code of type int",
		},
		{
			description: "null last_started event object",
			id:          "test.null.started",
			timerData: `{
				"last_started": null,
				"last_finished": {
					"timestamp": 1609462800,
					"exit_code": 0
				}
			}`,
			want: Timer{
				ID:           "test.null.started",
				LastStarted:  time.Time{}, // Zero value
				LastFinished: time.Unix(1609462800, 0),
				ExitCode:     0,
			},
		},
		{
			description: "null last_finished event object",
			id:          "test.null.finished",
			timerData: `{
				"last_started": {
					"timestamp": 1609459200
				},
				"last_finished": null
			}`,
			want: Timer{
				ID:           "test.null.finished",
				LastStarted:  time.Unix(1609459200, 0),
				LastFinished: time.Time{}, // Zero value
				ExitCode:     0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := newTimer(test.id, []byte(test.timerData))

			if test.wantError != "" {
				if err == nil {
					t.Errorf("newTimer() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), test.wantError) {
					t.Errorf("newTimer() error = %v, want error containing %q", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Errorf("newTimer(%q, %q) got unexpected error: %v", test.id, test.timerData, err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("newTimer() = %v; want %v", got, test.want)
				}
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	valid := [5]string{"com.redhat", "com.redhat.advisor", "org.example.collector.v1", "a.b", "v1.example2.collector3"}
	for _, id := range valid {
		t.Run("valid_"+id, func(t *testing.T) {
			gotId, err := validateID(id)
			if err != nil {
				t.Errorf("validateCollectorID(%q) got unexpected error: %v", id, err)
			}
			if id != gotId {
				t.Errorf("validateCollectorID(%q) = %q; want %q", id, gotId, id)
			}
		})
	}

	invalid := [13]string{
		"", "...", "org", "single.", "two..dots", "com.red-hat.advisor", ".invalid.id",
		"Com.RedHat.Advisor", "com.red_hat.advisor", "com.red@hat.advisor", "com.red hat.advisor",
		"/absolute/path/com.redhat.advisor", "relativepath/com.redhat.id",
	}
	for _, id := range invalid {
		t.Run("invalid_"+id, func(t *testing.T) {
			gotId, err := validateID(id)
			if err == nil {
				t.Errorf("validateCollectorID(%q) expected error but got none", id)
			}
			if gotId != "" {
				t.Errorf("validateCollectorID(%q) = %q; want empty string", id, gotId)
			}
		})
	}
}

func TestSendUploadRequest(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		errorContains string
	}{
		{
			name:          "successful upload",
			statusCode:    200,
			responseBody:  `{"status": "success"}`,
			expectedError: false,
		},
		{
			name:          "successful upload with 201",
			statusCode:    201,
			responseBody:  `{"id": "12345"}`,
			expectedError: false,
		},
		{
			name:          "client error 400",
			statusCode:    400,
			responseBody:  `{"error": "bad request"}`,
			expectedError: true,
			errorContains: "upload failed with status code: 400",
		},
		{
			name:          "server error 500",
			statusCode:    500,
			responseBody:  `{"error": "internal server error"}`,
			expectedError: true,
			errorContains: "upload failed with status code: 500",
		},
		{
			name:          "redirect should fail",
			statusCode:    302,
			responseBody:  `{"error": "redirect"}`,
			expectedError: true,
			errorContains: "upload failed with status code: 302",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.responseBody))
				if err != nil {
					return
				}
			}))
			defer server.Close()

			// Create HTTP client and request
			client := &http.Client{Timeout: 5 * time.Second}
			req, err := http.NewRequest("POST", server.URL, strings.NewReader("test data"))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Test sendUploadRequest
			err = sendUploadRequest(client, req)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestCreateUploadRequest(t *testing.T) {
	testData := multipartData{
		Buffer:      bytes.NewBuffer([]byte("test form data")),
		ContentType: "multipart/form-data; boundary=test123",
	}
	testConfig := ServiceConfig{
		URL:           "https://test.example.com/upload",
		CertPath:      "/test/cert.pem",
		ClientKeyPath: "/test/key.pem",
	}

	req, err := createUploadRequest(testData, testConfig)
	if err != nil {
		t.Fatalf("createUploadRequest failed: %v", err)
	}

	// Verify HTTP method
	if req.Method != "POST" {
		t.Errorf("Expected method POST, got %s", req.Method)
	}

	// Verify URL
	if req.URL.String() != testConfig.URL {
		t.Errorf("Expected URL %s, got %s", testConfig.URL, req.URL.String())
	}

	// Verify Content-Type header
	contentType := req.Header.Get("Content-Type")
	if contentType != testData.ContentType {
		t.Errorf("Expected Content-Type %s, got %s", testData.ContentType, contentType)
	}

	// Verify Accept header
	accept := req.Header.Get("Accept")
	if accept != "application/json" {
		t.Errorf("Expected Accept application/json, got %s", accept)
	}

	// Verify body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("Failed to read request body: %v", err)
	}
	if string(body) != "test form data" {
		t.Errorf("Expected body 'test form data', got %s", string(body))
	}
}

func TestCreateMultipartForm(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test-archive.tar.gz")
	testContent := "This is test archive content"

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	archive := ArchiveDto{
		Path:        testFile,
		ContentType: "application/gzip",
	}

	formData, err := createMultipartForm(archive)
	if err != nil {
		t.Fatalf("createMultipartForm failed: %v", err)
	}

	// Verify content type is multipart
	if !strings.HasPrefix(formData.ContentType, "multipart/form-data") {
		t.Errorf("Expected multipart/form-data content type, got %s", formData.ContentType)
	}

	// Verify buffer contains data
	if formData.Buffer.Len() == 0 {
		t.Error("Expected form data buffer to contain data")
	}

	// Verify the buffer contains the filename
	bufferContent := formData.Buffer.String()
	expectedFilename := filepath.Base(testFile)
	if !strings.Contains(bufferContent, expectedFilename) {
		t.Errorf("Expected buffer to contain filename %s", expectedFilename)
	}

	// Verify the buffer contains the content type
	if !strings.Contains(bufferContent, archive.ContentType) {
		t.Errorf("Expected buffer to contain content type %s", archive.ContentType)
	}

	// Verify the buffer contains the file content
	if !strings.Contains(bufferContent, testContent) {
		t.Errorf("Expected buffer to contain file content")
	}
}

func TestCreateMultipartFormFileNotFound(t *testing.T) {
	archive := ArchiveDto{
		Path:        "/nonexistent/file.tar.gz",
		ContentType: "application/gzip",
	}

	_, err := createMultipartForm(archive)
	if err == nil {
		t.Error("Expected error for nonexistent file, got none")
	}
}
