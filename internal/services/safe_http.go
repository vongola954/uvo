package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var allowedHostSuffixes = []string{
	"suno.ai",
	"suno.com",
	"acedata.cloud",
	"cdn1.suno.ai",
	"cdn2.suno.ai",
	"hedra.com",
	"together.ai",
	"klingai.com",
	"googleapis.com",
}

func hostAllowed(host string) bool {
	host = strings.ToLower(host)
	for _, s := range allowedHostSuffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	// env extra: DOWNLOAD_ALLOW_HOSTS=a.com,b.com
	for _, extra := range strings.Split(os.Getenv("DOWNLOAD_ALLOW_HOSTS"), ",") {
		extra = strings.ToLower(strings.TrimSpace(extra))
		if extra != "" && (host == extra || strings.HasSuffix(host, "."+extra)) {
			return true
		}
	}
	return false
}

func ipBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || !ip.IsGlobalUnicast()
}

func isPrivateIP(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ipBlocked(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // fail closed
	}
	for _, ip := range ips {
		if ipBlocked(ip) {
			return true
		}
	}
	return false
}

// assertSafeURL rejects non-HTTPS, non-allowlisted, or private/link-local targets.
func assertSafeURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("bad url")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https allowed")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}
	if !hostAllowed(host) {
		return fmt.Errorf("host not allowlisted: %s", host)
	}
	if isPrivateIP(host) {
		return fmt.Errorf("private IP blocked")
	}
	return nil
}

// checkDownloadRedirect re-validates every redirect hop (SSRF via allowlisted CDN → 302).
func checkDownloadRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("too many redirects")
	}
	return assertSafeURL(req.URL)
}

// dialContextRejectPrivate resolves at dial time and refuses RFC1918 / link-local / loopback
// (DNS rebinding between validate and connect).
func dialContextRejectPrivate(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var candidates []net.IP
	if ip := net.ParseIP(host); ip != nil {
		candidates = []net.IP{ip}
	} else {
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("private IP blocked")
		}
		for _, a := range addrs {
			candidates = append(candidates, a.IP)
		}
	}
	var d net.Dialer
	var last error
	for _, ip := range candidates {
		if ipBlocked(ip) {
			last = fmt.Errorf("private IP blocked")
			continue
		}
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("private IP blocked")
	}
	return nil, last
}

func downloadHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = dialContextRejectPrivate
	return &http.Client{
		Timeout:       120 * time.Second,
		Transport:     tr,
		CheckRedirect: checkDownloadRedirect,
	}
}

// SafeDownload downloads HTTPS audio only from allowlisted hosts.
// Redirects are re-validated; dial rejects private IPs (SSRF harden).
func SafeDownload(rawURL, destPath string, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 30 << 20 // 30MB
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("bad url: %w", err)
	}
	if err := assertSafeURL(u); err != nil {
		return err
	}

	resp, err := downloadHTTPClient().Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(resp.Body, maxBytes))
	return err
}
