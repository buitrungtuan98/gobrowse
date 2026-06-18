package render

import (
	"testing"

	"github.com/go-chromium-core/gcc"
)

func TestLayoutAndPaint(t *testing.T) {
	// 1. Mock DOM
	dom := &gcc.DOMTree{
		Root: &gcc.DOMNode{
			Type: "div",
			Attr: []map[string]string{{"id": "main"}},
			Children: []*gcc.DOMNode{
				{Type: "text", Data: "Hello Render Engine"},
			},
		},
	}

	// 2. Mock CSS
	css := &gcc.CSSOMTree{
		Rules: []gcc.CSSRule{
			{
				Selector: "#main",
				Styles:   map[string]string{"background-color": "#ff0000", "width": "500px"},
			},
		},
	}

	stack := NewRenderStack()
	layout, err := stack.ComputeLayout(dom, css)
	if err != nil {
		t.Fatalf("ComputeLayout failed: %v", err)
	}

	if layout.W != 500 {
		t.Errorf("Expected width 500 from CSS rule, got %v", layout.W)
	}

	canvas := NewTerminalCanvas()
	err = stack.Paint(layout, canvas)
	if err != nil {
		t.Fatalf("Paint failed: %v", err)
	}

	logs := canvas.GetLogs()
	if len(logs) == 0 {
		t.Fatal("Expected terminal canvas to log draw calls, got none.")
	}
}
