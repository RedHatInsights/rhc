package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

const (
	cliLogLevel  = "log-level"
	cliCertFile  = "cert-file"
	cliKeyFile   = "key-file"
	cliAPIServer = "base-url"
)

type Conf struct {
	CertFile string
	KeyFile  string
	LogLevel string
	CADir    string
}

// Create a TLSConfig using the rhsm certificates
func (conf *Conf) CreateTLSClientConfig() (*tls.Config, error) {
	var certData, keyData []byte
	var err error
	rootCAs := make([][]byte, 0)

	KeyDir := conf.KeyFile
	CertDir := conf.CertFile

	certData, err = os.ReadFile(CertDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read cert-file: %w", err)
	}

	keyData, err = os.ReadFile(KeyDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read key-file: %w", err)
	}
	CAFiles, err := os.ReadDir(conf.CADir)
	if err != nil {
		return nil, fmt.Errorf("cannot read ca files: %w", err)
	}
	for _, file := range CAFiles {
		fPath := filepath.Join(conf.CADir, file.Name())
		data, err := os.ReadFile(fPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read ca-file %s : %w", fPath, err)
		}
		rootCAs = append(rootCAs, data)
	}

	tlsConfig := &tls.Config{}
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, fmt.Errorf("cannot create key pair: %w", err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	// Create a pool with CAcerts from rhsm CA directory
	pool := x509.NewCertPool()
	for _, data := range rootCAs {
		pool.AppendCertsFromPEM(data)
	}

	return tlsConfig, nil
}
