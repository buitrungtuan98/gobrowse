# Go-Chromium-Core (GCC) Development Plan

This document outlines the granular phases, architectural components, and detailed tasks required to build the Go-Chromium-Core browser engine framework.

## Phase 1: Core Definitions & Framework Bootstrap (Current Focus)
**Goal:** Establish the foundation of the project by defining interfaces, directory structures, and core data types.
- [x] **Task 1.1:** Initialize the Go module (`go.mod`).
- [x] **Task 1.2:** Scaffold the standard directory structure (`api`, `cmd`, `pkg`, `internal`, `docs`).
- [x] **Task 1.3:** Define core types (`RoutingMode`, `FetchOptions`, `Response`, `DOMTree`, `CSSOMTree`, `LayoutTree`) in `engine.go`.
- [x] **Task 1.4:** Define primary plugin interfaces (`NetworkEngine`, `ParserEngine`, `RenderEngine`, `JSEngine`, `TargetCanvas`) in `engine.go`.

## Phase 2: Core Components Implementation (Mock / Basic level)
**Goal:** Implement the fundamental modules behind the core interfaces.
- [x] **Task 2.1 - Network (`pkg/network`):**
  * Implement `Clearnet` HTTP fetcher.
  * Implement `Tor` proxy dialer with SOCKS5 and forced remote DNS resolution.
- [x] **Task 2.2 - Parser (`pkg/parser`):**
  * Build an HTML lexer and parser that generates a `DOMTree`.
  * Build a basic CSS parser to generate `CSSOMTree`.
- [x] **Task 2.3 - Layout & Render (`pkg/render`):**
  * Implement a simple layout algorithm to combine `DOMTree` and `CSSOMTree` into a `LayoutTree`.
  * Create a mock text-based or terminal renderer (`TargetCanvas`).
- [x] **Task 2.4 - JavaScript (`pkg/javascript`):**
  * Integrate `goja` to provide a JS runtime.
  * Implement the `BindGlobalAPI` to inject basic window/document context.

## Phase 3: gRPC Process IPC Isolation
**Goal:** Separate the monolithic design into isolated, concurrent processes using gRPC and Named Pipes/Unix Sockets.
- [x] **Task 3.1 - Protobuf Definitions (`api`):**
  * Define `network.proto` for fetching resources via a secure proxy process.
  * Define `renderer.proto` for IPC layout calculation and paint instructions.
  * Define `javascript.proto` for JS DOM mutation events and execution results.
- [x] **Task 3.2 - Main Orchestrator (`cmd/gcc-browser`):**
  * Implement process management (spawning child processes).
  * Setup gRPC servers in child processes and clients in the main UI process.
- [x] **Task 3.3 - IPC Adapters (`internal/ipc`):**
  * Implement wrappers that satisfy the `engine.go` interfaces but internally translate calls to gRPC over Named Pipes/Sockets.

## Phase 4: Sandbox Locking & Security Enforcements
**Goal:** Ensure untrusted code (Render & JS) executes in highly constrained environments.
- [x] **Task 4.1 - Seccomp/Namespaces (`internal/sandbox`):**
  * Implement Linux namespace separation (CLONE_NEWNET, CLONE_NEWPID) for render/JS processes.
  * Define strict seccomp-bpf filters to prevent child processes from calling forbidden syscalls (e.g., `socket()`, `open()` outside of whitelisted paths).
- [x] **Task 4.2 - Secure Storage Context:**
  * Implement in-memory ephemeral storage models for the `Tor` context.

## Phase 5: Hardware Paint Acceleration & GUI
**Goal:** Replace the terminal/mock renderer with a high-performance cross-platform GUI.
- [x] **Task 5.1 - GPU Backend (`pkg/render`):**
  * Integrate raw OpenGL and GLFW bindings to implement a hardware-accelerated immediate mode GUI.
  * Implement hardware-accelerated draw calls for the `TargetCanvas` interface.
- [x] **Task 5.2 - Tab Management:**
  * Build multi-tab UI handling, mapping each tab to its dedicated render/JS processes.

## Phase 6: Interactive UI & Text Rendering (Future)
**Goal:** Upgrade the mock hardware renderer to handle real text rendering and user input.
- [x] **Task 6.1 - Text Rasterization (`pkg/render`):**
  * Integrate `freetype` or standard Go font rendering libraries (`golang.org/x/image/font`).
  * Translate text strings into OpenGL texture atlases for hardware acceleration.
- [x] **Task 6.2 - Event Handling (`cmd/gcc-browser`):**
  * Capture keyboard and mouse events from the GLFW window.
  * Implement hit-testing to map screen coordinates to DOM nodes.
