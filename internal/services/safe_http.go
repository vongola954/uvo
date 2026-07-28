package services

import (
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

func isPrivateIP(host string) bool {
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // fail closed
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// SafeDownload downloads HTTPS audio only from allowlisted hosts.
func SafeDownload(rawURL, destPath string, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 30 << 20 // 30MB
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("bad url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https allowed")
	}
	if !hostAllowed(u.Hostname()) {
		return fmt.Errorf("host not allowlisted: %s", u.Hostname())
	}
	if isPrivateIP(u.Hostname()) {
		return fmt.Errorf("private IP blocked")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(rawURL)
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
