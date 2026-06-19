package parser

import (
	"bytes"
	"testing"
)

func TestParseCSSMedia(t *testing.T) {
	css := `
		body { background: white; }
		@media (max-width: 600px) {
			body { background: black; }
			p { color: red; }
		}
		a { text-decoration: none; }
	`

	tree, err := ParseCSS(bytes.NewReader([]byte(css)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tree.Rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(tree.Rules))
	}

	if tree.Rules[0].Selector != "body" || tree.Rules[0].MediaQuery != "" {
		t.Errorf("rule 0 incorrect: %+v", tree.Rules[0])
	}

	if tree.Rules[1].Selector != "body" || tree.Rules[1].MediaQuery != "(max-width: 600px)" {
		t.Errorf("rule 1 incorrect: %+v", tree.Rules[1])
	}

	if tree.Rules[2].Selector != "p" || tree.Rules[2].MediaQuery != "(max-width: 600px)" {
		t.Errorf("rule 2 incorrect: %+v", tree.Rules[2])
	}

	if tree.Rules[3].Selector != "a" || tree.Rules[3].MediaQuery != "" {
		t.Errorf("rule 3 incorrect: %+v", tree.Rules[3])
	}
}
