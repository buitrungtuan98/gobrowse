package parser

import (
	"bytes"
	"io"
	"strings"

	"github.com/go-chromium-core/gcc"
)

// ParseCSS is a basic, rudimentary CSS parser designed to fulfill the initial mock/basic
// milestone. It parses simple rules like `selector { key: value; }`.
// Note: A full CSS engine requires complex tokenization (e.g. nested rules, @media, etc.).
func ParseCSS(r io.Reader) (*gcc.CSSOMTree, error) {
	var rules []gcc.CSSRule

	// Read entire CSS content into memory for basic string manipulation
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	cssContent := buf.String()

	// Quick and dirty parser: split by closing brace to isolate blocks
	blocks := strings.Split(cssContent, "}")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Expected block format: `selector { key: value; key: value; `
		parts := strings.Split(block, "{")
		if len(parts) != 2 {
			continue // Skip malformed blocks
		}

		selector := strings.TrimSpace(parts[0])
		stylesStr := strings.TrimSpace(parts[1])

		rule := gcc.CSSRule{
			Selector: selector,
			Styles:   make(map[string]string),
		}

		// Parse key-value declarations split by semicolons
		declarations := strings.Split(stylesStr, ";")
		for _, decl := range declarations {
			decl = strings.TrimSpace(decl)
			if decl == "" {
				continue
			}

			kv := strings.SplitN(decl, ":", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				value := strings.TrimSpace(kv[1])
				rule.Styles[key] = value
			}
		}

		if len(rule.Styles) > 0 {
			rules = append(rules, rule)
		}
	}

	return &gcc.CSSOMTree{
		Rules: rules,
	}, nil
}
