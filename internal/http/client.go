package httpapi

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// FIXME: Make uploadTimeout configurable
const uploadTimeout = 60 * time.Second

type Client struct {
	client http.Client
}

// NewHTTPClient returns an HTTP client configured with TLS certificates for secure uploads.
func NewHTTPClient(tlsConfig *tls.Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig.Clone()
	return &http.Client{
		Timeout:   uploadTimeout,
		Transport: transport,
	}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	return c.client.Do(req)
}
