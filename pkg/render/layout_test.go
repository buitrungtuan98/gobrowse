package render

import (
	"testing"
	"github.com/go-chromium-core/gcc"
)

func TestGridBasics(t *testing.T) {
	dom := &gcc.DOMTree{
		Root: &gcc.DOMNode{
			Type: "div",
			Attr: []map[string]string{
				{"style": "display: grid; grid-template-columns: 100px 100px; grid-template-rows: 50px 50px; gap: 10px; width: 300px;"},
			},
			Children: []*gcc.DOMNode{
				{Type: "div", Attr: []map[string]string{{"style": "grid-column: 1 / 3; width: auto; height: auto;"}}}, // span 2 columns
				{Type: "div", Attr: []map[string]string{{"style": "width: auto; height: auto;"}}}, // row 2, col 1
				{Type: "div", Attr: []map[string]string{{"style": "width: auto; height: auto;"}}}, // row 2, col 2
			},
		},
	}

	css := &gcc.CSSOMTree{}

	layout, err := ComputeLayout(dom, css, 800, 600)
	if err != nil {
		t.Fatalf("ComputeLayout error: %v", err)
	}

	if layout.W != 300 {
		t.Errorf("expected width 300, got %v", layout.W)
	}

	if len(layout.Children) != 3 {
		t.Fatalf("expected 3 children, got %v", len(layout.Children))
	}

	c1 := layout.Children[0]
	if c1.X != 0 || c1.Y != 0 {
		t.Errorf("child 1 expected at 0,0 got %v,%v", c1.X, c1.Y)
	}
	if c1.W != 210 { // 100 + 10 + 100
		t.Errorf("child 1 expected width 210, got %v", c1.W)
	}
	if c1.H != 50 {
		t.Errorf("child 1 expected height 50, got %v", c1.H)
	}

	c2 := layout.Children[1]
	if c2.X != 0 || c2.Y != 60 { // row 2 start = 50 + 10
		t.Errorf("child 2 expected at 0,60 got %v,%v", c2.X, c2.Y)
	}
	if c2.W != 100 {
		t.Errorf("child 2 expected width 100, got %v", c2.W)
	}
	if c2.H != 50 {
		t.Errorf("child 2 expected height 50, got %v", c2.H)
	}

	c3 := layout.Children[2]
	if c3.X != 110 || c3.Y != 60 { // row 2 col 2 = 100 + 10, 50 + 10
		t.Errorf("child 3 expected at 110,60 got %v,%v", c3.X, c3.Y)
	}
	if c3.W != 100 {
		t.Errorf("child 3 expected width 100, got %v", c3.W)
	}
	if c3.H != 50 {
		t.Errorf("child 3 expected height 50, got %v", c3.H)
	}
}
