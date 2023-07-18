package http

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

type Client struct {
	client http.Client
}

func NewHTTPClient(tlsConfig *tls.Config) *Client {

	// Create a httpClient with the configured tlsConfig.
	// Use the DefaultTransport, as it has some configuration by default.
	client := http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = tlsConfig.Clone()

	return &Client{
		client: client,
	}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	return c.client.Do(req)
}
