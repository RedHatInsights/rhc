package collector

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
			errorContains: "failed with status code: 400",
		},
		{
			name:          "server error 500",
			statusCode:    500,
			responseBody:  `{"error": "internal server error"}`,
			expectedError: true,
			errorContains: "failed with status code: 500",
		},
		{
			name:          "redirect should fail",
			statusCode:    302,
			responseBody:  `{"error": "redirect"}`,
			expectedError: true,
			errorContains: "failed with status code: 302",
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
		Buffer:      bytes.NewBufferString("test form data"),
		ContentType: "multipart/form-data; boundary=test123",
	}
	testConfig := ServiceConfig{
		URL:            "https://test.example.com/upload",
		ClientCertPath: "/test/cert.pem",
		ClientKeyPath:  "/test/key.pem",
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
	testFile := filepath.Join(tempDir, "test-archive.tar.xz")
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
	if formData.Buffer == nil {
		t.Error("Expected form data buffer to be non-nil")
	}

	// Read the form data from the buffer
	bufferContent := formData.Buffer.String()

	// Verify the buffer contains the filename
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
		Path:        "/nonexistent/file.tar.xz",
		ContentType: "application/vnd.redhat.advisor.collection",
	}

	_, err := createMultipartForm(archive)
	if err == nil {
		t.Error("Expected error when creating multipart form with nonexistent file, got none")
	}
}

func TestValidateArchive(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()

	// Create test files
	validFile := filepath.Join(tempDir, "test.tar.xz")
	err := os.WriteFile(validFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test directory
	testDir := filepath.Join(tempDir, "testdir.tar.xz")
	err = os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name          string
		archive       ArchiveDto
		expectedError bool
		errorContains string
	}{
		{
			name: "valid archive",
			archive: ArchiveDto{
				Path:        validFile,
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: false,
		},
		{
			name: "empty path",
			archive: ArchiveDto{
				Path:        "",
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: true,
			errorContains: "path or content type is required",
		},
		{
			name: "whitespace path",
			archive: ArchiveDto{
				Path:        "   ",
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: true,
			errorContains: "path or content type is required",
		},
		{
			name: "empty content type",
			archive: ArchiveDto{
				Path:        validFile,
				ContentType: "",
			},
			expectedError: true,
			errorContains: "path or content type is required",
		},
		{
			name: "whitespace content type",
			archive: ArchiveDto{
				Path:        validFile,
				ContentType: "   ",
			},
			expectedError: true,
			errorContains: "path or content type is required",
		},
		{
			name: "nonexistent file",
			archive: ArchiveDto{
				Path:        "/nonexistent/file.tar.xz",
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: true,
			errorContains: "archive file does not exist",
		},
		{
			name: "directory instead of file",
			archive: ArchiveDto{
				Path:        testDir,
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: true,
			errorContains: "path is not a .tar.xz file",
		},
		{
			name: "wrong file extension",
			archive: ArchiveDto{
				Path:        validFile,
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: false, // This should pass since it has .tar.xz extension
		},
		{
			name: "file without tar.xz extension",
			archive: ArchiveDto{
				Path:        filepath.Join(tempDir, "test.txt"),
				ContentType: "application/vnd.redhat.advisor.collection",
			},
			expectedError: true,
			errorContains: "path is not a .tar.xz file",
		},
	}

	// Create the test.txt file for the last test case
	txtFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(txtFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create txt test file: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArchive(tt.archive)

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

func TestValidateArchivePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "normal filename",
			path:     "/path/to/archive.tar.xz",
			expected: "archive.tar.xz",
		},
		{
			name:     "filename with double quotes",
			path:     `/path/to/"malicious".tar.xz`,
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with single quotes",
			path:     "/path/to/'malicious'.tar.xz",
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with newlines",
			path:     "/path/to/mali\ncious.tar.xz",
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with carriage returns",
			path:     "/path/to/mali\rcious.tar.xz",
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with tabs",
			path:     "/path/to/mali\tcious.tar.xz",
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with slashes and backslashes",
			path:     "//path///to////malicious\\.tar.xz",
			expected: "malicious.tar.xz",
		},
		{
			name:     "filename with multiple dangerous chars",
			path:     "/path/to/\"ma\tli'cio\r\nus.ta\\r.xz",
			expected: "malicious.tar.xz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateArchivePath(tt.path)
			if result != tt.expected {
				t.Errorf("validateArchivePath(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}
