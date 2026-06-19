package render

import (
	"strconv"
	"strings"

	"github.com/go-chromium-core/gcc"
)

// ComputeLayout walks the DOM tree and applies styles from the CSSOM tree to build a LayoutTree.
func ComputeLayout(dom *gcc.DOMTree, css *gcc.CSSOMTree, viewportWidth, viewportHeight float64) (*gcc.LayoutTree, error) {
	if dom == nil || dom.Root == nil {
		return nil, nil
	}

	if viewportWidth <= 0 {
		viewportWidth = 800.0
	}

	rootLayout := computeNode(dom.Root, css, 0, 0, viewportWidth, nil)
	return rootLayout, nil
}

// computeNode recursively builds the layout structure.

// Inheritable properties
type childResult struct {
	Index  int
	Layout *gcc.LayoutTree
}

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

	// Apply inline styles first (highest priority)
	for _, attr := range domNode.Attr {
		if styleStr, ok := attr["style"]; ok {
			rules := strings.Split(styleStr, ";")
			for _, rule := range rules {
				kv := strings.Split(rule, ":")
				if len(kv) == 2 {
					layout.Styles[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}
	}

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

	// Only set default if it wasn't already set by inline styles
	if _, ok := layout.Styles["display"]; !ok {
		layout.Styles["display"] = displayType
	}

	if css != nil {
		// Very basic cascade: Element -> Class -> ID
		applyRule := func(rule gcc.CSSRule) {
			for k, v := range rule.Styles {
				layout.Styles[k] = v
			}
		}

		// Helper to check pseudo-classes
		hasPseudoState := func(pseudo string) bool {
			stateAttr := "_" + strings.TrimPrefix(pseudo, ":")
			for _, attrMap := range domNode.Attr {
				if val, exists := attrMap[stateAttr]; exists && val == "true" {
					return true
				}
			}
			return false
		}

		// Helper to match selectors including pseudo-classes
		matchSelector := func(baseSelector, ruleSelector string) bool {
			if ruleSelector == baseSelector {
				return true
			}
			if strings.HasPrefix(ruleSelector, baseSelector+":") {
				pseudo := strings.TrimPrefix(ruleSelector, baseSelector)
				return hasPseudoState(pseudo)
			}
			return false
		}

		// Helper to check media queries against current viewport
		matchMedia := func(query string) bool {
			if query == "" {
				return true
			}
			// Basic max-width / min-width evaluator
			if strings.Contains(query, "max-width:") {
				parts := strings.Split(query, "max-width:")
				if len(parts) == 2 {
					valStr := strings.TrimSpace(strings.TrimSuffix(parts[1], ")"))
					if maxW, err := parseDimension(valStr); err == nil {
						return availableWidth <= maxW
					}
				}
			}
			if strings.Contains(query, "min-width:") {
				parts := strings.Split(query, "min-width:")
				if len(parts) == 2 {
					valStr := strings.TrimSpace(strings.TrimSuffix(parts[1], ")"))
					if minW, err := parseDimension(valStr); err == nil {
						return availableWidth >= minW
					}
				}
			}
			return false // Unrecognized query fails match
		}

		// 1. Tag matching
		for _, rule := range css.Rules {
			if matchMedia(rule.MediaQuery) && matchSelector(domNode.Type, rule.Selector) {
				applyRule(rule)
			}
		}

		// 2. Class matching
		for _, attrMap := range domNode.Attr {
			if classes, exists := attrMap["class"]; exists {
				classArray := strings.Split(classes, " ")
				for _, class := range classArray {
					for _, rule := range css.Rules {
						if matchMedia(rule.MediaQuery) && matchSelector("."+class, rule.Selector) {
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
					if matchMedia(rule.MediaQuery) && matchSelector("#"+id, rule.Selector) {
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
	if displayType == "block" || displayType == "flex" || displayType == "grid" || domNode.Type == "document" || domNode.Type == "html" || domNode.Type == "body" {
		layout.W = availableWidth
		if domNode.Type == "document" || domNode.Type == "html" || domNode.Type == "body" {
			if displayType == "" || displayType == "inline" {
				layout.Styles["display"] = "block" // normalize structural elements
				displayType = "block"
			}
		}
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

	// 3. Children Flow (Parallel computation using Channels)
	// We compute the raw intrinsic dimensions of all children concurrently.
	// We CANNOT finalize their X,Y positions in parallel because Flow Layout depends on previous siblings.
	// But we CAN parallelize the heavy DOM evaluation, CSS matching, and intrinsic width/height calcs.

	childCount := len(domNode.Children)
	results := make(chan childResult, childCount)

	for i, child := range domNode.Children {
		go func(idx int, c *gcc.DOMNode) {
			// Compute layout independently (assuming relative X=0, Y=0 initially)
			// Pass available width down (respecting parent padding/borders in a real engine)
			cLayout := computeNode(c, css, 0, 0, layout.W, layout.Styles)
			results <- childResult{Index: idx, Layout: cLayout}
		}(i, child)
	}

	// Collect computed children and store them in correct order
	computedChildren := make([]*gcc.LayoutTree, childCount)
	for i := 0; i < childCount; i++ {
		res := <-results
		computedChildren[res.Index] = res.Layout
	}
	close(results)

	// 4. Sequential Flow Positioning
	// Now that heavy lifting (dimension/css calculation) is done concurrently,
	// we sequence their X,Y bounds quickly on the main routine.

	childX := currentX
	childY := currentY
	maxInlineHeight := 0.0

	// displayType is already defined earlier, just retrieve the current one
	displayType = layout.Styles["display"]
	isFlex := displayType == "flex"
	isGrid := displayType == "grid"
	flexDirection := layout.Styles["flex-direction"]
	if flexDirection == "" {
		flexDirection = "row" // default flex direction
	}

	if isGrid {
		// Basic Grid Implementation
		gridColsStr := layout.Styles["grid-template-columns"]
		gridRowsStr := layout.Styles["grid-template-rows"]
		gapStr := layout.Styles["gap"]
		if gapStr == "" {
			gapStr = layout.Styles["grid-gap"]
		}

		gap := 0.0
		if gapStr != "" {
			gap, _ = parseDimension(gapStr)
		}

		cols := strings.Fields(gridColsStr)
		rows := strings.Fields(gridRowsStr)

		colWidths := make([]float64, len(cols))
		for i, c := range cols {
			w, _ := parseDimension(c)
			colWidths[i] = w
		}

		rowHeights := make([]float64, len(rows))
		for i, r := range rows {
			h, _ := parseDimension(r)
			rowHeights[i] = h
		}

		currentCol := 0
		currentRow := 0

		for _, childLayout := range computedChildren {
			if childLayout != nil {
				layout.Children = append(layout.Children, childLayout)

				// Parse grid-column and grid-row overrides
				colSpan := 1
				rowSpan := 1
				colStart := currentCol
				rowStart := currentRow

				if colStr, ok := childLayout.Styles["grid-column"]; ok {
					parts := strings.Split(colStr, "/")
					if len(parts) > 0 {
						if c, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
							colStart = c - 1 // 1-based to 0-based
						}
					}
					if len(parts) > 1 {
						if c, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							colSpan = c - colStart - 1 // End line minus start line
						} else if strings.Contains(parts[1], "span") {
							spanParts := strings.Fields(parts[1])
							if len(spanParts) == 2 {
								if s, err := strconv.Atoi(spanParts[1]); err == nil {
									colSpan = s
								}
							}
						}
					}
				}

				if rowStr, ok := childLayout.Styles["grid-row"]; ok {
					parts := strings.Split(rowStr, "/")
					if len(parts) > 0 {
						if r, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
							rowStart = r - 1
						}
					}
					if len(parts) > 1 {
						if r, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							rowSpan = r - rowStart - 1
						} else if strings.Contains(parts[1], "span") {
							spanParts := strings.Fields(parts[1])
							if len(spanParts) == 2 {
								if s, err := strconv.Atoi(spanParts[1]); err == nil {
									rowSpan = s
								}
							}
						}
					}
				}

				// Calculate position
				cx := currentX
				for i := 0; i < colStart && i < len(colWidths); i++ {
					cx += colWidths[i] + gap
				}

				cy := currentY
				for i := 0; i < rowStart && i < len(rowHeights); i++ {
					cy += rowHeights[i] + gap
				}

				// Calculate size
				cw := 0.0
				for i := colStart; i < colStart+colSpan && i < len(colWidths); i++ {
					cw += colWidths[i]
					if i > colStart {
						cw += gap
					}
				}

				ch := 0.0
				for i := rowStart; i < rowStart+rowSpan && i < len(rowHeights); i++ {
					ch += rowHeights[i]
					if i > rowStart {
						ch += gap
					}
				}

				childLayout.X = cx
				childLayout.Y = cy
				childLayout.W = cw
				childLayout.H = ch
				offsetSubTree(childLayout, cx, cy)

				// Auto-placement progression
				currentCol = colStart + colSpan
				if currentCol >= len(colWidths) {
					currentCol = 0
					currentRow = rowStart + 1
				} else {
					currentRow = rowStart
				}

				// Keep track of grid container height
				if ch+cy-currentY > layout.H {
					layout.H = ch + cy - currentY
				}
			}
		}
	} else {
		for _, childLayout := range computedChildren {
			if childLayout != nil {
				layout.Children = append(layout.Children, childLayout)

				childDisplay := childLayout.Styles["display"]

				if isFlex {
					if flexDirection == "row" {
						// Flex Row: stack horizontally
						childLayout.X = childX
						childLayout.Y = currentY

						// Recursively offset any deep children inside this container due to positional shifting
						offsetSubTree(childLayout, childX, currentY)

						childX += childLayout.W
						if childLayout.H > layout.H {
							layout.H = childLayout.H
						}
					} else {
						// Flex Column: stack vertically
						childLayout.X = currentX
						childLayout.Y = childY
						offsetSubTree(childLayout, currentX, childY)
						childY += childLayout.H
					}
				} else if childDisplay == "inline" {
					// Inline Formatting Context: Wrap horizontally
					if childX+childLayout.W > currentX+layout.W && layout.W > 0 {
						childX = currentX
						childY += maxInlineHeight
						maxInlineHeight = 0
					}

					childLayout.X = childX
					childLayout.Y = childY
					offsetSubTree(childLayout, childX, childY)

					childX += childLayout.W
					if childLayout.H > maxInlineHeight {
						maxInlineHeight = childLayout.H
					}
				} else {
					// Block Formatting Context: Stack vertically
					if childX > currentX {
						childY += maxInlineHeight
						childX = currentX
						maxInlineHeight = 0
					}

					childLayout.X = currentX
					childLayout.Y = childY
					offsetSubTree(childLayout, currentX, childY)

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
	}

	// Make sure grid layout overrides container height correctly
	if layout.Styles["display"] == "grid" && layout.H < 0 {
		layout.H = 0
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

// offsetSubTree recursively shifts the physical coordinates of a tree once
// its parent's flow position has been finalized.
func offsetSubTree(layout *gcc.LayoutTree, dX, dY float64) {
	if layout == nil {
		return
	}
	for _, child := range layout.Children {
		child.X += dX
		child.Y += dY
		offsetSubTree(child, dX, dY)
	}
}
