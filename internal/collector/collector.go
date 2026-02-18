package collector

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ConfigDir is the default directory path where collector configuration files are stored.
const ConfigDir = "/usr/lib/rhc/collectors/"

// TimerDir is the default directory path where information about collectors execution are stored.
const TimerDir = "/var/cache/rhc/collectors/"

const defaultMetaType = "ingress"
const defaultUser = "root"
const defaultGroup = "root"
const defaultOutputDir = "/var/tmp/rhc/"
const compactTimestamp = "20060102150405.000"
const uploadTimeout = 60 * time.Second

// Config represents the configuration for a collector instance.
type Config struct {
	// ID is the unique identifier for the collector.
	ID string
	// Name is the human-readable name of the collector.
	Name string
	// IsAnalyticsFeature indicates whether collector provides analytics functionality.
	IsAnalyticsFeature bool
	// User specifies the system user under which the collector should run.
	User string
	// Group specifies the system group under which the collector should run.
	Group string
	// ContentType is used by rhc when it uploads the data archive to Ingress.
	ContentType string
}

// configDto represents the structure of a TOML configuration file for parsing.
type configDto struct {
	Meta    *metaDto    `toml:"meta"`
	Ingress *ingressDto `toml:"ingress"`
}

// metaDto represents the metadata section of a TOML configuration file.
type metaDto struct {
	Name    string  `toml:"name"`
	Feature *string `toml:"feature,omitempty"`
	Type    *string `toml:"type"`
}

// ingressDto represents the ingress section of a TOML configuration file.
type ingressDto struct {
	User        *string `toml:"user,omitempty"`
	Group       *string `toml:"group,omitempty"`
	ContentType string  `toml:"content_type"`
}

// ArchiveDto represents an archive file with its path and MIME content type.
type ArchiveDto struct {
	// Path is a path to the archive file.
	Path string
	// ContentType is the MIME type of the archive (e.g., "application/vnd.redhat.advisor.collection").
	ContentType string
}

// ServiceConfig represents the configuration for an upload service endpoint.
type ServiceConfig struct {
	URL           string
	CertPath      string
	ClientKeyPath string
}

// multipartData encapsulates a multipart form buffer and its content type.
type multipartData struct {
	Buffer      *bytes.Buffer
	ContentType string
}

// Timer represents the execution timing information for a collector.
type Timer struct {
	// ID is the unique identifier for the collector.
	ID string
	// LastStarted is the timestamp when the collector was last started.
	LastStarted time.Time
	// LastFinished is the timestamp when the collector last finished execution.
	LastFinished time.Time
	// ExitCode is the exit code from the last execution (0 indicates success).
	ExitCode int
}

// startedEventDto represents a collector start event for JSON serialization.
type startedEventDto struct {
	Timestamp int64 `json:"timestamp"`
}

// finishedEventDto represents a collector finish event for JSON serialization.
type finishedEventDto struct {
	Timestamp int64 `json:"timestamp"`
	ExitCode  int   `json:"exit_code"`
}

// timerDto represents the complete timer data structure for JSON serialization.
type timerDto struct {
	LastStarted  *startedEventDto  `json:"last_started"`
	LastFinished *finishedEventDto `json:"last_finished"`
}

// GetArchive generates an archive filename and creates a compressed archive from the specified directory.
func GetArchive(sourceDir, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	archiveTimestamp := strings.ReplaceAll(time.Now().Format(compactTimestamp), ".", "")
	archiveName := "rhc-collector-" + archiveTimestamp + ".tar.xz"
	archivePath, err := createArchive(archiveName, filepath.Clean(sourceDir), filepath.Clean(outputDir))
	if err != nil {
		return "", err
	}
	return archivePath, nil
}

// GetCollectors returns list of available collectors from valid TOML files in ConfigDir.
func GetCollectors() ([]string, error) {
	configFiles, err := os.ReadDir(ConfigDir)
	if err != nil {
		return nil, err
	}

	var collectors []string
	for _, configFile := range configFiles {
		configName, err := getConfigFilename(configFile)
		if err != nil {
			slog.Warn("Failed to load config", "error", err)
		} else {
			collectorId := strings.TrimSuffix(configName, ".toml")
			if _, err = loadConfigFromFile(collectorId); err != nil {
				slog.Warn("Failed to load config", "file", configName, "error", err)
			} else {
				collectors = append(collectors, collectorId)
			}
		}
	}

	return collectors, nil
}

