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

	// Quick and dirty parser: find blocks properly to handle nested blocks like @media
	var currentMedia string
	inMediaBlock := false

	// Basic lexing loop
	var buffer string
	for i := 0; i < len(cssContent); i++ {
		char := cssContent[i]

		if char == '{' {
			header := strings.TrimSpace(buffer)
			buffer = ""

			if strings.HasPrefix(header, "@media") {
				currentMedia = strings.TrimSpace(strings.TrimPrefix(header, "@media"))
				inMediaBlock = true
				continue
			}

			// We are at a standard rule block start
			// Parse the styles until '}'
			var stylesBuf string
			nestedBraces := 1
			for i++; i < len(cssContent); i++ {
				if cssContent[i] == '{' {
					nestedBraces++
				} else if cssContent[i] == '}' {
					nestedBraces--
					if nestedBraces == 0 {
						break
					}
				}
				stylesBuf += string(cssContent[i])
			}

			rule := gcc.CSSRule{
				Selector:   header,
				Styles:     make(map[string]string),
				MediaQuery: currentMedia,
			}

			declarations := strings.Split(stylesBuf, ";")
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
		} else if char == '}' {
			if inMediaBlock {
				inMediaBlock = false
				currentMedia = ""
			}
			buffer = ""
		} else {
			buffer += string(char)
		}
	}

	return &gcc.CSSOMTree{
		Rules: rules,
	}, nil
}
