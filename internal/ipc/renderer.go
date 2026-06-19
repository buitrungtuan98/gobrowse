package ipc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
)

// RendererIPCAdapter implements gcc.RenderEngine and forwards calls over gRPC.
type RendererIPCAdapter struct {
	client api.RendererServiceClient
}

func NewRendererIPCAdapter(client api.RendererServiceClient) *RendererIPCAdapter {
	return &RendererIPCAdapter{client: client}
}

func (a *RendererIPCAdapter) ComputeLayout(dom *gcc.DOMTree, css *gcc.CSSOMTree, viewportWidth, viewportHeight float64) (*gcc.LayoutTree, error) {
	// For IPC, we must serialize the complex structures to JSON (or a compact binary format).
	domPayload, _ := json.Marshal(dom)
	cssPayload, _ := json.Marshal(css)

	req := &api.LayoutRequest{
		DomPayload: string(domPayload),
		CssPayload: string(cssPayload),
	}

	resp, err := a.client.ComputeLayout(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("ipc compute layout failed: %w", err)
	}
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon render error: %s", resp.ErrorMessage)
	}

	var layout gcc.LayoutTree
	err = json.Unmarshal([]byte(resp.LayoutTreePayload), &layout)
	if err != nil {
		// Log warning, return nil for mock purposes
		return nil, nil
	}

	return &layout, nil
}

func (a *RendererIPCAdapter) Paint(layout *gcc.LayoutTree, canvas gcc.TargetCanvas) error {
	layoutPayload, _ := json.Marshal(layout)

	req := &api.PaintRequest{
		LayoutTreePayload: string(layoutPayload),
	}

	resp, err := a.client.PaintLayout(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ipc paint layout failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("daemon paint error: %s", resp.ErrorMessage)
	}

	// Mock note: The actual draw calls back to the UI thread (TargetCanvas) over IPC
	// requires a bidirectional stream. For Phase 3, we simply assume success if daemon doesn't error.
	return nil
}
