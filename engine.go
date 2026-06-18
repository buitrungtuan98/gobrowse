package gcc

import (
	"context"
	"io"
	"net/http"
)

// RoutingMode defines the isolation state of the network stack.
type RoutingMode int

const (
	ModeRegular RoutingMode = iota // Native Clearnet
	ModeTor                        // SOCKS5 and remote Onion DNS routing
)

// FetchOptions configures individual network request characteristics.
type FetchOptions struct {
	Method          string
	Headers         http.Header
	Body            io.Reader
	SkipCache       bool
	CustomUserAgent string
	DisableCookies  bool
}

// Response models the unified result returned from network interfaces.
type Response struct {
	StatusCode int
	Proto      string
	Header     http.Header
	Body       io.ReadCloser
}

// DNSConfig specifies upstream name resolution servers.
type DNSConfig struct {
	Provider string   // e.g., "Cloudflare", "Quad9", "LocalTor"
	Servers  []string // IP list or DoH URL
}

// DOMTree represents the in-memory document object model.
type DOMTree struct {
	Root *DOMNode
}

type DOMNode struct {
	Type     string
	Data     string
	Attr     []map[string]string
	Children []*DOMNode
}

// CSSOMTree represents computed style rules parsed from CSS.
type CSSOMTree struct {
	Rules []CSSRule
}

type CSSRule struct {
	Selector string
	Styles   map[string]string
}

// LayoutTree links DOM structure with computed geometric style dimensions.
type LayoutTree struct {
	Node     *DOMNode
	X, Y     float64
	W, H     float64
	Styles   map[string]string
	Children []*LayoutTree
}

// TargetCanvas abstracts the visual paint surface.
type TargetCanvas interface {
	DrawRect(x, y, w, h float64, hexColor string)
	DrawText(x, y float64, text string, font string, size float64)
	DrawImage(x, y float64, data []byte)
	Flush() error
}

// ==========================================
// Primary GCC Plug Interfaces
// ==========================================

// NetworkEngine handles multi-context resource fetching and routing policies.
type NetworkEngine interface {
	Fetch(ctx context.Context, url string, opt FetchOptions) (*Response, error)
	SetRoutingMode(mode RoutingMode) error // ModeRegular or ModeTor
	ConfigureDNS(provider DNSConfig) error
}

// ParserEngine tokens and builds memory structures from raw web assets.
type ParserEngine interface {
	ParseHTML(r io.Reader) (*DOMTree, error)
	ParseCSS(r io.Reader) (*CSSOMTree, error)
}

// RenderEngine computes visual layout metrics and rasterizes elements to viewport.
type RenderEngine interface {
	ComputeLayout(dom *DOMTree, css *CSSOMTree) (*LayoutTree, error)
	Paint(layout *LayoutTree, canvas TargetCanvas) error
}

// JSEngine executes scripts and handles bindings with the DOM.
type JSEngine interface {
	ExecuteScript(script string) (interface{}, error)
	BindGlobalAPI(name string, handler interface{}) error
}
