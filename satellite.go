package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// SatellitePingResponse is used for un-marshaling JSON document
// returned from Satellite server
type SatellitePingResponse struct {
	Results struct {
		Katello struct {
			Services struct {
				Candlepin struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"candlepin"`
			} `json:"services"`
			Status string `json:"status"`
		} `json:"katello"`
	} `json:"results"`
}

// ConfigureSatelliteResult is structure holding information about results
// of configuring host from Satellite server. The result could be printed
// in machine-readable format.
type ConfigureSatelliteResult struct {
	Hostname                 string `json:"hostname"`
	HostnameError            string `json:"hostname_error,omitempty"`
	IsServerSatellite        bool   `json:"is_server_satellite_running"`
	SatelliteServerHostname  string `json:"satellite_server_hostname"`
	SatelliteServerScriptUrl string `json:"satellite_server_script_url"`
	HostConfigured           bool   `json:"host_configured"`
	format                   string
}

// Error implement error interface for structure ConfigureSatelliteResult
func (result ConfigureSatelliteResult) Error() string {
	var msg string
	switch result.format {
	case "json":
		data, err := json.MarshalIndent(result, "", "    ")
		if err != nil {
			return err.Error()
		}
		msg = string(data)
	case "":
		break
	default:
		msg = "error: unsupported document format: " + result.format
	}
	return msg
}

// SatelliteHTTPClient represents http httpClient used for communication with
// satellite server
type SatelliteHTTPClient struct {
	httpClient   *http.Client
	satelliteURL *url.URL
}

// NewSatelliteClient creates instance of SatelliteHTTPClient and
// configure it to use HTTPS
func NewSatelliteClient(satelliteURL *url.URL) *SatelliteHTTPClient {
	satClient := SatelliteHTTPClient{}
	// We have to use insecure HTTPs connection, because most of the customers use
	// self-signed certificates
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tlsConfig
	satClient.httpClient = &http.Client{Transport: transport}
	satClient.satelliteURL = satelliteURL
	return &satClient
}

// Ping tries to ping Satellite server to be sure that user
// tries to reach get and run bootstrap script from Satellite server.
// We use following endpoint that is available for unauthenticated users:
// https://satellite.sat.engineering.redhat.com/apidoc/v2/ping/ping.en.html
// It is not necessary to try to reach this endpoint, but download and run
// some bash script downloaded from the internet. It is risky action. For
// this reason we do this extra step to minimize risk that some non-intentional
// code is downloaded and run as root user.
//
// In the future we could use following endpoint to determine version of
// Satellite (Katello) and change rhc behavior according detected Satellite version
// https://satellite.sat.engineering.redhat.com/apidoc/v2/ping/server_status.en.html
func (client *SatelliteHTTPClient) Ping() (*SatellitePingResponse, error) {
	// Copy URL from struct URL and modify scheme, path and query
	satellitePingUrl := *client.satelliteURL
	satellitePingUrl.Scheme = "https"
	satellitePingUrl.Path = "/api/ping"
	satellitePingUrl.RawQuery = ""

	response, err := client.httpClient.Get(satellitePingUrl.String())
	if err != nil {
		return nil, fmt.Errorf("ping satellite server failed with error: %v", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("ping satellite server failed with status code %d", response.StatusCode)
	}
	defer func() {
		// TODO: If error happens, then log this error
		_ = response.Body.Close()
	}()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read body of ping satellite server response: %v", err)
	}

	satellitePingResponse := SatellitePingResponse{}
	err = json.Unmarshal(resBody, &satellitePingResponse)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ping satellite server response: %v", err)
	}

	return &satellitePingResponse, nil
}

// downloadSatelliteScript tries to download script from provided URL to given file
func (client *SatelliteHTTPClient) downloadScript(ctx *cli.Context) (*string, error) {
	var scriptFile *os.File
	// Create file for script file
	satelliteScriptPath := filepath.Join(VarLibDir, "katello-rhsm-consumer")

	scriptFile, err := os.Create(satelliteScriptPath)
	if err != nil {
		return nil, cli.Exit(fmt.Errorf("could not create %v file: %w", satelliteScriptPath, err), 1)
	}
	defer func() {
		// TODO: If error happens, then log this error
		_ = scriptFile.Close()
	}()
	err = os.Chmod(satelliteScriptPath, 0700)
	if err != nil {
		return nil, cli.Exit(fmt.Errorf("could not set permissions on %v file: %w", satelliteScriptPath, err), 1)
	}

	// Try to get script from Satellite server
	response, err := http.Get(client.satelliteURL.String())
	if err != nil {
		return nil, fmt.Errorf("could not download file %v : %w", client.satelliteURL.String(), err)
	}

	defer func() {
		// TODO: If error happens, then log this error
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"downloading %v terminated with status: %v",
			client.satelliteURL.String(),
			response.Status,
		)
	}

	numberBytesWritten, err := io.Copy(scriptFile, response.Body)
	if err != nil {
		return nil, fmt.Errorf("could not write response body to file %v: %w", scriptFile.Name(), err)
	}
	if numberBytesWritten == 0 {
		return nil, fmt.Errorf("zero bytes written from response body to file %v", scriptFile.Name())
	}

	// In theory, we could check for the status of candlepin and candlepin auth, etc.,
	// but we only try to configure host. We do not care about status of candlepin ATM.
	// if satellitePingResponse.Results.Katello.Services.Candlepin.Status != "ok" {
	//     return fmt.Errorf("wrong status of candlepin: %v",
	//          satellitePingResponse.Results.Katello.Services.Candlepin.Status)
	// }

	return &satelliteScriptPath, nil
}

// normalizeSatelliteScriptUrl normalize URL of bootstrap script, and it returns
// URL structure, when parsing of URL is successful
func normalizeSatelliteScriptUrl(satelliteUrlStr string) (*url.URL, error) {
	satelliteUrl, err := url.Parse(satelliteUrlStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse satellite url: %w", err)
	}

	// It would be better to use "https", but the most of the customers use
	// self-signed certificates.
	if satelliteUrl.Scheme == "" {
		satelliteUrl.Scheme = "http"
	}

	// When path is not set the path to standard path to bootstrap script
	if satelliteUrl.Path == "" || satelliteUrl.Path == "/" {
		satelliteUrl.Path = "/pub/katello-rhsm-consumer"
	} else {
		// When CLI argument is provided like this --url satellite.company.com,
		// then url.Parse parses hostname as a path
		if satelliteUrl.Path == satelliteUrlStr {
			satelliteUrl.Host = satelliteUrlStr
			satelliteUrl.Path = "/pub/katello-rhsm-consumer"
		}
	}

	return satelliteUrl, nil
}
