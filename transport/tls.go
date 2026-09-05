package transport

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

var baselineHTTPClient = http.DefaultClient

// DefaultTLSConfig returns the minimum TLS policy used by chess-go clients
// and servers. TLS 1.3 removes legacy protocol versions and cipher-suite
// choices that are not appropriate for a new network service.
func DefaultTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13}
}

// TLSConfigFromEnv loads optional trust and client-certificate files. The
// private key is never read from an environment variable; only its path is.
// CHESS_TLS_CA extends the platform trust store for development or a private
// deployment CA. CHESS_TLS_CLIENT_CERT and CHESS_TLS_CLIENT_KEY enable mTLS.
func TLSConfigFromEnv() (*tls.Config, error) {
	config := DefaultTLSConfig()
	caPath := strings.TrimSpace(os.Getenv("CHESS_TLS_CA"))
	if caPath != "" {
		data, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read CHESS_TLS_CA %q: %w", caPath, err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return nil, errors.New("CHESS_TLS_CA does not contain a PEM certificate")
		}
		config.RootCAs = roots
	}
	certPath := strings.TrimSpace(os.Getenv("CHESS_TLS_CLIENT_CERT"))
	keyPath := strings.TrimSpace(os.Getenv("CHESS_TLS_CLIENT_KEY"))
	if (certPath == "") != (keyPath == "") {
		return nil, errors.New("CHESS_TLS_CLIENT_CERT and CHESS_TLS_CLIENT_KEY must be provided together")
	}
	if certPath != "" {
		certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}

func newHTTPClient(config *tls.Config) *http.Client {
	// Preserve the standard injection seam used by callers and tests that
	// replace http.DefaultClient. The normal process-wide default gets an
	// isolated TLS policy below, so changing it cannot weaken this package's
	// default transport.
	if http.DefaultClient != baselineHTTPClient {
		return http.DefaultClient
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: config}}
	}
	transport := base.Clone()
	transport.TLSClientConfig = config
	return &http.Client{Transport: transport}
}

func newClientWithTLS(baseURL, token string, config *tls.Config) (*Client, error) {
	client, err := newClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	client.HTTPClient = newHTTPClient(config)
	return client, nil
}
