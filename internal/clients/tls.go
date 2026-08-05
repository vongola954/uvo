package clients

import (
	"crypto/tls"
	"crypto/x509"
	"embed"
	"net/http"
	"sync"
	"time"
)

// Russian Trusted Root/Sub CA (Минцифры) — MAX / many RU APIs use this chain.
// Alpine/system bundles often omit it → x509: certificate signed by unknown authority.
//
//go:embed certs/*.pem
var russianTrustedCerts embed.FS

var (
	tlsOnce   sync.Once
	rootCAs   *x509.CertPool
	tlsInitEr error
)

func russianTrustedPool() (*x509.CertPool, error) {
	tlsOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		entries, err := russianTrustedCerts.ReadDir("certs")
		if err != nil {
			tlsInitEr = err
			rootCAs = pool
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			b, err := russianTrustedCerts.ReadFile("certs/" + e.Name())
			if err != nil {
				continue
			}
			pool.AppendCertsFromPEM(b)
		}
		rootCAs = pool
	})
	return rootCAs, tlsInitEr
}

// HTTPClient returns an HTTP client that trusts system CAs plus Russian Trusted Root.
func HTTPClient(timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if pool, err := russianTrustedPool(); err == nil && pool != nil {
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}
