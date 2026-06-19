package javascript

import (
	"fmt"

	"github.com/dop251/goja"
	"sync"
)

// GojaEngine is the concrete implementation of gcc.JSEngine using the goja VM.
type GojaEngine struct {
	vm            *goja.Runtime
	mu            sync.Mutex
	mutationQueue []DOMMutation
}

// NewGojaEngine initializes a new JavaScript sandbox environment.
func NewGojaEngine() *GojaEngine {
	engine := &GojaEngine{
		vm:            goja.New(),
		mutationQueue: make([]DOMMutation, 0),
	}

	// Inject the global document object
	engine.BindGlobalAPI("document", &DocumentWrapper{engine: engine})

	return engine
}

func (e *GojaEngine) queueMutation(nodeID, prop, val string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mutationQueue = append(e.mutationQueue, DOMMutation{
		NodeID:   nodeID,
		Property: prop,
		Value:    val,
	})
}

// FlushMutations retrieves and clears the pending DOM changes.
func (e *GojaEngine) FlushMutations() []DOMMutation {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.mutationQueue) == 0 {
		return nil
	}

	copied := make([]DOMMutation, len(e.mutationQueue))
	copy(copied, e.mutationQueue)
	e.mutationQueue = e.mutationQueue[:0]
	return copied
}

// ExecuteScript runs an arbitrary block of JavaScript code inside the VM.
func (e *GojaEngine) ExecuteScript(script string) (interface{}, error) {
	val, err := e.vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	return val.Export(), nil
}

// BindGlobalAPI injects a Go struct or function into the global JavaScript object under a specific name.
// This allows bridging WebAPIs (like `window` or `document`) or custom crypto providers into the sandbox.
func (e *GojaEngine) BindGlobalAPI(name string, handler interface{}) error {
	err := e.vm.Set(name, handler)
	if err != nil {
		return fmt.Errorf("failed to bind global api %s: %w", name, err)
	}
	return nil
}

// DispatchEvent simulates firing a DOM event (e.g. click, keypress) into the JS context.
func (e *GojaEngine) DispatchEvent(nodeID string, eventType string, payload string) error {
	// For this milestone, we log the event and execute a mock JS handler if it exists globally.
	// In a full implementation, this routes through a synthetic DOM event dispatcher.
	fmt.Printf("[JSEngine] Event Dispatched -> Node: %s | Type: %s | Payload: %s\n", nodeID, eventType, payload)

	// Try to execute a global mock handler: `onEvent(nodeId, type, payload)`
	script := fmt.Sprintf("if (typeof window !== 'undefined' && typeof window.onEvent === 'function') { window.onEvent('%s', '%s', '%s'); }", nodeID, eventType, payload)
	_, _ = e.vm.RunString(script)

	return nil
}

// DOMMutation represents a change requested by JS.
type DOMMutation struct {
	NodeID   string
	Property string
	Value    string
}

// ElementWrapper allows JS to interact with a specific DOM Node.
type ElementWrapper struct {
	engine *GojaEngine
	id     string
}

// SetAttribute simulates element.setAttribute("style", ...) or element.color = ...
func (e *ElementWrapper) SetAttribute(prop, val string) {
	e.engine.queueMutation(e.id, prop, val)
}

// DocumentWrapper represents the global `document` object in JS.
type DocumentWrapper struct {
	engine *GojaEngine
}

func (d *DocumentWrapper) GetElementById(id string) *ElementWrapper {
	return &ElementWrapper{
		engine: d.engine,
		id:     id,
	}
}

// WSSender represents a function closure to send websocket payloads
type WSSender func(payload string)

// WebSocketProvider bridges JS creation of websockets over to the IPC adapter
type WebSocketProvider interface {
	OpenWebSocket(url string, onMessage func(string), onClose func()) (func(string), error)
}

// WebSocketWrapper represents the `WebSocket` class in JS.
type WebSocketWrapper struct {
	engine   *GojaEngine
	provider WebSocketProvider
	sendFunc WSSender

	// JS Callbacks
	Onmessage func(interface{}) `goja:"onmessage"`
	Onclose   func()            `goja:"onclose"`
}

func (w *WebSocketWrapper) Send(data string) {
	if w.sendFunc != nil {
		w.sendFunc(data)
	}
}

// InjectWebSocketFactory gives the VM a `createWebSocket(url)` global function
func (e *GojaEngine) InjectWebSocketFactory(provider WebSocketProvider) {
	e.BindGlobalAPI("createWebSocket", func(url string) *WebSocketWrapper {
		wrapper := &WebSocketWrapper{
			engine:   e,
			provider: provider,
		}

		sender, err := provider.OpenWebSocket(url, func(msg string) {
			// Trigger JS callback
			if wrapper.Onmessage != nil {
				// Execute callback safely in JS VM context
				// Note: in full implementation, this should be dispatched on the main JS event loop
				// to avoid concurrent map read/write in goja runtime.
				wrapper.Onmessage(msg)
			}
		}, func() {
			if wrapper.Onclose != nil {
				wrapper.Onclose()
			}
		})

		if err == nil {
			wrapper.sendFunc = sender
		}

		return wrapper
	})
}
