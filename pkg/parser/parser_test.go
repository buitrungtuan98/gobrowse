package parser

import (
	"strings"
	"testing"
)

func TestParseHTML(t *testing.T) {
	htmlStr := `<html><body><div id="main"><p>Hello GCC</p></div></body></html>`
	r := strings.NewReader(htmlStr)

	stack := NewParserStack()
	dom, err := stack.ParseHTML(r)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if dom.Root == nil {
		t.Fatal("Expected non-nil DOM root")
	}

	// Root should be document, first child should be html
	if dom.Root.Type != "document" {
		t.Errorf("Expected root type 'document', got %s", dom.Root.Type)
	}
}

func TestParseCSS(t *testing.T) {
	cssStr := `body { background-color: #000; color: white; } p { font-size: 14px; }`
	r := strings.NewReader(cssStr)

	stack := NewParserStack()
	cssom, err := stack.ParseCSS(r)
	if err != nil {
		t.Fatalf("ParseCSS failed: %v", err)
	}

	if len(cssom.Rules) != 2 {
		t.Fatalf("Expected 2 CSS rules, got %d", len(cssom.Rules))
	}

	if cssom.Rules[0].Selector != "body" {
		t.Errorf("Expected selector 'body', got %s", cssom.Rules[0].Selector)
	}

	if cssom.Rules[0].Styles["background-color"] != "#000" {
		t.Errorf("Expected background-color #000, got %s", cssom.Rules[0].Styles["background-color"])
	}
}