- [x] **Task 6.3 - JS Event Bridge (`internal/ipc`):**
  * Forward user interactions via gRPC to the JS Daemon to trigger DOM event listeners (e.g., `onclick`).

## Phase 7: End-to-End Pipeline & Standard Layouts (Future)
**Goal:** Connect the isolated daemons into a unified web browsing flow and support standard W3C layouts.
- [x] **Task 7.1 - The Navigation Pipeline:**
  * Implement the core flow: User types URL -> Orchestrator commands Network Daemon -> Network returns HTML -> Orchestrator sends HTML to Parser Daemon -> Parser returns DOM/CSSOM -> Orchestrator sends to Render Daemon -> Render paints to OpenGL.
- [x] **Task 7.2 - W3C Layout Engine Basics (`pkg/render`):**
  * Replace the rudimentary layout algorithm with standard block and inline formatting contexts.
  * Add support for basic Flexbox structures.
- [x] **Task 7.3 - Resource Fetching:**
  * Implement recursive asset fetching (images, linked stylesheets) during the HTML parsing phase.

## Phase 8: Advanced UI & Image Rendering (Future)
**Goal:** Enhance the browser's graphical capabilities and prepare the architecture for multi-tab environments.
- [x] **Task 8.1 - Image Rasterization & Rendering (`pkg/render`):**
  * Extend Orchestrator to fetch image assets (`<img>` tags) via the Network Daemon.
  * Decode PNG/JPEG payloads and map them to OpenGL Textures.
  * Update `DrawImage` to render texture quads accurately.
- [x] **Task 8.2 - Tab UI Management (`cmd/gcc-browser`):**
  * Implement an OpenGL-based Tab bar allowing context switching between distinct DOM/CSSOM environments.
  * Handle active tab state routing for rendering and JS events.
- [x] **Task 8.3 - Parallel Layout Computation (`pkg/render`):**
  * Introduce Goroutines inside the layout tree traversal to calculate dimensions of disjoint DOM branches concurrently.

## Phase 9: Keyboard Input & Real URL Navigation (Future)
**Goal:** Transition the browser from a static mock pipeline to a dynamic, interactive web client capable of free-form navigation.
- [x] **Task 9.1 - Keyboard Event Handling (`pkg/render`):**
  * Hook into GLFW character and key callbacks to capture text input.
  * Map input events to the Orchestrator's active context.
- [x] **Task 9.2 - Omnibox (URL Bar) Navigation (`cmd/gcc-browser`):**
  * Manage URL input state and caret behavior.
  * Trigger real-world end-to-end `Fetch -> Parse -> Render` pipelines via the IPC daemons when the user presses Enter.

## Phase 10: JavaScript DOM API Bridge (Future)
**Goal:** Enable dynamic web interactivity by bridging the isolated JavaScript sandbox with the Orchestrator's DOM layout tree.
- [x] **Task 10.1 - JavaScript DOM Bindings (`pkg/javascript`):**
  * Inject standard `document` objects into the Goja VM.
  * Implement `document.getElementById` and element style mutation trackers.
- [x] **Task 10.2 - Bidirectional IPC via Polling (`api/`, `internal/ipc`):**
  * Define `GetDOMMutations` RPC to extract pending DOM changes from the JS Daemon without complex bidirectional streaming.
- [x] **Task 10.3 - Rendering Updates (`cmd/gcc-browser`):**
  * Poll the JS Daemon during the OpenGL loop.
  * Apply received mutations to the Orchestrator's DOMTree, mark the context as dirty, and trigger a hardware repaint.

## Phase 11: Real-time WebSockets over IPC (Future)
**Goal:** Implement full-duplex communication protocols bridging the isolated JS sandbox to external servers via the Network daemon.
- [x] **Task 11.1 - Bidirectional Streaming IPC (`api/network.proto`):**
  * Define `OpenWebSocket` as a bidirectional gRPC stream to pass frames back and forth.
- [x] **Task 11.2 - Network Daemon Implementation (`pkg/network`):**
  * Utilize `golang.org/x/net/websocket` to handle the RFC 6455 protocol to the outside world.
- [x] **Task 11.3 - JS WebSocket API (`pkg/javascript`):**
  * Bind a `WebSocket` object into the Goja environment, allowing scripts to call `.send()` and trigger `.onmessage()` callbacks.

## Phase 12: WebAssembly (WASM) Execution (Future)
**Goal:** Enhance the isolated scripting environment to execute high-performance binary WASM modules alongside JavaScript.
- [x] **Task 12.1 - WASM Engine Integration (`pkg/javascript`):**
  * Integrate `tetratelabs/wazero` to provide a pure Go WebAssembly runtime.
  * Bridge the Goja JS VM with Wazero, allowing JavaScript to instantiate and call exported WASM functions.
