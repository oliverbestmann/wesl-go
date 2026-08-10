// Package wesl embeds the official WESL JavaScript transpiler and runs it via
// the goja JavaScript runtime, letting Go code link WESL/WGSL sources into a
// single WGSL string without a Node.js dependency.
package wesl

import (
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync/atomic"

	"github.com/dop251/goja"
)

// weslSource holds the bundled wesl JavaScript, embedded at build time.
//
//go:embed wesl.js
var weslSource string

// Stdout, if set, receives console output (console.log/info/warn/error) from
// the embedded transpiler. When nil (the default) that output is discarded.
var Stdout atomic.Pointer[io.Writer]

// Options configures a single Transpile call.
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

	// Libs are library bundles available for the link.
	Libs []Bundle
}

// Bundle is a bundle of WESL/WGSL modules that can be provided to the
// transpiler as a library.
type Bundle struct {
	// Name is the npm package name sanitized to be a valid WESL identifier
	// (@ removed, / => __, - => _).
	Name string

	// Edition is the WESL edition of the code, e.g. "2026_pre".
	Edition string

	// Modules maps file paths (relative to the package root, e.g. "./lib.wgsl")
	// to WESL/WGSL code strings.
	Modules map[string]string

	// Dependencies are the packages referenced by this package.
	Dependencies []Bundle
}

// Files describe an in-memory file system that contains all files
// available to the transpiler.
type Files map[string]string

// FilesOf walks fsys and collects every .wesl and .wgsl file into a Files map,
// keyed by path. Other files are ignored. It is a convenience for building the
// Options.Files input from an embed.FS or on-disk directory.
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

// Transpiler links WESL/WGSL sources into WGSL. A Transpiler owns a JavaScript
// runtime and is not safe for concurrent use; create one per goroutine or
// guard it with a mutex. It can be reused across many Transpile calls.
type Transpiler struct {
	vm   *goja.Runtime
	link goja.Callable
}

// New creates a Transpiler by spinning up a goja runtime and loading the
// embedded wesl JavaScript. It panics if the embedded source fails to load;
// since that only happens on a broken build, reuse a single Transpiler rather
// than creating one per call.
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

// Transpile links the module graph rooted at entrypoint (a file name such as
// "main.wesl") into a single WGSL string, pulling in referenced local files
// and library bundles. Dead code outside the root is dropped. It returns an
// error if parsing or linking fails.
func (t *Transpiler) Transpile(entrypoint string, opts Options) (string, error) {
	params := map[string]any{
		"weslSrc":        opts.Files,
		"rootModuleName": entrypoint,
	}

	if opts.Constants != nil {
		params["constants"] = opts.Constants
	}

	if opts.Conditions != nil {
		params["conditions"] = opts.Conditions
	}

	if opts.PackageName != "" {
		params["packageName"] = opts.PackageName
	}

	if opts.Libs != nil {
		libs := make([]any, len(opts.Libs))
		for idx, lib := range opts.Libs {
			libs[idx] = lib.toJS()
		}
		params["libs"] = libs
	}

	// transpile
	result, err := t.link(t.vm.GlobalObject(), t.vm.ToValue(params))
	if err != nil {
		return "", fmt.Errorf("transpile: %w", err)
	}

	// wait for the promise to finish
	result = result.Export().(*goja.Promise).Result()

	// check for an error
	if errorStr := result.ToObject(t.vm).Get("error"); errorStr != nil {
		return "", fmt.Errorf("transpiling failed: %s", errorStr)
	}

	// now extract the wgsl code
	output := result.ToObject(t.vm).Get("dest").String()
	return output, nil
}

// toJS converts the bundle into the goja-compatible map shape expected by the
// wesl JavaScript library's WeslBundle.
func (b Bundle) toJS() map[string]any {
	js := map[string]any{
		"name":    b.Name,
		"edition": b.Edition,
		"modules": b.Modules,
	}

	if b.Dependencies != nil {
		deps := make([]any, len(b.Dependencies))
		for idx, dep := range b.Dependencies {
			deps[idx] = dep.toJS()
		}
		js["dependencies"] = deps
	}

	return js
}

// must panics on error, returning the value otherwise. Used for setup steps
// that can only fail on a broken build.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// linkWrapper is the JavaScript glue evaluated once at startup. It exposes a
// callable that invokes the bundle's exported link() and normalizes rejected
// promises into a resolved {error} object, so Go only has to inspect the
// result value.
const linkWrapper = `
	var module = {};

	(function _link(opts) {
		return module.exports.link(opts).catch(function (err) {
			return {error: err};
		});
	})
`

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
