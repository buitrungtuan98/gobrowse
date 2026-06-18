package render

import (
	"fmt"
)

// TerminalCanvas acts as a mock GUI surface, translating draw calls to standard output.
type TerminalCanvas struct {
	logs []string
}

// NewTerminalCanvas creates a new mock canvas.
func NewTerminalCanvas() *TerminalCanvas {
	return &TerminalCanvas{
		logs: make([]string, 0),
	}
}

// DrawRect simulates drawing a solid rectangle color.
func (c *TerminalCanvas) DrawRect(x, y, w, h float64, hexColor string) {
	msg := fmt.Sprintf("DrawRect: [%.2f, %.2f, %.2f, %.2f] Color: %s", x, y, w, h, hexColor)
	c.logs = append(c.logs, msg)
}

// DrawText simulates rendering a font sequence onto coordinates.
func (c *TerminalCanvas) DrawText(x, y float64, text string, font string, size float64) {
	msg := fmt.Sprintf("DrawText: [%.2f, %.2f] '%s' Font: %s-%.2f", x, y, text, font, size)
	c.logs = append(c.logs, msg)
}

// DrawImage simulates dumping raw byte graphics.
func (c *TerminalCanvas) DrawImage(x, y float64, data []byte) {
	msg := fmt.Sprintf("DrawImage: [%.2f, %.2f] Image Data Length: %d bytes", x, y, len(data))
	c.logs = append(c.logs, msg)
}

// Flush streams the collected layout metrics to stdout.
func (c *TerminalCanvas) Flush() error {
	for _, log := range c.logs {
		fmt.Println("[TerminalCanvas]", log)
	}
	// Clear the mock buffer after painting
	// c.logs = c.logs[:0]
	return nil
}

// GetLogs is a helper method to inspect what was drawn during testing
func (c *TerminalCanvas) GetLogs() []string {
	return c.logs
}
