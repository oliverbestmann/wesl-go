//go:build js

package wesl

import (
	"syscall/js"
	"testing"

	"github.com/stretchr/testify/require"
)

// Transpile must not block on the JS event loop, or calling it from an event
// handler deadlocks.
func TestTranspile_InsideJSCallback(t *testing.T) {
	tr := New()

	var out string
	var err error

	callback := js.FuncOf(func(this js.Value, args []js.Value) any {
		out, err = tr.Transpile("main.wesl", Options{
			Files: Files{"main.wesl": "@compute @workgroup_size(1) fn main() {}"},
		})
		return nil
	})
	defer callback.Release()

	// invoking through JS runs the Go function as a JS callback
	js.Global().Get("Reflect").Call("apply", callback, js.Undefined(), js.Global().Get("Array").New())

	require.NoError(t, err)
	require.Contains(t, out, "fn main")
}
