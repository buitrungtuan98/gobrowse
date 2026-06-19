package render

import (
	"testing"
	"github.com/go-chromium-core/gcc"
)

func TestMediaQueryLayout(t *testing.T) {
	dom := &gcc.DOMTree{
		Root: &gcc.DOMNode{
			Type: "div",
			Attr: []map[string]string{
				{"id": "box"},
			},
		},
	}

	css := &gcc.CSSOMTree{
		Rules: []gcc.CSSRule{
			{Selector: "#box", Styles: map[string]string{"color": "red"}},
			{Selector: "#box", Styles: map[string]string{"color": "blue"}, MediaQuery: "(max-width: 600px)"},
		},
	}

	// Test 1: Desktop layout (800px width defaults in ComputeLayout)
	layout, err := ComputeLayout(dom, css, 800, 600)
	if err != nil {
		t.Fatalf("ComputeLayout error: %v", err)
	}
	if layout.Styles["color"] != "red" {
		t.Errorf("expected desktop color 'red', got '%v'", layout.Styles["color"])
	}

	// Test 2: Force narrow width using parentStyles helper structure
	// We'll call computeNode manually to inject a smaller availableWidth
	narrowLayout := computeNode(dom.Root, css, 0, 0, 400.0, nil)
	if narrowLayout.Styles["color"] != "blue" {
		t.Errorf("expected mobile color 'blue', got '%v'", narrowLayout.Styles["color"])
	}
}
