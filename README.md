Go-Chromium-Core (GCC)

Final Design Specification & Developer Onboarding Guide

Go-Chromium-Core (GCC) is a pluggable, multi-process, security-first browser engine framework written in pure Go. It features native, decoupled regular and Tor network transport layers designed specifically to protect user privacy, prevent identity cross-contamination, and support modular runtime customization.

1. Architectural Blueprint

GCC adopts a strict Multi-Process Architecture using IPC (implemented via gRPC and secured Named Pipes) to guarantee hardware-level isolation, crash resilience, and sandbox containment.

                               +---------------------------+
                               |     Main UI Process       |
                               | (Window, Tabs, Navigation)|
                               +---------------------------+
                                     /       |       \
               +--------------------+        |        +--------------------+
               |                     | (gRPC IPC)     |                    |
               v                     v                v                    v
+--------------------+ +--------------------+  +--------------------+ +--------------------+
|  Network Process   | |   Render Process   |  |   Render Process   | | JS Runtime Process |
| (Regular Context)  | |  (Tab 1 - Isolated)|  |  (Tab 2 - Isolated)| |  (V8 / Goja Engine)|
+--------------------+ +--------------------+  +--------------------+ +--------------------+
          |                      |                      |                      |
          v                      v                      v                      v
   [ Clearnet/HTTP ]      [ Layout/Painting ]    [ Layout/Painting ]     [ Sandbox/DOM Bind ]
          OR
   [ Tor SOCKS5/DNS ]


Process Isolation Model

Main UI Process (Orchestrator): Runs with user-level privileges. Orchestrates window frames, user interactions, tabs state, and enforces IPC access control. It has no direct access to network sockets or unparsed raw web assets.

Network Process: Runs with network access privileges (CAP_NET_RAW / standard socket creation). It acts as a sandboxed local proxy managing connection pooling, TLS handshakes, DNS caches, and Tor circuits.

Render Process (Tab-isolated): Runs in an unprivileged sandbox (using Linux namespaces, seccomp, or Windows AppContainer). It parses HTML/CSS and computes layout metrics. It has zero network access and can only communicate visual frames or layout requests back to the UI Process.

JS Runtime Process: Executes client-side scripts inside a sandboxed execution environment. Mutates the mock DOM structures and pushes event cycles back to the renderer.

2. Core Interfaces (engine.go)

The core of GCC's modular design is interface-driven development. Developers can swap, test, and mock any layer of the browser engine by satisfying these interfaces.

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
	ModeTor                       // SOCKS5 and remote Onion DNS routing
)

// FetchOptions configures individual network request characteristics.
type FetchOptions struct {
	Method            string
	Headers           http.Header
	Body              io.Reader
	SkipCache         bool
	CustomUserAgent   string
	DisableCookies    bool
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
	ComputeLayout(dom *DOMTree, css *CSSOMTree, viewportWidth, viewportHeight float64) (*LayoutTree, error)
	Paint(layout *LayoutTree, canvas TargetCanvas) error
}

// JSEngine executes scripts and handles bindings with the DOM.
type JSEngine interface {
	ExecuteScript(script string) (interface{}, error)
	BindGlobalAPI(name string, handler interface{}) error
}


3. Standard Project Layout

GCC adheres to standard Go project layouts (historically influenced by the Golang Standard Project Layout) to separate concerns and protect private binaries.

go-chromium-core/
├── api/                  # gRPC Proto definitions for secure Inter-Process Communication (IPC)
│   ├── network.proto     # Network proxy services
│   ├── renderer.proto    # Layout calculations & rasterization RPCs
│   └── javascript.proto  # Sandboxed execution instructions
├── cmd/                  # Executable entrance binaries
│   ├── gcc-browser/      # Reference implementation (Demo hybrid graphical browser)
│   └── gcc-daemon/       # Headless browser daemon agent (Scraper / automation host)
├── pkg/                  # Public reusable framework modules
│   ├── network/          # Isolated transport layers (Clearnet http.Client vs Tor SOCKS5 Proxy)
│   ├── parser/           # HTML5/CSS Lexers & Parsers (DOM & CSSOM builder)
│   ├── javascript/       # Pure-Go ECMAScript Interpreter (Goja integration wrapper)
│   └── render/           # Cross-platform GPU/CPU paint engines (Gio / Fyne integrations)
├── internal/             # Framework-locked private utilities (Non-importable system setups)
│   ├── sandbox/          # seccomp/unshare container utilities for isolated execution
│   └── ipc/              # Named Pipe and UNIX Socket connection handshakes
├── docs/                 # Component integration guides, threat models, and specifications
├── go.mod                # Dependency tracking
└── README.md             # Developer onboarding, architecture map, and quick-start


