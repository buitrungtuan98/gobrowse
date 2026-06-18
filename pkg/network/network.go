package network

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chromium-core/gcc"
	"golang.org/x/net/proxy"
)

// NetworkStack is the concrete implementation of gcc.NetworkEngine.
type NetworkStack struct {
	mode   gcc.RoutingMode
	dnsCfg gcc.DNSConfig
	client *http.Client
}

// NewNetworkStack initializes a default network stack operating in Regular mode.
func NewNetworkStack() *NetworkStack {
	return &NetworkStack{
		mode: gcc.ModeRegular,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     NewEphemeralJar(),
		},
	}
}

// SetRoutingMode updates the network routing context (e.g. ModeRegular vs ModeTor)
// and dynamically rebuilds the underlying HTTP transport to match the required security context.
func (n *NetworkStack) SetRoutingMode(mode gcc.RoutingMode) error {
	n.mode = mode
	return n.rebuildTransport()
}

// ConfigureDNS sets up upstream name resolution and reconfigures the transport if necessary.
func (n *NetworkStack) ConfigureDNS(cfg gcc.DNSConfig) error {
	n.dnsCfg = cfg
	return n.rebuildTransport()
}

// rebuildTransport constructs the http.Transport based on the current mode and DNS config.
func (n *NetworkStack) rebuildTransport() error {
	transport := &http.Transport{}

	if n.mode == gcc.ModeTor {
		if len(n.dnsCfg.Servers) == 0 {
			return fmt.Errorf("tor mode requires at least one proxy server address in DNS config")
		}

		proxyAddr := n.dnsCfg.Servers[0] // e.g. 127.0.0.1:9050

		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return fmt.Errorf("failed type assertion to ContextDialer")
		}

		transport.DialContext = contextDialer.DialContext
	}

	n.client.Transport = transport
	return nil
}

// Fetch securely executes the network request according to the current routing and execution modes.
func (n *NetworkStack) Fetch(ctx context.Context, url string, opt gcc.FetchOptions) (*gcc.Response, error) {
	req, err := http.NewRequestWithContext(ctx, opt.Method, url, opt.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if opt.Headers != nil {
		req.Header = opt.Headers.Clone()
	}

	if opt.CustomUserAgent != "" {
		req.Header.Set("User-Agent", opt.CustomUserAgent)
	} else {
		// Provide default GCC user agent to prevent basic fingerprinting deviations if absent
		req.Header.Set("User-Agent", "GCC-Engine/1.0.0")
	}

	if opt.SkipCache {
		req.Header.Set("Cache-Control", "no-cache")
	}

	if n.mode == gcc.ModeTor {
		// Ensure Tor mode strictly avoids identity leakage
		req.Header.Del("Forwarded")
		req.Header.Del("X-Forwarded-For")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network execution failed: %w", err)
	}

	return &gcc.Response{
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Header:     resp.Header,
		Body:       resp.Body,
	}, nil
}
