package ipc

import (
	"context"
	"fmt"

	"github.com/go-chromium-core/gcc/api"
)

// JavascriptIPCAdapter implements gcc.JSEngine and forwards calls over gRPC.
type JavascriptIPCAdapter struct {
	client api.JavaScriptServiceClient
}

func NewJavascriptIPCAdapter(client api.JavaScriptServiceClient) *JavascriptIPCAdapter {
	return &JavascriptIPCAdapter{client: client}
}

func (a *JavascriptIPCAdapter) ExecuteScript(script string) (interface{}, error) {
	req := &api.ScriptRequest{
		Script: script,
	}

	resp, err := a.client.ExecuteScript(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("ipc execute script failed: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon js error: %s", resp.ErrorMessage)
	}

	// Returning the payload string directly as interface{} for the mock milestone
	return resp.ResultPayload, nil
}

func (a *JavascriptIPCAdapter) BindGlobalAPI(name string, handler interface{}) error {
	// For full IPC, this requires complex reflection and bidirectional streaming.
	// We'll mock the signature for Phase 3.
	req := &api.BindRequest{
		ApiName:               name,
		ImplementationPayload: fmt.Sprintf("%v", handler),
	}

	resp, err := a.client.BindGlobalAPI(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ipc bind global api failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("daemon js bind error: %s", resp.ErrorMessage)
	}

	return nil
}
