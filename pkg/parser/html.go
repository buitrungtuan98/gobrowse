package parser

import (
	"io"
	"strings"

	"github.com/go-chromium-core/gcc"
	"golang.org/x/net/html"
)

// parseHTMLNode recursively converts a standard x/net/html.Node into our custom gcc.DOMNode format.
func parseHTMLNode(n *html.Node, resources *[]string) *gcc.DOMNode {
	if n.Type == html.TextNode {
		// Ignore empty or purely whitespace text nodes
		data := strings.TrimSpace(n.Data)
		if data == "" {
			return nil
		}
		return &gcc.DOMNode{
			Type: "text",
			Data: data,
		}
	}

	if n.Type == html.ElementNode {
		node := &gcc.DOMNode{
			Type: n.Data, // Tag name, e.g., "div", "p", "a"
			Attr: make([]map[string]string, 0, len(n.Attr)),
		}

		// Map attributes
		for _, a := range n.Attr {
			attrMap := map[string]string{
				a.Key: a.Val,
			}
			node.Attr = append(node.Attr, attrMap)
		}

		// Recursively parse children
		// Resource extraction
		if n.Data == "link" {
			isCSS := false
			href := ""
			for _, a := range n.Attr {
				if a.Key == "rel" && a.Val == "stylesheet" {
					isCSS = true
				}
				if a.Key == "href" {
					href = a.Val
				}
			}
			if isCSS && href != "" {
				*resources = append(*resources, href)
			}
		} else if n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key == "src" && a.Val != "" {
					*resources = append(*resources, a.Val)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			childNode := parseHTMLNode(c, resources)
			if childNode != nil {
				node.Children = append(node.Children, childNode)
			}
		}

		return node
	}

	// For DocumentNode, CommentNode, DoctypeNode, we either skip or pass through to children
	if n.Type == html.DocumentNode {
		// Document root is usually an invisible container, just parse its children
		root := &gcc.DOMNode{
			Type: "document",
		}
		// Resource extraction
		if n.Data == "link" {
			isCSS := false
			href := ""
			for _, a := range n.Attr {
				if a.Key == "rel" && a.Val == "stylesheet" {
					isCSS = true
				}
				if a.Key == "href" {
					href = a.Val
				}
			}
			if isCSS && href != "" {
				*resources = append(*resources, href)
			}
		} else if n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key == "src" && a.Val != "" {
					*resources = append(*resources, a.Val)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			childNode := parseHTMLNode(c, resources)
			if childNode != nil {
				root.Children = append(root.Children, childNode)
			}
		}
		return root
	}

	return nil
}

// ParseHTML reads an HTML document from an io.Reader and generates a DOMTree.
func ParseHTML(r io.Reader) (*gcc.DOMTree, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var resources []string
	rootNode := parseHTMLNode(doc, &resources)
	return &gcc.DOMTree{
		Root:      rootNode,
		Resources: resources,
	}, nil
}
