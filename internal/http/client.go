package httpapi

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Timeouts holds the HTTP timeouts applied to a client, resolved from
// rhc.conf's [http.timeout] section.
type Timeouts struct {
	// Connect bounds establishing the TCP connection.
	Connect time.Duration
	// Request bounds the whole request/response, including the body.
	Request time.Duration
	// Idle bounds how long an idle keep-alive connection is kept open.
	Idle time.Duration
}

// NewHTTPClient returns an HTTP client configured with the given TLS
// configuration and timeouts for secure uploads.
func NewHTTPClient(tlsConfig *tls.Config, timeouts Timeouts) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig.Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   timeouts.Connect,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.IdleConnTimeout = timeouts.Idle
	return &http.Client{
		Timeout:   timeouts.Request,
		Transport: transport,
	}
}
