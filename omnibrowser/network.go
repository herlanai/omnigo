package omnibrowser

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"strings"
)

type network struct {
	client    *http.Client
	transport *http.Transport
	options   Options
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}
func newNetwork(options Options) *network {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range addresses {
			if !options.AllowPrivateNetwork && !publicAddress(ip) {
				return nil, errors.New("omnibrowser: private network blocked")
			}
		}
		var last error
		for _, ip := range addresses {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		if last == nil {
			last = errors.New("omnibrowser: no resolved addresses")
		}
		return nil, last
	}
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client := &http.Client{Transport: transport, Jar: jar, Timeout: options.Timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("omnibrowser: redirect limit")
		}
		return validateURL(req.URL)
	}}
	return &network{client, transport, options}
}
func validateURL(u *url.URL) error {
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return errors.New("omnibrowser: requires HTTP(S) URL without credentials")
	}
	return nil
}
func (n *network) close() { n.transport.CloseIdleConnections() }
func (n *network) fetch(ctx context.Context, address string, document bool) (string, string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", "", err
	}
	if err = validateURL(u); err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "OmniBrowser/0.1 (Go document engine)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	client := *n.client
	if !document {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host {
				return errors.New("omnibrowser: cross-origin script redirect unsupported")
			}
			return n.client.CheckRedirect(req, via)
		}
	}
	response, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", fmt.Errorf("omnibrowser: HTTP %d", response.StatusCode)
	}
	if ct := response.Header.Get("Content-Type"); document && ct != "" && !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml+xml") {
		return "", "", fmt.Errorf("omnibrowser: unsupported document content type %s", ct)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, n.options.MaxBytes+1))
	if err != nil {
		return "", "", err
	}
	if int64(len(data)) > n.options.MaxBytes {
		return "", "", errors.New("omnibrowser: response byte limit exceeded")
	}
	return string(data), response.Request.URL.String(), nil
}

// Navigate loads an HTTP(S) document with redirects, cookies and cancellation.
// Same-origin classic scripts execute in document order. Styles/images and JS fetch
// are not fetched by this version.
func (p *Page) Navigate(ctx context.Context, address string) error {
	return p.operation(ctx, func(ctx context.Context) error {
		source, final, err := p.client.fetch(ctx, address, true)
		if err != nil {
			return err
		}
		return p.load(ctx, source, final, true)
	})
}
