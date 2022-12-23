package http

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

type Client struct {
	client *http.Client
}

func NewHTTPClient(tlsConfig *tls.Config) (*Client, error) {

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &Client{
		client: client,
	}, nil
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	return c.client.Do(req)
}
