package render

import (
	"github.com/go-chromium-core/gcc"
)

// RenderStack implements the gcc.RenderEngine interface.
type RenderStack struct{}

// NewRenderStack creates the layout & painting engine.
func NewRenderStack() *RenderStack {
	return &RenderStack{}
}

// ComputeLayout acts as the facade for calculating bounding geometry and layout trees.
func (r *RenderStack) ComputeLayout(dom *gcc.DOMTree, css *gcc.CSSOMTree) (*gcc.LayoutTree, error) {
	return ComputeLayout(dom, css)
}

// Paint traverses the layout tree and rasterizes elements to the given canvas surface.
func (r *RenderStack) Paint(layout *gcc.LayoutTree, canvas gcc.TargetCanvas) error {
	if layout == nil {
		return nil
	}

	paintNode(layout, canvas)

	return canvas.Flush()
}

func paintNode(layout *gcc.LayoutTree, canvas gcc.TargetCanvas) {
	// Attempt to pull visual properties
	bg := ""
	if val, ok := layout.Styles["background-color"]; ok {
		bg = val
	} else {
		bg = "transparent"
	}

	if layout.Node.Type == "text" {
		canvas.DrawText(layout.X, layout.Y, layout.Node.Data, "sans-serif", 14)
	} else if layout.Node.Type == "img" {
		if imgDataStr, ok := layout.Styles["_img_data"]; ok {
			canvas.DrawImage(layout.X, layout.Y, []byte(imgDataStr))
		} else {
			canvas.DrawRect(layout.X, layout.Y, layout.W, layout.H, bg)
		}
	} else {
		canvas.DrawRect(layout.X, layout.Y, layout.W, layout.H, bg)
	}

	for _, child := range layout.Children {
		paintNode(child, canvas)
	}
}
