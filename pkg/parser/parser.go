package parser

import (
	"io"

	"github.com/go-chromium-core/gcc"
)

// ParserStack is the concrete implementation of gcc.ParserEngine.
type ParserStack struct{}

// NewParserStack initializes a new DOM/CSSOM parser engine.
func NewParserStack() *ParserStack {
	return &ParserStack{}
}

// ParseHTML acts as the primary facade for the internal HTML parser.
func (p *ParserStack) ParseHTML(r io.Reader) (*gcc.DOMTree, error) {
	return ParseHTML(r)
}

// ParseCSS acts as the primary facade for the internal CSS parser.
func (p *ParserStack) ParseCSS(r io.Reader) (*gcc.CSSOMTree, error) {
	return ParseCSS(r)
}
