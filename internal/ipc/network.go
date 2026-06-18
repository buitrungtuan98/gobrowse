package ipc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
)

// NetworkIPCAdapter implements gcc.NetworkEngine and forwards calls over gRPC.
type NetworkIPCAdapter struct {
	client api.NetworkServiceClient
}

func NewNetworkIPCAdapter(client api.NetworkServiceClient) *NetworkIPCAdapter {
	return &NetworkIPCAdapter{client: client}
}

func (a *NetworkIPCAdapter) Fetch(ctx context.Context, url string, opt gcc.FetchOptions) (*gcc.Response, error) {
	req := &api.FetchRequest{
		Url:     url,
		Method:  opt.Method,
		Headers: make(map[string]string),
	}

	for k, vals := range opt.Headers {
		if len(vals) > 0 {
			req.Headers[k] = vals[0]
		}
	}

	if opt.Body != nil {
		bodyBytes, _ := io.ReadAll(opt.Body)
		req.Body = bodyBytes
	}

	resp, err := a.client.FetchResource(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ipc fetch failed: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon network error: %s", resp.ErrorMessage)
	}

	header := make(http.Header)
	for k, v := range resp.Headers {
		header.Set(k, v)
	}

	return &gcc.Response{
		StatusCode: int(resp.StatusCode),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(resp.Body)),
	}, nil
}

func (a *NetworkIPCAdapter) SetRoutingMode(mode gcc.RoutingMode) error {
	// Not implemented via RPC in this milestone yet, assume local stack configures this
	return nil
}

func (a *NetworkIPCAdapter) ConfigureDNS(provider gcc.DNSConfig) error {
	// Not implemented via RPC in this milestone yet
	return nil
}
