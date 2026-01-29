package collector

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	httpapi "github.com/redhatinsights/rhc/internal/http"
)

const maxResponseBodySize = 1024

// ArchiveDto represents an archive file with its path and MIME content type.
type ArchiveDto struct {
	// Path is a path to the archive file.
	Path string
	// ContentType is the MIME type of the archive (e.g., "application/vnd.redhat.advisor.collection").
	ContentType string
}

// ServiceConfig represents the configuration for an upload service endpoint.
type ServiceConfig struct {
	// URL is the endpoint where archive will be uploaded.
	URL string
	// ClientCertPath is the file path to the identity certificate.
	ClientCertPath string
	// ClientKeyPath is the file path to the private key associated with identity certificate.
	ClientKeyPath string
}

// UploadArchive uploads an archive file to the Red Hat Hybrid Cloud Console.
func UploadArchive(archive ArchiveDto, config ServiceConfig) error {
	if err := validateArchive(archive); err != nil {
		return err
	}

	slog.Info("Uploading archive", slog.String("archive", archive.Path), slog.String("url", config.URL))
	formData, err := createMultipartForm(archive)
	if err != nil {
		return err
	}
	tlsConfig, err := loadClientCertificate(config)
	if err != nil {
		return err
	}
	client := httpapi.NewHTTPClient(tlsConfig)
	req, err := createUploadRequest(formData, config)
	if err != nil {
		return err
	}
	if err := sendUploadRequest(client, req); err != nil {
		return err
	}

	slog.Info("Successfully uploaded archive", slog.String("archive", archive.Path))
	return nil
}

// loadClientCertificate loads X.509 client certificates from the provided service configuration.
// Returns an error if files cannot be read or parsed.
func loadClientCertificate(config ServiceConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.ClientCertPath, config.ClientKeyPath)
	if err != nil {
		slog.Error("Failed to load client certificate", "error", err)
		return nil, fmt.Errorf("failed to load client certificate from %s and %s: %w", config.ClientCertPath, config.ClientKeyPath, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		slog.Error("Failed to load system certificate pool", "error", err)
		return nil, fmt.Errorf("failed to load system certificates: %w", err)
	}
	return &tls.Config{RootCAs: pool, Certificates: []tls.Certificate{cert}}, nil
}

// validateArchive validates the archiveDto fields.
func validateArchive(archive ArchiveDto) error {
	if strings.TrimSpace(archive.Path) == "" || strings.TrimSpace(archive.ContentType) == "" {
		return fmt.Errorf("invalid archive: path or content type is required")
	}
	fileInfo, err := os.Stat(archive.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("archive file does not exist: %s", archive.Path)
	} else if err != nil {
		return fmt.Errorf("failed to access archive file: %w", err)
	}
	if fileInfo.IsDir() || !strings.HasSuffix(archive.Path, ".tar.xz") {
		return fmt.Errorf("invalid archive: path is not a .tar.xz file")
	}
	return nil
}

// validateArchivePath sanitizes the archive path for safe use in HTTP headers.
func validateArchivePath(path string) string {
	filename := filepath.Base(path)
	re := regexp.MustCompile(`["'\r\n\t\\]`)
	return re.ReplaceAllString(filename, "")
}

// multipartData encapsulates a multipart form reader and its content type.
type multipartData struct {
	Buffer      *bytes.Buffer
	ContentType string
}

// createMultipartForm creates multipart form data for archive file upload.
//
// This function loads the entire file into memory using bytes.Buffer to create multipart form data.
// Multipart form data is required by the Ingress service to properly handle file uploads with
// metadata (filename, content type). This allows the server to distinguish between the file content
// and form metadata in a single HTTP request.
//
// See: https://developers.redhat.com/api-catalog/api/payload_ingress.
func createMultipartForm(archive ArchiveDto) (multipartData, error) {
	// TODO: Investigate alternatives for large file handling
	buffer := new(bytes.Buffer)
	writer := multipart.NewWriter(buffer)

	archiveHeader := make(textproto.MIMEHeader)
	archiveHeader.Set(
		"Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, validateArchivePath(archive.Path)),
	)
	archiveHeader.Set("Content-Type", archive.ContentType)
	archiveFieldWriter, err := writer.CreatePart(archiveHeader)
	if err != nil {
		slog.Error("Failed to create archive field", "error", err)
		return multipartData{}, fmt.Errorf("failed to create multipart field for archive: %w", err)
	}
	dataContentType := writer.FormDataContentType()
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			slog.Error("Failed to close multipart writer", "error", closeErr)
		}
	}()
	archiveFileReader, err := os.Open(archive.Path)
	if err != nil {
		slog.Error("Failed to open archive", "error", err)
		return multipartData{}, fmt.Errorf("failed to open archive file %s: %w", archive.Path, err)
	}
	defer func() {
		if closeErr := archiveFileReader.Close(); closeErr != nil {
			slog.Error("Failed to close archive file", "error", closeErr)
		}
	}()
	if _, err = io.Copy(archiveFieldWriter, archiveFileReader); err != nil {
		slog.Error("Failed to copy archive", "error", err)
		return multipartData{}, fmt.Errorf("failed to copy archive data from %s to multipart form: %w", archive.Path, err)
	}

	return multipartData{
		Buffer:      buffer,
		ContentType: dataContentType,
	}, nil
}

// createUploadRequest creates an HTTP POST request for uploading multipart form data.
// Returns an error if request creation fails.
func createUploadRequest(formData multipartData, config ServiceConfig) (*http.Request, error) {
	// FIXME: Add proper User-Agent header to identify the rhc
	req, err := http.NewRequest("POST", config.URL, formData.Buffer)
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return nil, fmt.Errorf("failed to create HTTP POST request to %s: %w", config.URL, err)
	}
	req.Header.Set("Content-Type", formData.ContentType)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// sendUploadRequest executes an HTTP request and validates the response status.
// Returns an error if the request fails or status is not 2xx.
func sendUploadRequest(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to upload archive", "error", err)
		return fmt.Errorf("failed to execute HTTP request to %s: %w", req.URL.String(), err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Debug("Failed to close response body", "error", closeErr)
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		slog.Error("Failed to upload archive", "status code", resp.StatusCode, "url", req.URL.String())
		return fmt.Errorf("upload to %s failed with status code: %d", req.URL.String(), resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		slog.Warn("Failed to read response body", "url", req.URL.String(), "error", err)
		slog.Debug("Response status", slog.String("status", resp.Status))
	} else {
		slog.Debug("Response body", slog.String("body", string(body)), slog.String("status", resp.Status))
	}
	return nil
}
