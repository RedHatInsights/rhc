package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/briandowns/spinner"
	"io"
	"net/http"
	"net/url"
	"os"
)

type SatellitePingResponse struct {
	Results struct {
		Foreman struct {
			Database struct {
				Active     bool   `json:"active"`
				DurationMs string `json:"duration_ms"`
			} `json:"database"`
			Cache struct {
				Servers []struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"servers"`
			} `json:"cache"`
		} `json:"foreman"`
		Katello struct {
			Services struct {
				Candlepin struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"candlepin"`
				CandlepinAuth struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"candlepin_auth"`
				ForemanTasks struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"foreman_tasks"`
				KatelloEvents struct {
					Status     string `json:"status"`
					Message    string `json:"message"`
					DurationMs string `json:"duration_ms"`
				} `json:"katello_events"`
				CandlepinEvents struct {
					Status     string `json:"status"`
					Message    string `json:"message"`
					DurationMs string `json:"duration_ms"`
				} `json:"candlepin_events"`
				Pulp3 struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"pulp3"`
				Pulp3Content struct {
					Status     string `json:"status"`
					DurationMs string `json:"duration_ms"`
				} `json:"pulp3_content"`
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
func (configureSatelliteResult ConfigureSatelliteResult) Error() string {
	var result string
	switch configureSatelliteResult.format {
	case "json":
		data, err := json.MarshalIndent(configureSatelliteResult, "", "    ")
		if err != nil {
			return err.Error()
		}
		result = string(data)
	case "":
		break
	default:
		result = "error: unsupported document format: " + configureSatelliteResult.format
	}
	return result
}

// pingSatelliteServer tries to ping Satellite server to be sure that user
// tries to reach get and run bootstrap script from Satellite server.
// We use following endpoint that is available for unauthenticated users:
// https://satellite.sat.engineering.redhat.com/apidoc/v2/ping/ping.en.html
// It is not necessary to try to reach this endpoint, but download and run
// some bash script downloaded from the internet. It is risky action. For
// this reason we do this extra step to minimize risk that some non-intentional
// code is downloaded and run as root user.
//
// In the future we could use following endpoint to determine version of
// Satellite (Katello) and
// https://satellite.sat.engineering.redhat.com/apidoc/v2/ping/server_status.en.html
func pingSatelliteServer(satelliteScriptUrl *url.URL, s *spinner.Spinner) error {
	if s != nil {
		s.Suffix = fmt.Sprintf(" Connecting Satellite server: %v", satelliteScriptUrl.Host)
	}

	// Copy URL from struct URL and modify scheme, path and query
	satellitePingUrl := *satelliteScriptUrl
	satellitePingUrl.Scheme = "https"
	satellitePingUrl.Path = "/api/ping"
	satellitePingUrl.RawQuery = ""

	// We have to use insecure HTTPs connection, because most of the customers use
	// self-signed certificates
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tlsConfig
	client := &http.Client{Transport: transport}

	response, err := client.Get(satellitePingUrl.String())
	if err != nil {
		return fmt.Errorf("ping satellite server failed with error: %v", err)
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("ping satellite server failed with status code %d", response.StatusCode)
	}
	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("unable to read body of ping satellite server response: %v", err)
	}

	satellitePingResponse := SatellitePingResponse{}
	err = json.Unmarshal(resBody, &satellitePingResponse)
	if err != nil {
		return fmt.Errorf("unable to parse ping satellite server response: %v", err)
	}

	// In theory, we could check for the status of candlepin and candlepin auth, etc.,
	// but we only try to configure host. We do not care about status of candlepin ATM.
	// if satellitePingResponse.Results.Katello.Services.Candlepin.Status != "ok" {
	//     return fmt.Errorf("wrong status of candlepin: %v",
	//          satellitePingResponse.Results.Katello.Services.Candlepin.Status)
	// }

	return nil
}

// downloadSatelliteScript tries to download script from provided URL to given
// file
func downloadSatelliteScript(scriptFile *os.File, satelliteUrl *url.URL, s *spinner.Spinner) error {
	if s != nil {
		s.Suffix = fmt.Sprintf(" Downloading configuration from %v", satelliteUrl.Host)
	}

	// Try to get script from Satellite server
	response, err := http.Get(satelliteUrl.String())
	if err != nil {
		return fmt.Errorf("could not download file %v : %w", satelliteUrl.String(), err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %v terminated with status: %v", satelliteUrl.String(), response.Status)
	}

	numberBytesWritten, err := io.Copy(scriptFile, response.Body)
	if err != nil {
		return fmt.Errorf("could not response body to file %v: %w", scriptFile.Name(), err)
	}
	if numberBytesWritten == 0 {
		return fmt.Errorf("zero bytes written from response body to file %v", scriptFile.Name())
	}

	return nil
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
