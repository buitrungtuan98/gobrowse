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
- [ ] **Task 6.1 - Text Rasterization (`pkg/render`):**
  * Integrate `freetype` or standard Go font rendering libraries (`golang.org/x/image/font`).
  * Translate text strings into OpenGL texture atlases for hardware acceleration.
- [ ] **Task 6.2 - Event Handling (`cmd/gcc-browser`):**
  * Capture keyboard and mouse events from the GLFW window.
  * Implement hit-testing to map screen coordinates to DOM nodes.
- [ ] **Task 6.3 - JS Event Bridge (`internal/ipc`):**
  * Forward user interactions via gRPC to the JS Daemon to trigger DOM event listeners (e.g., `onclick`).

## Phase 7: End-to-End Pipeline & Standard Layouts (Future)
**Goal:** Connect the isolated daemons into a unified web browsing flow and support standard W3C layouts.
- [ ] **Task 7.1 - The Navigation Pipeline:**
  * Implement the core flow: User types URL -> Orchestrator commands Network Daemon -> Network returns HTML -> Orchestrator sends HTML to Parser Daemon -> Parser returns DOM/CSSOM -> Orchestrator sends to Render Daemon -> Render paints to OpenGL.
- [ ] **Task 7.2 - W3C Layout Engine Basics (`pkg/render`):**
  * Replace the rudimentary layout algorithm with standard block and inline formatting contexts.
  * Add support for basic Flexbox structures.
- [ ] **Task 7.3 - Resource Fetching:**
  * Implement recursive asset fetching (images, linked stylesheets) during the HTML parsing phase.
