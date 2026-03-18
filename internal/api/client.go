package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type UserAgent struct {
	identifier string
	caller     *string
	os         string
}

func NewUserAgent(programName, programVersion string) UserAgent {
	ua := UserAgent{
		identifier: fmt.Sprint(programName, "/", programVersion),
		// TODO Read from /usr/lib/os-release
		os: "Red Hat Enterprise Linux/10.2",
	}
	return ua
}

func (ua *UserAgent) WithCaller(callerName, callerVersion string) UserAgent {
	value := fmt.Sprint(callerName, "/", callerVersion)
	ua.caller = &value
	return *ua
}

func (ua *UserAgent) String() string {
	result := ua.identifier
	if ua.caller != nil {
		result += " " + *ua.caller
	}
	result += " " + ua.os
	return result
}

// Client wraps an HTTP client with automatic header injection.
type Client struct {
	http          *http.Client
	headerSetters []func(*http.Request)
}

func NewClient(callables ...NewClientOption) *Client {
	options := newClientOptions()
	for _, apply := range callables {
		apply(&options)
	}
	return &Client{
		http: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: options.tlsConfig,
			},
			Timeout: options.timeout,
		},
		headerSetters: options.headerSetters,
	}
}

// Do executes an HTTP request after applying configured headers.
// Request-ID is automatically generated for each request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Apply configured headers (User-Agent, custom Correlation-ID, etc.)
	for _, setter := range c.headerSetters {
		setter(req)
	}

	// Set connection tracking headers
	requestID := uuid.Must(uuid.NewV7()).String()
	req.Header.Set("Request-ID", requestID)
	if req.Header.Get("Correlation-ID") == "" {
		req.Header.Set("Correlation-ID", requestID)
	}

	return c.http.Do(req)
}

type clientOptions struct {
	tlsConfig     *tls.Config
	timeout       time.Duration
	headerSetters []func(*http.Request)
}

func newClientOptions() clientOptions {
	return clientOptions{
		tlsConfig: &tls.Config{
			RootCAs: loadSystemCertPool(),
		},
	}
}

// NewClientOption is a callable passed into NewClient to construct a http.Client.
type NewClientOption func(*clientOptions)

// loadSystemCertPool loads the system certificate pool for TLS verification.
// Returns an empty pool if the system pool cannot be loaded.
func loadSystemCertPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	return pool
}

func WithMutualTLS(certPath, keyPath string) NewClientOption {
	return func(opts *clientOptions) {
		// FIXME Do not ignore errors
		cert, _ := tls.LoadX509KeyPair(certPath, keyPath)
		opts.tlsConfig.Certificates = []tls.Certificate{cert}
	}
}

func WithTimeout(timeout time.Duration) NewClientOption {
	return func(opts *clientOptions) {
		opts.timeout = timeout
	}
}

func WithCorrelationID(value string) NewClientOption {
	return func(opts *clientOptions) {
		opts.headerSetters = append(opts.headerSetters, func(req *http.Request) {
			req.Header.Set("Correlation-ID", value)
		})
	}
}

func WithCustomCA(certPath string) NewClientOption {
	return func(opts *clientOptions) {
		// FIXME Do not ignore errors
		cert, _ := os.ReadFile(certPath)
		opts.tlsConfig.RootCAs = x509.NewCertPool()
		opts.tlsConfig.RootCAs.AppendCertsFromPEM(cert)
	}
}

func WithUserAgent(ua UserAgent) NewClientOption {
	return func(opts *clientOptions) {
		opts.headerSetters = append(opts.headerSetters, func(req *http.Request) {
			req.Header.Set("User-Agent", ua.String())
		})
	}
}
