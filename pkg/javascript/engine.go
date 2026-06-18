package javascript

import (
	"fmt"

	"github.com/dop251/goja"
)

// GojaEngine is the concrete implementation of gcc.JSEngine using the goja VM.
type GojaEngine struct {
	vm *goja.Runtime
}

// NewGojaEngine initializes a new JavaScript sandbox environment.
func NewGojaEngine() *GojaEngine {
	return &GojaEngine{
		vm: goja.New(),
	}
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