4. Hybrid Security & Routing Pipeline

Network isolation prevents identity leaks, correlation attacks, and cross-contamination between runtime contexts:

Security Vector

Regular Mode

Tor Mode

Transport Layer

Native WAN / Direct Clearnet via TCP sockets

Encrypted Tor Circuit Tunnel (127.0.0.1:9050)

DNS Resolution

Local OS System DNS / Cloudflare DoH

Forced Remote Onion Resolution (Guards against DNS leaks)

WebRTC State

Enabled (with optional STUN/TURN binding)

Hard Disabled (Stripped APIs, RTCConfiguration blocked)

Storage Context

Persistent SQLite databases, Session & Disk Caching

Ephemeral, In-Memory Only (Zero Disk footprint)

Fingerprint Noise

Native Browser User-Agent, Hardware Canvas info

Randomized Canvas Noise & Standardized Aspect Ratio / Bounds

5. Getting Started & Quick-Start

Prerequisites

Make sure you have Go (1.21+) and a local Tor daemon installed.

# Install Tor daemon on macOS
brew install tor

# Install Tor daemon on Ubuntu/Debian
sudo apt-get install tor


Ensure your Tor service is running locally on port 9050:

tor --SocksPort 9050 &


Initializing the GCC Framework

The following boilerplate initializes GCC, boots up a secure network proxy, configures it to run over the Tor network stack, and parses a simple webpage.

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"[github.com/go-chromium-core/gcc](https://github.com/go-chromium-core/gcc)"
	"[github.com/go-chromium-core/gcc/pkg/network](https://github.com/go-chromium-core/gcc/pkg/network)"
	"[github.com/go-chromium-core/gcc/pkg/parser](https://github.com/go-chromium-core/gcc/pkg/parser)"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Initialize our secure, decoupled Network Layer
	netStack := network.NewNetworkStack()

	// 2. Configure network stack to run over Tor context
	err := netStack.SetRoutingMode(gcc.ModeTor)
	if err != nil {
		log.Fatalf("Failed to initialize Tor routing pipeline: %v", err)
	}

	// 3. Perform onion-safe remote DNS configuration
	dnsErr := netStack.ConfigureDNS(gcc.DNSConfig{
		Provider: "LocalTor",
		Servers:  []string{"127.0.0.1:9050"},
	})
	if dnsErr != nil {
		log.Fatalf("Unable to configure remote Onion DNS protection: %v", dnsErr)
	}

	// 4. Fetch an onion address securely
	fmt.Println("[GCC] Requesting resource via Tor pipeline...")
	response, err := netStack.Fetch(ctx, "[http://3g2upl4pq6kufc4m.onion](http://3g2upl4pq6kufc4m.onion)", gcc.FetchOptions{
		Method: "GET",
		Headers: map[string][]string{
			"User-Agent": {"GCC-Tor-Secure/1.0.0"},
		},
	})
	if err != nil {
		log.Fatalf("Network Fetch Error: %v", err)
	}
	defer response.Body.Close()

	// 5. Build our secure parser engine
	parserEngine := parser.NewHTMLParser()
	domTree, parseErr := parserEngine.ParseHTML(response.Body)
	if parseErr != nil {
		log.Fatalf("HTML Parsing Error: %v", parseErr)
	}

	fmt.Printf("[GCC] Success! Parsed DOM Tree with Root Node: %s\n", domTree.Root.Type)
}


6. Extensibility Framework

GCC is engineered to accommodate broad configuration scenarios for developers.

A. Headless / UI-less Scraping Operations

By bypassing GUI requirements, you can strip away OpenGL or GPU dependencies and use GCC as a lightning-fast, sandboxed scraper.

To build a customized renderer that outputs directly to standard terminal logs or text output:

package myrenderer

import (
	"fmt"
	"[github.com/go-chromium-core/gcc](https://github.com/go-chromium-core/gcc)"
)

type TerminalRenderer struct{}

