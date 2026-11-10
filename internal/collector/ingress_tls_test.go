package collector

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSelfSignedPair writes a self-signed certificate and its private key as
// PEM files in dir and returns their paths. The certificate is usable both as a
// TLS client identity and as a CA for these tests.
func writeSelfSignedPair(t *testing.T, dir, name string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPath = filepath.Join(dir, name+"-cert.pem")
	keyPath = filepath.Join(dir, name+"-key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

// loadTestCert builds a tls.Certificate from a freshly generated self-signed
// pair in dir, standing in for the client identity in buildTLSConfig tests.
func loadTestCert(t *testing.T, dir string) tls.Certificate {
	t.Helper()
	certPath, keyPath := writeSelfSignedPair(t, dir, "client")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
	return cert
}

func TestBuildTLSConfigTLSVerify(t *testing.T) {
	cert := loadTestCert(t, t.TempDir())

	for _, tc := range []struct {
		name         string
		tlsVerify    bool
		wantInsecure bool
	}{
		{name: "verify enabled", tlsVerify: true, wantInsecure: false},
		{name: "verify disabled", tlsVerify: false, wantInsecure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := buildTLSConfig(cert, ServiceConfig{TLSVerify: tc.tlsVerify})
			if err != nil {
				t.Fatalf("buildTLSConfig() error = %v", err)
			}
			if cfg.InsecureSkipVerify != tc.wantInsecure {
				t.Errorf("InsecureSkipVerify = %v, want %v", cfg.InsecureSkipVerify, tc.wantInsecure)
			}
			if len(cfg.Certificates) != 1 {
				t.Errorf("Certificates = %d, want 1", len(cfg.Certificates))
			}
			if cfg.RootCAs == nil {
				t.Error("RootCAs = nil, want the system pool")
			}
		})
	}
}

func TestBuildTLSConfigSystemCA(t *testing.T) {
	cert := loadTestCert(t, t.TempDir())

	for _, caPath := range []string{"", systemCAPath} {
		cfg, err := buildTLSConfig(cert, ServiceConfig{CACertPath: caPath, TLSVerify: true})
		if err != nil {
			t.Fatalf("buildTLSConfig(ca=%q) error = %v", caPath, err)
		}
		if cfg.RootCAs == nil {
			t.Errorf("buildTLSConfig(ca=%q): RootCAs = nil", caPath)
		}
	}
}

func TestBuildTLSConfigExplicitCAFile(t *testing.T) {
	dir := t.TempDir()
	cert := loadTestCert(t, dir)
	caPath, _ := writeSelfSignedPair(t, dir, "ca")

	cfg, err := buildTLSConfig(cert, ServiceConfig{CACertPath: caPath, TLSVerify: true})
	if err != nil {
		t.Fatalf("buildTLSConfig() error = %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs = nil, want a pool including the explicit CA")
	}
}

func TestBuildTLSConfigInvalidCAPath(t *testing.T) {
	cert := loadTestCert(t, t.TempDir())

	_, err := buildTLSConfig(cert, ServiceConfig{
		CACertPath: filepath.Join(t.TempDir(), "missing.pem"),
		TLSVerify:  true,
	})
	if err == nil {
		t.Error("buildTLSConfig(missing ca) error = nil, want error")
	}
}

func TestAddCACerts(t *testing.T) {
	dir := t.TempDir()
	caPath, _ := writeSelfSignedPair(t, dir, "ca")

	t.Run("file", func(t *testing.T) {
		pool := x509.NewCertPool()
		if err := addCACerts(pool, caPath); err != nil {
			t.Fatalf("addCACerts(file) error = %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		// A CA directory holds only certificate PEMs (e.g. /etc/rhsm/ca).
		caDir := t.TempDir()
		certPEM, err := os.ReadFile(caPath)
		if err != nil {
			t.Fatalf("read ca cert: %v", err)
		}
		if err := os.WriteFile(filepath.Join(caDir, "root.pem"), certPEM, 0600); err != nil {
			t.Fatalf("write ca cert: %v", err)
		}
		pool := x509.NewCertPool()
		if err := addCACerts(pool, caDir); err != nil {
			t.Fatalf("addCACerts(dir) error = %v", err)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		pool := x509.NewCertPool()
		if err := addCACerts(pool, filepath.Join(dir, "does-not-exist.pem")); err == nil {
			t.Error("addCACerts(missing) error = nil, want error")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		pool := x509.NewCertPool()
		if err := addCACerts(pool, t.TempDir()); err == nil {
			t.Error("addCACerts(empty dir) error = nil, want error")
		}
	})

	t.Run("invalid pem", func(t *testing.T) {
		bad := filepath.Join(dir, "bad.pem")
		if err := os.WriteFile(bad, []byte("not a certificate"), 0600); err != nil {
			t.Fatalf("write bad pem: %v", err)
		}
		pool := x509.NewCertPool()
		if err := addCACerts(pool, bad); err == nil {
			t.Error("addCACerts(invalid pem) error = nil, want error")
		}
	})
}
