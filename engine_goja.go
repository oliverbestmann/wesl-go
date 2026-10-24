//go:build !js

package wesl

import (
	"fmt"

	"github.com/dop251/goja"
)

func newEngine() engine {
	return newGojaEngine()
}

// gojaEngine runs the wesl JavaScript in the goja runtime.
type gojaEngine struct {
	vm     *goja.Runtime
	linkFn goja.Callable
}

// newGojaEngine spins up a goja runtime and loads the embedded wesl
// JavaScript. It panics if the embedded source fails to load.
func newGojaEngine() *gojaEngine {
	vm := goja.New()

	registerConsole(vm)

	// wesl.js is a CommonJS bundle, give it a module object to export into
	module := vm.NewObject()
	if err := vm.Set("module", module); err != nil {
		panic(err)
	}

	must(vm.RunString(weslSource))

	link, ok := goja.AssertFunction(module.Get("exports").ToObject(vm).Get("link"))
	if !ok {
		panic("link is not a function")
	}

	return &gojaEngine{vm: vm, linkFn: link}
}

func (e *gojaEngine) link(params map[string]any) (string, error) {
	result, err := e.linkFn(goja.Undefined(), e.vm.ToValue(params))
	if err != nil {
		return "", fmt.Errorf("transpiling failed: %w", err)
	}

	return result.String(), nil
}

// must panics on error, returning the value otherwise. Used for setup steps
// that can only fail on a broken build.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// registerConsole installs a console object whose log/info/warn/error methods
// forward to the package-level Stdout (or discard output when it is nil).
func registerConsole(vm *goja.Runtime) {
	print := func(values ...goja.Value) {
		w := Stdout.Load()
		if w == nil {
			return
		}

		var anyValues []any
		for _, value := range values {
			anyValues = append(anyValues, value)
		}

		_, _ = fmt.Fprintln(*w, anyValues...)
	}

	err := vm.Set("console", map[string]any{
		"log":   print,
		"info":  print,
		"warn":  print,
		"error": print,
	})
	if err != nil {
		panic(err)
	}
}
