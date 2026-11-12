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
	"github.com/redhatinsights/rhc/internal/rhcconf"
)

const maxResponseBodySize = 1024

// ArchiveDto represents an archive file with its path and MIME content type.
type ArchiveDto struct {
	// Path is a path to the archive file.
	Path string
	// ContentType is the MIME type of the archive (e.g., "application/vnd.redhat.advisor.collection").
	ContentType string
}

// systemCAPath is the sentinel ca-path value that selects the system trust
// store instead of an explicit CA bundle.
const systemCAPath = "system"

// ServiceConfig represents the configuration for an upload service endpoint.
type ServiceConfig struct {
	// URL is the endpoint where archive will be uploaded.
	URL string
	// CACertPath is the CA bundle used to verify the server's TLS certificate.
	// Empty or "system" selects the system trust store; otherwise it is a path
	// to a PEM file or a directory of *.pem files to add to the trust store.
	CACertPath string
	// TLSVerify controls whether the server's TLS certificate is verified.
	TLSVerify bool
	// Timeouts are the HTTP timeouts applied to the upload client.
	Timeouts httpapi.Timeouts
}

// UploadArchive uploads an archive file to the Red Hat Hybrid Cloud Console.
func UploadArchive(archive ArchiveDto, config ServiceConfig, userAgent string) error {
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
	client := httpapi.NewHTTPClient(tlsConfig, config.Timeouts)
	req, err := createUploadRequest(formData, config, userAgent)
	if err != nil {
		return err
	}
	if err := sendUploadRequest(client, req); err != nil {
		return err
	}

	slog.Info("Successfully uploaded archive", slog.String("archive", archive.Path))
	return nil
}

// loadClientCertificate builds the TLS configuration for the upload: the client
// identity certificate plus the server trust store and verification policy
// resolved from the service configuration.
func loadClientCertificate(config ServiceConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(rhcconf.ConsumerCertPath, rhcconf.ConsumerKeyPath)
	if err != nil {
		slog.Error("Failed to load client certificate", "error", err)
		return nil, fmt.Errorf("failed to load client certificate from %s and %s: %w", rhcconf.ConsumerCertPath, rhcconf.ConsumerKeyPath, err)
	}
	return buildTLSConfig(cert, config)
}

// buildTLSConfig assembles the TLS configuration from the client identity
// certificate and the server trust store and verification policy in config.
func buildTLSConfig(cert tls.Certificate, config ServiceConfig) (*tls.Config, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		slog.Error("Failed to load system certificate pool", "error", err)
		return nil, fmt.Errorf("failed to load system certificates: %w", err)
	}
	// An explicit CA path is added to the system trust store so a Satellite's
	// self-signed CA is trusted alongside it.
	if config.CACertPath != "" && config.CACertPath != systemCAPath {
		if err := addCACerts(pool, config.CACertPath); err != nil {
			return nil, err
		}
	}
	return &tls.Config{
		RootCAs:            pool,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: !config.TLSVerify,
	}, nil
}

// addCACerts adds the PEM certificate(s) at caPath to pool. caPath may be a
// single PEM file or a directory, in which case every *.pem file in it is added.
func addCACerts(pool *x509.CertPool, caPath string) error {
	info, err := os.Stat(caPath)
	if err != nil {
		return fmt.Errorf("failed to access CA path %s: %w", caPath, err)
	}

	var files []string
	if info.IsDir() {
		matches, err := filepath.Glob(filepath.Join(caPath, "*.pem"))
		if err != nil {
			return fmt.Errorf("failed to list CA directory %s: %w", caPath, err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("no *.pem files found in CA directory %s", caPath)
		}
		files = matches
	} else {
		files = []string{caPath}
	}

	for _, file := range files {
		pem, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read CA file %s: %w", file, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("no valid certificates found in CA file %s", file)
		}
	}
	return nil
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
func createUploadRequest(formData multipartData, config ServiceConfig, userAgent string) (*http.Request, error) {
	req, err := http.NewRequest("POST", config.URL, formData.Buffer)
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return nil, fmt.Errorf("failed to create HTTP POST request to %s: %w", config.URL, err)
	}
	req.Header.Set("Content-Type", formData.ContentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

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
