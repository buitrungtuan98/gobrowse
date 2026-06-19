package render

import (
	"testing"
	"github.com/go-chromium-core/gcc"
)

func TestPseudoClasses(t *testing.T) {
	dom := &gcc.DOMTree{
		Root: &gcc.DOMNode{
			Type: "div",
			Attr: []map[string]string{
				{"id": "btn"},
				{"_hover": "true"}, // simulate hover state
			},
		},
	}

	css := &gcc.CSSOMTree{
		Rules: []gcc.CSSRule{
			{Selector: "#btn", Styles: map[string]string{"color": "blue"}},
			{Selector: "#btn:hover", Styles: map[string]string{"color": "red"}},
			{Selector: "#btn:active", Styles: map[string]string{"color": "green"}},
		},
	}

	layout, err := ComputeLayout(dom, css)
	if err != nil {
		t.Fatalf("ComputeLayout error: %v", err)
	}

	if layout.Styles["color"] != "red" {
		t.Errorf("expected hover color 'red', got '%v'", layout.Styles["color"])
	}

	// Remove hover, add active
	dom.Root.Attr[1]["_hover"] = "false"
	dom.Root.Attr = append(dom.Root.Attr, map[string]string{"_active": "true"})

	layout2, _ := ComputeLayout(dom, css)
	if layout2.Styles["color"] != "green" {
		t.Errorf("expected active color 'green', got '%v'", layout2.Styles["color"])
	}

	// Remove all pseudo
	dom.Root.Attr[2]["_active"] = "false"
	layout3, _ := ComputeLayout(dom, css)
	if layout3.Styles["color"] != "blue" {
		t.Errorf("expected default color 'blue', got '%v'", layout3.Styles["color"])
	}
}
