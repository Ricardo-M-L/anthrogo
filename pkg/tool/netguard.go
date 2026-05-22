package tool

// NetGuard provides SSRF protection by blocking requests to loopback, link-local
// (cloud metadata endpoints such as 169.254.169.254), private RFC-1918, multicast,
// and unspecified address ranges.
//
// Production deployments MUST leave all Allow* flags false (the default).
//
// The following environment variables are available for development / CI ONLY:
//
//	ANTHROGO_NETGUARD_ALLOW_LOOPBACK=1  — allow 127.x / ::1
//	ANTHROGO_NETGUARD_ALLOW_PRIVATE=1   — allow 10/8, 172.16/12, 192.168/16
//	ANTHROGO_NETGUARD_ALLOW_LINKLOCAL=1 — allow 169.254/16, fe80::/10
//
// Setting any of these in production is a security violation.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// NetGuard holds the SSRF policy for outbound HTTP requests.
type NetGuard struct {
	// AllowLoopback permits requests to 127.0.0.0/8 and ::1.
	AllowLoopback bool
	// AllowPrivate permits requests to RFC-1918 ranges (10/8, 172.16/12, 192.168/16).
	AllowPrivate bool
	// AllowLinkLocal permits requests to 169.254.0.0/16 and fe80::/10.
	// This covers AWS/GCP/Azure cloud metadata endpoints.
	AllowLinkLocal bool
}

// DefaultNetGuard returns the production policy: all ranges blocked.
// Env overrides (ANTHROGO_NETGUARD_ALLOW_*) are read at call time.
func DefaultNetGuard() *NetGuard {
	return &NetGuard{
		AllowLoopback:  os.Getenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK") == "1",
		AllowPrivate:   os.Getenv("ANTHROGO_NETGUARD_ALLOW_PRIVATE") == "1",
		AllowLinkLocal: os.Getenv("ANTHROGO_NETGUARD_ALLOW_LINKLOCAL") == "1",
	}
}

// CheckURL validates rawURL against the SSRF policy.
// It rejects non-http/https schemes, then resolves all IPs for the host and
// checks each against the configured deny list.
// Callers should use HTTPClient (which installs a Dialer Control hook) to
// defend against DNS-rebinding TOCTOU as well.
func (g *NetGuard) CheckURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("netguard: invalid URL: %w", err)
	}
	scheme := u.Scheme
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("netguard: scheme %q not allowed (only http/https)", scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("netguard: empty host in URL")
	}

	// If the host is already an IP literal, check it directly without DNS.
	if ip := net.ParseIP(hostname); ip != nil {
		return g.checkIP(ip)
	}

	// Resolve hostname to all IPs and check each.
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("netguard: DNS lookup failed for %q: %w", hostname, err)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if err := g.checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// checkIP returns an error if the IP falls into a denied range.
func (g *NetGuard) checkIP(ip net.IP) error {
	if ip.IsUnspecified() {
		return fmt.Errorf("netguard: request to unspecified address %s is blocked", ip)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("netguard: request to multicast address %s is blocked", ip)
	}
	if !g.AllowLoopback && ip.IsLoopback() {
		return fmt.Errorf("netguard: request to loopback address %s is blocked (SSRF protection)", ip)
	}
	if !g.AllowLinkLocal && ip.IsLinkLocalUnicast() {
		return fmt.Errorf("netguard: request to link-local address %s is blocked (cloud metadata SSRF protection)", ip)
	}
	if !g.AllowPrivate && ip.IsPrivate() {
		return fmt.Errorf("netguard: request to private address %s is blocked (SSRF protection)", ip)
	}
	return nil
}

// HTTPClient returns a new *http.Client that enforces the NetGuard policy via a
// Dialer Control hook. This provides defense-in-depth against DNS-rebinding: even
// if CheckURL passed at pre-flight, the dialer re-checks the actual resolved IP
// right before connection.
//
// If base is nil, a fresh client is created. If base is non-nil and its Transport
// is a *http.Transport, TLSClientConfig/TLSHandshakeTimeout/MaxIdleConns/etc. are
// inherited; otherwise the transport is replaced.
func (g *NetGuard) HTTPClient(base *http.Client) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	dialCtx := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("netguard: bad dial address %q: %w", address, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("netguard: dial to non-IP address %q (unexpected after DNS resolution)", host)
		}
		if err := g.checkIP(ip); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	var transport *http.Transport
	if base != nil {
		if t, ok := base.Transport.(*http.Transport); ok && t != nil {
			// Clone inherits TLS settings, idle conns, etc.
			transport = t.Clone()
		}
	}
	if transport == nil {
		transport = &http.Transport{
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	transport.DialContext = dialCtx

	var client http.Client
	if base != nil {
		client = *base // shallow copy preserves Timeout, Jar, CheckRedirect
	}
	client.Transport = transport
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	return &client
}
