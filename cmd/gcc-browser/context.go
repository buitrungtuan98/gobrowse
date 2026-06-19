package main

import (
	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/internal/ipc"
)

type Tab struct {
	URL       string
	Title     string
	DOM       *gcc.DOMTree
	CSS       *gcc.CSSOMTree
	IsDirty   bool
	JSAdapter *ipc.JavascriptIPCAdapter
}

type BrowserContext struct {
	Tabs           []*Tab
	ActiveTab      int
	Orchestrator   *Orchestrator
	NetworkAdapter *ipc.NetworkIPCAdapter
	ParserAdapter  *ipc.ParserIPCAdapter
}

func NewBrowserContext(o *Orchestrator, net *ipc.NetworkIPCAdapter, parser *ipc.ParserIPCAdapter) *BrowserContext {
	return &BrowserContext{
		Tabs:           make([]*Tab, 0),
		ActiveTab:      -1,
		Orchestrator:   o,
		NetworkAdapter: net,
		ParserAdapter:  parser,
	}
}

func (ctx *BrowserContext) CreateTab(url, title string) *Tab {
	tab := &Tab{
		URL:     url,
		Title:   title,
		IsDirty: true,
	}
	ctx.Tabs = append(ctx.Tabs, tab)
	if ctx.ActiveTab == -1 {
		ctx.ActiveTab = 0
	}
	return tab
}

func (ctx *BrowserContext) GetActiveTab() *Tab {
	if ctx.ActiveTab >= 0 && ctx.ActiveTab < len(ctx.Tabs) {
		return ctx.Tabs[ctx.ActiveTab]
	}
	return nil
}

// Helper to set pseudo-state attributes on DOM nodes
func setPseudoState(node *gcc.DOMNode, state string, active bool) {
	if node == nil {
		return
	}

	attrName := "_" + state
	found := false
	for _, attr := range node.Attr {
		if _, ok := attr[attrName]; ok {
			if active {
				attr[attrName] = "true"
			} else {
				attr[attrName] = "false"
			}
			found = true
			break
		}
	}

	if !found && active {
		node.Attr = append(node.Attr, map[string]string{attrName: "true"})
	}
}

// Helper to clear pseudo-state recursively
func clearPseudoState(node *gcc.DOMNode, state string) {
	if node == nil {
		return
	}

	attrName := "_" + state
	for _, attr := range node.Attr {
		if _, ok := attr[attrName]; ok {
			attr[attrName] = "false"
		}
	}

	for _, child := range node.Children {
		clearPseudoState(child, state)
	}
}
