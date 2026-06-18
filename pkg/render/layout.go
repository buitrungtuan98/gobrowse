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

	rootLayout := computeNode(dom.Root, css, 0, 0)
	return rootLayout, nil
}

// computeNode recursively builds the layout structure.
func computeNode(domNode *gcc.DOMNode, css *gcc.CSSOMTree, currentX, currentY float64) *gcc.LayoutTree {
	if domNode == nil {
		return nil
	}

	layout := &gcc.LayoutTree{
		Node:   domNode,
		X:      currentX,
		Y:      currentY,
		Styles: make(map[string]string),
	}

	// Apply styles from CSSOM if the selector matches the node type or id/class (simplified matching)
	if css != nil {
		for _, rule := range css.Rules {
			match := false
			if rule.Selector == domNode.Type {
				match = true
			} else if strings.HasPrefix(rule.Selector, "#") || strings.HasPrefix(rule.Selector, ".") {
				selectorName := rule.Selector[1:]
				attrKey := "id"
				if strings.HasPrefix(rule.Selector, ".") {
					attrKey = "class"
				}

				for _, attrMap := range domNode.Attr {
					if val, exists := attrMap[attrKey]; exists {
						if val == selectorName {
							match = true
							break
						}
					}
				}
			}

			if match {
				for k, v := range rule.Styles {
					layout.Styles[k] = v
				}
			}
		}
	}

	// Basic dimension defaults
	layout.W = 100 // default width
	layout.H = 20  // default height

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

	// Calculate children layout
	childY := currentY
	for _, child := range domNode.Children {
		childLayout := computeNode(child, css, currentX, childY)
		if childLayout != nil {
			layout.Children = append(layout.Children, childLayout)
			// Stack children vertically for now
			childY += childLayout.H
		}
	}

	// Update the height of the parent based on children
	if len(layout.Children) > 0 {
		layout.H = childY - currentY
	}

	return layout
}

func parseDimension(val string) (float64, error) {
	val = strings.TrimSuffix(val, "px")
	return strconv.ParseFloat(val, 64)
}
