//go:build js

package wesl

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

func newEngine() engine {
	return newBrowserEngine()
}

// browserEngine runs the wesl JavaScript in the host JavaScript runtime
// (browser or node) via syscall/js. Console output goes to the host console;
// Stdout is not used.
type browserEngine struct {
	linkFn js.Value
}

// newBrowserEngine evaluates the embedded wesl JavaScript in the host
// runtime. It panics if the embedded source fails to load.
func newBrowserEngine() *browserEngine {
	// wesl.js is a CommonJS bundle, run it with its own module object.
	// Needs 'unsafe-eval' if the page sets a Content-Security-Policy.
	module := js.Global().Get("Object").New()
	js.Global().Get("Function").New("module", weslSource).Invoke(module)

	link := module.Get("exports").Get("link")
	if link.Type() != js.TypeFunction {
		panic("link is not a function")
	}

	return &browserEngine{linkFn: link}
}

func (e *browserEngine) link(params map[string]any) (dest string, err error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("encode params: %w", err)
	}

	// js.Value.Invoke panics with a js.Error if link throws
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		jsErr, ok := r.(js.Error)
		if !ok {
			panic(r)
		}

		err = fmt.Errorf("transpiling failed: %w", jsErr)
	}()

	opts := js.Global().Get("JSON").Call("parse", string(paramsJSON))
	return e.linkFn.Invoke(opts).String(), nil
}
