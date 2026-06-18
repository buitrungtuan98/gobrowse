package render

import (
	"strconv"
	"strings"

	"github.com/go-chromium-core/gcc"
)

// ComputeLayout walks the DOM tree and applies styles from the CSSOM tree to build a LayoutTree.
func ComputeLayout(dom *gcc.DOMTree, css *gcc.CSSOMTree) (*gcc.LayoutTree, error) {
	if dom == nil || dom.Root == nil {
		return nil, nil
	}

	// Initialize layout with standard desktop width
	defaultWidth := 800.0
	rootLayout := computeNode(dom.Root, css, 0, 0, defaultWidth, nil)
	return rootLayout, nil
}

// computeNode recursively builds the layout structure.

// Inheritable properties
var inheritable = map[string]bool{
	"color":       true,
	"font-family": true,
	"font-size":   true,
}

// computeNode recursively builds the layout structure using standard W3C flow basics.
func computeNode(domNode *gcc.DOMNode, css *gcc.CSSOMTree, currentX, currentY float64, availableWidth float64, parentStyles map[string]string) *gcc.LayoutTree {
	if domNode == nil {
		return nil
	}

	layout := &gcc.LayoutTree{
		Node:   domNode,
		X:      currentX,
		Y:      currentY,
		Styles: make(map[string]string),
	}

	// 1. CSS Cascade & Inheritance
	// Inherit specific styles from parent
	if parentStyles != nil {
		for k, v := range parentStyles {
			if inheritable[k] {
				layout.Styles[k] = v
			}
		}
	}

	// Apply matched styles from CSSOM
	displayType := "block" // Default display mode
	if domNode.Type == "span" || domNode.Type == "a" || domNode.Type == "text" {
		displayType = "inline"
	}
	layout.Styles["display"] = displayType

	if css != nil {
		// Very basic cascade: Element -> Class -> ID
		applyRule := func(rule gcc.CSSRule) {
			for k, v := range rule.Styles {
				layout.Styles[k] = v
			}
		}

		// 1. Tag matching
		for _, rule := range css.Rules {
			if rule.Selector == domNode.Type {
				applyRule(rule)
			}
		}

		// 2. Class matching
		for _, attrMap := range domNode.Attr {
			if classes, exists := attrMap["class"]; exists {
				classArray := strings.Split(classes, " ")
				for _, class := range classArray {
					for _, rule := range css.Rules {
						if rule.Selector == "."+class {
							applyRule(rule)
						}
					}
				}
			}
		}

		// 3. ID matching
		for _, attrMap := range domNode.Attr {
			if id, exists := attrMap["id"]; exists {
				for _, rule := range css.Rules {
					if rule.Selector == "#"+id {
						applyRule(rule)
					}
				}
			}
		}
	}

	// Check if display was overridden
	if d, ok := layout.Styles["display"]; ok {
		displayType = d
	}

	// 2. Dimension Calculation (Block Formatting Context)
	// Default to available width for blocks, 0 for inline
	if displayType == "block" || domNode.Type == "document" || domNode.Type == "html" || domNode.Type == "body" {
		layout.W = availableWidth
		layout.Styles["display"] = "block" // normalize structural elements
	} else {
		layout.W = 0
	}
	layout.H = 0

	// Explicit dimension overrides
	if wStr, ok := layout.Styles["width"]; ok {
		if w, err := parseDimension(wStr); err == nil {
			layout.W = w
		}
	}

	if hStr, ok := layout.Styles["height"]; ok {
		if h, err := parseDimension(hStr); err == nil {
			layout.H = h
		}
	}

	// Text dimensions are calculated dynamically based on length for this mock milestone
	if domNode.Type == "text" {
		fontSize := 14.0
		if sizeStr, ok := layout.Styles["font-size"]; ok {
			if s, err := parseDimension(sizeStr); err == nil {
				fontSize = s
			}
		}
		layout.W = float64(len(domNode.Data)) * (fontSize * 0.6) // rough estimate for monospace
		layout.H = fontSize * 1.2
	}

	// 3. Children Flow (Block vs Inline flow)
	childX := currentX
	childY := currentY
	maxInlineHeight := 0.0

	isFlex := layout.Styles["display"] == "flex"
	flexDirection := layout.Styles["flex-direction"]
	if flexDirection == "" {
		flexDirection = "row" // default flex direction
	}

	for _, child := range domNode.Children {
		// Pass available width down (respecting parent padding/borders in a real engine)
		childLayout := computeNode(child, css, childX, childY, layout.W, layout.Styles)
		if childLayout != nil {
			layout.Children = append(layout.Children, childLayout)

			childDisplay := childLayout.Styles["display"]

			if isFlex {
				if flexDirection == "row" {
					// Flex Row: stack horizontally regardless of display type
					childLayout.X = childX
					childLayout.Y = currentY
					childX += childLayout.W
					if childLayout.H > layout.H {
						layout.H = childLayout.H
					}
				} else {
					// Flex Column: stack vertically
					childLayout.X = currentX
					childLayout.Y = childY
					childY += childLayout.H
				}
			} else if childDisplay == "inline" {
				// Inline Formatting Context: Wrap text/elements horizontally
				if childX+childLayout.W > currentX+layout.W && layout.W > 0 {
					// Line break
					childX = currentX
					childY += maxInlineHeight
					maxInlineHeight = 0

					// Reposition this child on the new line
					childLayout.X = childX
					childLayout.Y = childY
				}
				childX += childLayout.W
				if childLayout.H > maxInlineHeight {
					maxInlineHeight = childLayout.H
				}
			} else {
				// Block Formatting Context: Stack vertically
				if childX > currentX {
					// Preceding inline elements finish their line
					childY += maxInlineHeight
					childX = currentX
					maxInlineHeight = 0
				}

				// Re-adjust block element's Y if preceded by inline
				childLayout.X = currentX
				childLayout.Y = childY

				childY += childLayout.H
			}
		}
	}

	// Flush remaining inline height
	if childX > currentX {
		childY += maxInlineHeight
	}

	// Update the height of the parent based on children flow if not explicitly set
	if _, ok := layout.Styles["height"]; !ok {
		layout.H = childY - currentY
	}

	return layout
}

func parseDimension(val string) (float64, error) {
	val = strings.TrimSuffix(val, "px")
	return strconv.ParseFloat(val, 64)
}

// HitTest recursively searches the layout tree to find the deepest node intersecting with (x, y).
func HitTest(layout *gcc.LayoutTree, x, y float64) *gcc.LayoutTree {
	if layout == nil {
		return nil
	}

	// Check if coordinate is within the bounding box of this node
	if x >= layout.X && x <= (layout.X+layout.W) && y >= layout.Y && y <= (layout.Y+layout.H) {
		// Node contains point, now check children (last drawn / highest z-index gets priority)
		for i := len(layout.Children) - 1; i >= 0; i-- {
			hit := HitTest(layout.Children[i], x, y)
			if hit != nil {
				return hit
			}
		}
		// If no child contains the point, return this node
		return layout
	}

	return nil
}
