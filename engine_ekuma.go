//go:build !js

package wesl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hajimehoshi/ekuma"
)

func newEngine() engine {
	return newEkumaEngine()
}

// ekumaEngine runs the wesl JavaScript in the ekuma runtime.
type ekumaEngine struct {
	vm        *ekuma.VM
	jsonParse ekuma.Value
	linkFn    ekuma.Value
}

// newEkumaEngine creates an ekuma VM and loads the embedded wesl
// JavaScript. It panics if the embedded source fails to load.
func newEkumaEngine() *ekumaEngine {
	ctx := context.Background()
	vm := new(ekuma.VM)

	// console.* forwards to Stdout
	print := vm.FuncOf("log", func(this ekuma.Value, args []ekuma.Value) (any, error) {
		w := Stdout.Load()
		if w == nil {
			return nil, nil
		}

		var values []any
		for _, arg := range args {
			values = append(values, arg.String())
		}

		_, _ = fmt.Fprintln(*w, values...)
		return nil, nil
	})

	console := map[string]any{"log": print, "info": print, "warn": print, "error": print}
	if err := vm.Global().Set(ctx, "console", console); err != nil {
		panic(err)
	}

	// wesl.js is a CommonJS bundle, run it with its own module object
	script := must(vm.ParseScript("wesl.js", "(function(module) {\n"+weslSource+"\n})"))
	wrapper := must(script.Evaluate(ctx))

	module := vm.ValueOf(map[string]any{})
	must(wrapper.Invoke(ctx, module))

	exports := must(module.Get(ctx, "exports"))
	link := must(exports.Get(ctx, "link"))
	if link.Type() != ekuma.TypeFunction {
		panic("link is not a function")
	}

	jsonParse := must(must(vm.Global().Get(ctx, "JSON")).Get(ctx, "parse"))

	return &ekumaEngine{vm: vm, jsonParse: jsonParse, linkFn: link}
}

func (e *ekumaEngine) link(params map[string]any) (string, error) {
	ctx := context.Background()

	// ValueOf only takes map[string]any and []any, go through JSON like the browser engine
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("encode params: %w", err)
	}

	opts, err := e.jsonParse.Invoke(ctx, string(paramsJSON))
	if err != nil {
		return "", fmt.Errorf("decode params: %w", err)
	}

	result, err := e.linkFn.Invoke(ctx, opts)
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
