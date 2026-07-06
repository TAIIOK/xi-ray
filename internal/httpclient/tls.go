package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"
)

var caBundlePaths = []string{
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/ssl/cert.pem",
}

// RootCAs returns a cert pool with system roots plus common OpenWrt bundle paths.
func RootCAs() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	for _, path := range caBundlePaths {
		pem, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pool.AppendCertsFromPEM(pem)
	}
	return pool
}

// Default returns an HTTP client with router-friendly TLS roots and timeout.
func Default(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: RootCAs()},
		},
	}
}