// GetConfig retrieves a collector configuration by its ID.
func GetConfig(id string) (Config, error) {
	config, err := loadConfigFromFile(id)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

// UploadArchive uploads an archive file to the Red Hat Console ingress service.
func UploadArchive(archive ArchiveDto, config ServiceConfig) error {
	slog.Info("Uploading archive", slog.String("archive", archive.Path), slog.String("url", config.URL))

	formData, err := createMultipartForm(archive)
	if err != nil {
		return err
	}
	tlsConfig, err := loadClientCertificate(config)
	if err != nil {
		return err
	}
	client := getHTTPClient(tlsConfig)
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

// ReadTimerCache loads timer data from the cache for the specified collector ID.
func ReadTimerCache(id string) (*Timer, error) {
	id, err := validateID(id)
	if err != nil {
		return nil, err
	}
	timer, err := loadTimerFromFile(id)
	if err != nil {
		return nil, err
	}
	return &timer, nil
}

// WriteTimerCache saves timer data to the cache for the specified collector ID.
func WriteTimerCache(id string, timer Timer) error {
	id, err := validateID(id)
	if err != nil {
		return err
	}
	dto := timerDto{}
	if !timer.LastStarted.IsZero() {
		dto.LastStarted = &startedEventDto{
			Timestamp: timer.LastStarted.Unix(),
		}
	}
	if !timer.LastFinished.IsZero() {
		dto.LastFinished = &finishedEventDto{
			Timestamp: timer.LastFinished.Unix(),
			ExitCode:  timer.ExitCode,
		}
	}
	return writeTimerToFile(id, dto)
}

// sendUploadRequest executes an HTTP request and validates the response status.
// Returns an error if the request fails or status is not 2xx.
func sendUploadRequest(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("Failed to upload archive", "error", err)
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Debug("Failed to close response body", "error", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("Failed to upload archive", "error", err)
		return err
	}
	slog.Debug("Response body", slog.String("body", string(body)), slog.String("status", resp.Status))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		slog.Debug("Failed to upload archive", "status code", resp.StatusCode, "response", string(body))
		return fmt.Errorf("upload failed with status code: %d", resp.StatusCode)
	}
	return nil
}

// createUploadRequest creates an HTTP POST request for uploading multipart form data.
// Returns an error if request creation fails.
func createUploadRequest(formData multipartData, config ServiceConfig) (*http.Request, error) {
	req, err := http.NewRequest("POST", config.URL, formData.Buffer)
	if err != nil {
		slog.Debug("Failed to create request", "error", err)
		return nil, err
	}
	req.Header.Set("Content-Type", formData.ContentType)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// createMultipartForm creates a multipart form data buffer from an archive file.
// Returns an error if the file cannot be opened or encoded.
func createMultipartForm(archive ArchiveDto) (multipartData, error) {
	buffer := new(bytes.Buffer)
	writer := multipart.NewWriter(buffer)

	archiveHeader := make(textproto.MIMEHeader)
	archiveHeader.Set(
		"Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", filepath.Base(archive.Path)),
	)
	archiveHeader.Set("Content-Type", archive.ContentType)
	archiveField, err := writer.CreatePart(archiveHeader)
	if err != nil {
		slog.Debug("Failed to create archive field", "error", err)
		return multipartData{}, err
	}
	archiveFile, err := os.Open(archive.Path)
	if err != nil {
		slog.Debug("Failed to open archive", "error", err)
		return multipartData{}, err
	}
	defer func() {
		if closeErr := archiveFile.Close(); closeErr != nil {
			slog.Debug("Failed to close archive file", "error", closeErr)
		}
	}()
	if _, err = io.Copy(archiveField, archiveFile); err != nil {
		slog.Debug("Failed to copy archive", "error", err)
		return multipartData{}, err
	}
	if err := writer.Close(); err != nil {
		slog.Debug("Failed to close multipart writer", "error", err)
		return multipartData{}, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return multipartData{
		Buffer:      buffer,
		ContentType: writer.FormDataContentType(),
	}, nil
}

// getHTTPClient returns an HTTP client configured with TLS certificates for secure uploads.
func getHTTPClient(tlsConfig *tls.Config) *http.Client {
	return &http.Client{
		Timeout: uploadTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

// loadClientCertificate loads X.509 client certificates from the provided service configuration.
// Returns an error if files cannot be read or parsed.
func loadClientCertificate(config ServiceConfig) (*tls.Config, error) {
	if _, err := os.Stat(config.CertPath); os.IsNotExist(err) {
		slog.Debug("No TLS certificate found", "error", err)
		return nil, fmt.Errorf("certificate file not found: %w", err)
	}
	cert, err := tls.LoadX509KeyPair(config.CertPath, config.ClientKeyPath)
	if err != nil {
		slog.Debug("Failed to load client certificate", "error", err)
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// validateID validates and sanitizes a collector ID.
// Returns the sanitized ID and an error if validation fails.
func validateID(id string) (string, error) {
	re := regexp.MustCompile(`^[a-z0-9]+\.[a-z0-9]+(\.[a-z0-9]+)*$`)
	if !re.MatchString(id) {
		slog.Debug("Invalid collector ID", "id", id)
		return "", fmt.Errorf("invalid collector ID %q", id)
	}
	return filepath.Base(id), nil
}

// writeTimerToFile marshals a timerDto to JSON and writes it to the cache file.
// Returns an error if JSON marshaling fails or the file cannot be written.
func writeTimerToFile(id string, dto timerDto) error {
	jsonData, err := json.Marshal(dto)
	if err != nil {
		return err
	}
	err = os.MkdirAll(TimerDir, 0755)
	if err != nil {
		slog.Debug("Failed to create timer cache directory", "error", err)
		return err
	}
	err = os.WriteFile(filepath.Join(TimerDir, id+".json"), jsonData, 0644)
	if err != nil {
		slog.Debug("Failed to write timer cache", "id", id, "error", err)
		return err
	}
	return nil
}

// loadTimerFromFile reads timer data from the cache file and converts it to a Timer struct.
// Returns an error if the file cannot be read or the timer data is invalid.
func loadTimerFromFile(id string) (Timer, error) {
	timerData, err := os.ReadFile(filepath.Join(TimerDir, id+".json"))
	if err != nil {
		return Timer{}, err
	}
	timer, err := newTimer(id, timerData)
	return timer, err
}

// newTimer creates a Timer struct from JSON timer data.
// Returns an error if the JSON data cannot be unmarshalled.
func newTimer(id string, timerData []byte) (Timer, error) {
	var t timerDto
	if err := json.Unmarshal(timerData, &t); err != nil {
		return Timer{}, err
	}

	timer := Timer{ID: id}
	if t.LastStarted != nil {
		timer.LastStarted = time.Unix(t.LastStarted.Timestamp, 0)
	}
	if t.LastFinished != nil {
		timer.LastFinished = time.Unix(t.LastFinished.Timestamp, 0)
		timer.ExitCode = t.LastFinished.ExitCode
	}
	return timer, nil
}

// createArchive compresses a directory into an xz-compressed tar archive.
// Returns an error if the tar command fails.
func createArchive(archiveName, sourceDir, outputDir string) (string, error) {
	archivePath := filepath.Join(outputDir, archiveName)
	cmd := exec.Command("tar", "--create", "--xz", "--file", archivePath, "--directory", sourceDir, ".")
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("tar command failed", "output", string(stdoutStderr))
		return "", fmt.Errorf("failed to create archive: %v", err)
	}
	if len(stdoutStderr) > 0 {
		slog.Info("tar command", "output", string(stdoutStderr))
	}
	return archivePath, nil
}

// getConfigFilename returns the filename if the file entry is a valid TOML configuration file.
// Returns an error if the entry is not a regular file with a .toml extension.
func getConfigFilename(configFile os.DirEntry) (string, error) {
	if isFileToml(configFile) {
		return configFile.Name(), nil
	}
	return "", fmt.Errorf("invalid config file %v", filepath.Join(ConfigDir, configFile.Name()))
}

// isFileToml returns true if the file entry is a regular file with a .toml extension.
func isFileToml(file os.DirEntry) bool {
	return !file.IsDir() && strings.HasSuffix(file.Name(), ".toml")
}

// parseConfigFromContent parses TOML content directly from a string into a Config.
func parseConfigFromContent(content string, id string) (Config, error) {
	var c *configDto
	_, err := toml.Decode(content, &c)
	if err != nil {
		return Config{}, err
	}
	return newConfig(id, c)
}

// loadConfigFromFile loads a collector configuration file from the ConfigDir directory.
// Returns an error if the file cannot be decoded.
func loadConfigFromFile(id string) (Config, error) {
	var c *configDto
	_, err := toml.DecodeFile(ConfigDir+id+".toml", &c)
	if err != nil {
		return Config{}, err
	}
	config, err := newConfig(id, c)
	return config, err
}

// newConfig creates a Config instance from a configDto and validates required fields.
// Returns an error if any required field is missing or is invalid.
func newConfig(id string, dto *configDto) (Config, error) {
	if dto.Meta == nil {
		return Config{}, fmt.Errorf("invalid config: meta section is required")
	}
	if dto.Meta.Name == "" {
		return Config{}, fmt.Errorf("invalid config: meta.name is required")
	}
	if dto.Meta.Type == nil || *dto.Meta.Type != defaultMetaType {
		return Config{}, fmt.Errorf("invalid config: meta.type must be '%s'", defaultMetaType)
	}

	if dto.Ingress == nil {
		return Config{}, fmt.Errorf("invalid config: ingress section is required")
	}
	var user string
	if dto.Ingress.User != nil {
		user = *dto.Ingress.User
	} else {
		user = defaultUser
	}
	var group string
	if dto.Ingress.Group != nil {
		group = *dto.Ingress.Group
	} else {
		group = defaultGroup
	}
	if dto.Ingress.ContentType == "" {
		return Config{}, fmt.Errorf("invalid config: ingress.content_type is required")
	}

	// Emit warning if meta.feature is present but not 'analytics'
	if dto.Meta.Feature != nil && *dto.Meta.Feature != "analytics" {
		slog.Warn("Unexpected meta.feature value", "actual", *dto.Meta.Feature, "expected", "analytics")
	}

	return Config{
		ID:                 id,
		Name:               dto.Meta.Name,
		IsAnalyticsFeature: dto.Meta.Feature == nil || *dto.Meta.Feature == "analytics",
		User:               user,
		Group:              group,
		ContentType:        dto.Ingress.ContentType,
	}, nil
}
