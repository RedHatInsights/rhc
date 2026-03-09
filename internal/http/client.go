package httpapi

import (
	"crypto/tls"
	"net/http"
	"time"
)

// FIXME: Make uploadTimeout configurable
const uploadTimeout = 60 * time.Second

// NewHTTPClient returns an HTTP client configured with TLS certificates for secure uploads.
func NewHTTPClient(tlsConfig *tls.Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig.Clone()
	return &http.Client{
		Timeout:   uploadTimeout,
		Transport: transport,
	}
}
