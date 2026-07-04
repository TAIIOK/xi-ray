package update

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	CheckReleaseTimeout   = 30 * time.Second
	DownloadClientTimeout = 15 * time.Minute
)

// CheckHTTPClient is tuned for GitHub API checks on routers (short timeouts, IPv4-first).
func CheckHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Timeout: CheckReleaseTimeout,
		Transport: &http.Transport{
			DialContext:           dialIPv4First(dialer),
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}

func DownloadHTTPClient() *http.Client {
	return &http.Client{Timeout: DownloadClientTimeout}
}

func dialIPv4First(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
		if err == nil && len(ips) > 0 {
			return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ips[0].String(), port))
		}
		return dialer.DialContext(ctx, "tcp", addr)
	}
}
