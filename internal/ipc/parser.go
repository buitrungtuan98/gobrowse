package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
)

// ParserIPCAdapter implements gcc.ParserEngine and forwards calls over gRPC.
type ParserIPCAdapter struct {
	client api.ParserServiceClient
}

func NewParserIPCAdapter(client api.ParserServiceClient) *ParserIPCAdapter {
	return &ParserIPCAdapter{client: client}
}

func (a *ParserIPCAdapter) ParseHTML(r io.Reader) (*gcc.DOMTree, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	req := &api.ParseRequest{
		Payload: buf.Bytes(),
	}

	resp, err := a.client.ParseHTML(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("ipc parse html failed: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon parser error: %s", resp.ErrorMessage)
	}

	var dom gcc.DOMTree
	err = json.Unmarshal([]byte(resp.DomPayload), &dom)
	if err != nil {
		return nil, fmt.Errorf("failed to decode daemon dom payload: %w", err)
	}

	return &dom, nil
}

func (a *ParserIPCAdapter) ParseCSS(r io.Reader) (*gcc.CSSOMTree, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	req := &api.ParseRequest{
		Payload: buf.Bytes(),
	}

	resp, err := a.client.ParseCSS(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("ipc parse css failed: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon parser error: %s", resp.ErrorMessage)
	}

	var cssom gcc.CSSOMTree
	err = json.Unmarshal([]byte(resp.CssomPayload), &cssom)
	if err != nil {
		return nil, fmt.Errorf("failed to decode daemon cssom payload: %w", err)
	}

	return &cssom, nil
}
