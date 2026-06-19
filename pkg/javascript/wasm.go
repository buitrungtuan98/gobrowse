package javascript

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/dop251/goja"
	"github.com/tetratelabs/wazero"
)

// WasmWrapper provides the WebAssembly global object to JavaScript.
type WasmWrapper struct {
	engine  *GojaEngine
	runtime wazero.Runtime
}

// InjectWasmEngine binds the WebAssembly.instantiate function into the JS VM.
func (e *GojaEngine) InjectWasmEngine() {
	wrapper := &WasmWrapper{
		engine:  e,
		runtime: wazero.NewRuntime(context.Background()),
	}

	// Create a WebAssembly namespace object
	wasmObj := e.vm.NewObject()

	// Expose WebAssembly.instantiate(base64Str)
	// Note: In real browsers it accepts ArrayBuffer/Promise, but for this POC we accept Base64 strings.
	err := wasmObj.Set("instantiate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(e.vm.ToValue("TypeError: instantiate requires a base64 wasm string"))
		}

		b64Str := call.Arguments[0].String()
		wasmBytes, err := base64.StdEncoding.DecodeString(b64Str)
		if err != nil {
			panic(e.vm.ToValue(fmt.Sprintf("CompileError: Invalid base64 wasm binary: %v", err)))
		}

		// Compile and Instantiate WASM module
		mod, err := wrapper.runtime.Instantiate(context.Background(), wasmBytes)
		if err != nil {
			panic(e.vm.ToValue(fmt.Sprintf("CompileError: Wasm instantiation failed: %v", err)))
		}

		// Build the Instance object containing exported functions
		instanceObj := e.vm.NewObject()
		exportsObj := e.vm.NewObject()

		// Map exported WASM functions to Goja JS functions
		for name, _ := range mod.ExportedFunctionDefinitions() {
			fn := mod.ExportedFunction(name)

			// We define a JS closure that proxies arguments to Wazero
			funcName := name
			exportsObj.Set(funcName, func(fc goja.FunctionCall) goja.Value {
				// Convert JS arguments to uint64 for wazero
				args := make([]uint64, len(fc.Arguments))
				for i, arg := range fc.Arguments {
					args[i] = uint64(arg.ToInteger())
				}

				results, err := fn.Call(context.Background(), args...)
				if err != nil {
					panic(e.vm.ToValue(fmt.Sprintf("RuntimeError: Wasm function trap: %v", err)))
				}

				if len(results) > 0 {
					return e.vm.ToValue(results[0]) // Return primary result
				}
				return goja.Undefined()
			})
		}

		instanceObj.Set("exports", exportsObj)

		// Return a mock Promise-like result: { instance: InstanceObject }
		resultObj := e.vm.NewObject()
		resultObj.Set("instance", instanceObj)

		return resultObj
	})

	if err == nil {
		e.vm.Set("WebAssembly", wasmObj)
	}
}