func (tr *TerminalRenderer) ComputeLayout(dom *gcc.DOMTree, css *gcc.CSSOMTree, viewportWidth, viewportHeight float64) (*gcc.LayoutTree, error) {
	// Construct basic structure nodes mapped directly into geometry bounding containers
	return &gcc.LayoutTree{
		Node: dom.Root,
		W:    80, // standardized monospace terminal width
		H:    40,
	}, nil
}

func (tr *TerminalRenderer) Paint(layout *gcc.LayoutTree, canvas gcc.TargetCanvas) error {
	// Print directly to text pipelines instead of GUI screens
	fmt.Printf("[Console Paint] Node %s rendered onto viewport coords (%f, %f)\n", 
		layout.Node.Type, layout.X, layout.Y)
	return nil
}


B. Web3 & Crypto DApp Integration

To inject a DApp context directly inside web rendering frames, you can pass custom hooks directly inside the JavaScript runner using BindGlobalAPI.

package main

import (
	"fmt"
	"[github.com/go-chromium-core/gcc/pkg/javascript](https://github.com/go-chromium-core/gcc/pkg/javascript)"
)

type EthProvider struct{}

func (p *EthProvider) Request(method string, params []interface{}) (interface{}, error) {
	// Custom wallet injection mapping (simulate MetaMask behavior)
	return fmt.Sprintf("Injected action: %s executed successfully", method), nil
}

func main() {
	jsEngine := javascript.NewGojaEngine()

	// Inject window.ethereum into global JavaScript frame
	err := jsEngine.BindGlobalAPI("ethereum", &EthProvider{})
	if err != nil {
		panic(err)
	}

	result, _ := jsEngine.ExecuteScript("ethereum.Request('eth_accounts', [])")
	fmt.Println(result) // Prints: Injected action: eth_accounts executed successfully
}


C. Swapping to Alternative Engine Architectures (V8 / QuickJS)

For applications that require faster execution speeds than pure Go, you can swap the default interpreter out for a CGO wrapper wrapping standard V8 or QuickJS. Simply implement the JSEngine interface structure:

package v8engine

import "[github.com/go-chromium-core/gcc](https://github.com/go-chromium-core/gcc)"

type V8Runner struct {
	// Private V8 context structures
}

func NewV8Runner() *V8Runner {
	return &V8Runner{}
}

func (v *V8Runner) ExecuteScript(script string) (interface{}, error) {
	// Map script directly to low-level CGO V8 bindings
	return nil, nil
}

func (v *V8Runner) BindGlobalAPI(name string, handler interface{}) error {
	// Implement secure CGO bridging
	return nil
}


7. Security Architecture & Threat Vectors

Threat Modeling & Mitigations

Identity Leak via DNS Lookup ($P_{\text{leak}} \to 0$): Traditional Tor proxies can leak local domain queries if the OS performs a system lookup before passing commands to SOCKS. GCC prevents this by completely routing DNS lookups inside the Network Process to the Tor controller using forced Remote Onion Resolution.

WebRTC Network Disclosure: When enabled, WebRTC exposes the local IP address of standard network devices ($I_{local}$), bypassing proxy configurations. GCC actively isolates or disables WebRTC bindings during Tor Routing Mode by setting custom engine-level headers and blocking connection handshakes.

Micro-timing Side-Channel Exploits: Scripts run inside a strictly defined, separate execution sandbox (JSEngine process). The runtime loop does not share CPU or cache segments with the browser UI engine process, minimizing risk from local cross-origin timing attacks.

8. Open-Source Project Milestones

  Phase 1: Core Definitions (Go Interfaces, Decoupled Proxy Struct) [DONE]
    └─ Establish network transport modes & routing logic pipelines.
  Phase 2: gRPC Process IPC Isolation [In Progress]
    └─ Separate parsing, JavaScript execution, and UI layers into independent processes.
  Phase 3: Sandbox Locking
    └─ Deploy strict seccomp profiles to isolate child parser tasks.
  Phase 4: Hardware Paint Acceleration
    └─ Complete GPU layout execution models over cross-platform Gio pipelines.


9. Contributing to GCC

We welcome modular enhancements and optimizations!

Fork the codebase on GitHub.

Build local tests: go test ./pkg/...

Adhere to strict modular boundaries: keep GUI render dependencies isolated from the core parsing loop, and guarantee that no network communication is processed inside standard tab sandboxes.

Go-Chromium-Core is licensed under the Apache 2.0 Security & Software License.
