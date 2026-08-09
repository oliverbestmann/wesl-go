package wesl

import (
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/dop251/goja"
)

//go:embed wesl.js
var weslSource string

var Stdout io.Writer = nil

type Options struct {
	// Files to present to the wesl transpiler
	Files Files

	// Constants to inject into the transpiler.
	// Value must be either int, float32 or string
	Constants map[string]any

	// Conditions for @if etc
	Conditions map[string]bool

	// PackageName for the local sources (in addition to default "package::").
	// Enables imports like `import mypkg::foo` alongside `import package::foo`.
	// Should be a valid WGSL identifier (use underscores, not hyphens or slashes).
	PackageName string

	// Root module name: file path (`./main.wesl`), module path (`package::main`), or bare name (`main`).
	// For apps, the root module contains entry points (`@compute`, `@vertex`, `@fragment`).
	// For libraries, it defines the public API.
	RootModuleName string
}

// Files describe an in-memory file system that contains all files
// available to the transpiler.
type Files map[string]string

func FilesOf(fsys fs.FS) (Files, error) {
	files := Files{}

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(path, ".wesl") && !strings.HasSuffix(path, ".wgsl") {
			return nil
		}

		src, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read shader file %q: %w", path, err)
		}

		files[path] = string(src)
		return nil
	})

	return files, err
}

type Transpiler struct {
	vm   *goja.Runtime
	link goja.Callable
}

func New() *Transpiler {
	vm := goja.New()

	registerConsole(vm)
	// registerRegExpHack(vm)

	// create the link function
	link, ok := goja.AssertFunction(must(vm.RunString(linkWrapper)))
	if !ok {
		panic("link is not a function")
	}

	// parse the wesl.js source
	must(vm.RunString(weslSource))

	return &Transpiler{vm: vm, link: link}
}

func (t *Transpiler) Transpile(entrypoint string, opts Options) (string, error) {
	params := map[string]any{
		"weslSrc":        opts.Files,
		"constants":      opts.Constants,
		"conditions":     opts.Conditions,
		"rootModuleName": entrypoint,
	}

	vm := t.vm

	// transpile
	result, err := t.link(vm.GlobalObject(), vm.ToValue(params))
	if err != nil {
		return "", fmt.Errorf("transpile: %w", err)
	}

	// wait for the promise to finish
	result = result.Export().(*goja.Promise).Result()

	// check for an error
	if errorStr := result.ToObject(vm).Get("error"); errorStr != nil {
		return "", fmt.Errorf("transpiling failed: %s", errorStr)
	}

	// now extract the wgsl code
	output := result.ToObject(vm).Get("dest").String()
	return output, nil
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

const linkWrapper = `
	var exports = {};

	(function _link(opts) {
		return exports.link(opts).catch(function (err) {
			return {error: err};
		});
	})
`

func registerRegExpHack(vm *goja.Runtime) {
	_, err := vm.RunString(`
		(function (global) {
			var OriginalRegExp = global.RegExp;

			global.RegExp = function (pattern, flags) {
				if (flags) {
					// this flag is not supported by goja
					flags = flags.replace("d", "");
				}
				return new OriginalRegExp(pattern, flags);
			};
		}(this));
	`)

	if err != nil {
		panic(err)
	}
}

func registerConsole(vm *goja.Runtime) {
	print := func(values ...goja.Value) {
		if Stdout == nil {
			return
		}

		var anyValues []any
		for _, value := range values {
			anyValues = append(anyValues, value)
		}

		_, _ = fmt.Fprintln(Stdout, anyValues...)
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
