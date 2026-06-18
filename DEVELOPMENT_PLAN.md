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
- [ ] **Task 4.1 - Seccomp/Namespaces (`internal/sandbox`):**
  * Implement Linux namespace separation (CLONE_NEWNET, CLONE_NEWPID) for render/JS processes.
  * Define strict seccomp-bpf filters to prevent child processes from calling forbidden syscalls (e.g., `socket()`, `open()` outside of whitelisted paths).
- [ ] **Task 4.2 - Secure Storage Context:**
  * Implement in-memory ephemeral storage models for the `Tor` context.

## Phase 5: Hardware Paint Acceleration & GUI
**Goal:** Replace the terminal/mock renderer with a high-performance cross-platform GUI.
- [ ] **Task 5.1 - GPU Backend (`pkg/render`):**
  * Integrate `gioui.org` (Gio) or `fyne` for cross-platform desktop windows.
  * Implement hardware-accelerated draw calls for the `TargetCanvas` interface.
- [ ] **Task 5.2 - Tab Management:**
  * Build multi-tab UI handling, mapping each tab to its dedicated render/JS processes.
