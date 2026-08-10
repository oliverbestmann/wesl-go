# wesl-go

Transpile [WESL](https://wesl-lang.dev/) shaders to WGSL from Go. The transpiler runs the
official `wesl` JavaScript implementation embedded via the [goja](https://github.com/dop251/goja)
JavaScript runtime, so no Node.js is required at runtime.

## Installation

```sh
go get github.com/oliverbestmann/wesl-go
```

## Usage

Create a transpiler with `New()`, provide your shader files, and call `Transpile` with the
name of the entrypoint file:

```go
package main

import (
	"fmt"

	wesl "github.com/oliverbestmann/wesl-go"
)

func main() {
	tr := wesl.New()

	out, err := tr.Transpile("main.wesl", wesl.Options{
		Files: wesl.Files{
			"main.wesl":    "import package::util::scale;\n// ...",
			"util.wesl":    "fn scale(x: f32) -> f32 { return x * 2.0; }",
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(out) // WGSL output
}
```

### Loading files from an `fs.FS`

`FilesOf` walks a filesystem and collects every `.wesl` and `.wgsl` file, which is handy with
`embed`:

```go
//go:embed shaders
var shaders embed.FS

sub, _ := fs.Sub(shaders, "shaders")

files, err := wesl.FilesOf(sub)
if err != nil {
	panic(err)
}

out, err := tr.Transpile("main.wesl", wesl.Options{Files: files})
```

## Options

`Transpile` takes the entrypoint file name plus an `Options` struct:

| Field | Type | Description |
| --- | --- | --- |
| `Files` | `Files` (`map[string]string`) | In-memory file system of shader sources available to the transpiler. |
| `Constants` | `map[string]any` | Constants injected into the transpiler. Each value must be an `int`, `float32`, or `string`. |
| `Conditions` | `map[string]bool` | Conditions used for `@if` and similar conditional compilation. |
| `PackageName` | `string` | Optional package name for the local sources (in addition to the default `package::`), enabling imports like `import mypkg::foo`. Must be a valid WGSL identifier. |
| `Libs` | `[]Bundle` | Library bundles available for the link (see below). |

Example with constants and conditions:

```go
out, err := tr.Transpile("main.wesl", wesl.Options{
	Files: files,
	Constants: map[string]any{
		"Foobar": 123,
	},
	Conditions: map[string]bool{
		"MOBILE": true,
	},
})
```

## Library bundles

Libraries are supplied as `Bundle` values via `Options.Libs`. A bundle groups a set of modules
under a package name and may declare transitive `Dependencies`:

```go
out, err := tr.Transpile("main.wesl", wesl.Options{
	Files: wesl.Files{
		"main.wesl": "import mylib::double;\n// ...",
	},
	Libs: []wesl.Bundle{
		{
			Name:    "mylib",
			Edition: "2026_pre",
			Modules: map[string]string{
				"./lib.wesl": "fn double(x: f32) -> f32 { return x * 2.0; }",
			},
		},
	},
})
```

## Console output

The embedded transpiler may call `console.log` and friends. By default this output is discarded.
`Stdout` is an `atomic.Pointer[io.Writer]`; store a writer into it to capture the output:

```go
var w io.Writer = os.Stdout
wesl.Stdout.Store(new(w))
```

## Updating the embedded transpiler

The bundled `wesl` JavaScript is embedded from `wesl.js`. To regenerate it, run:

```sh
js/prepare-wesl.sh
```

This installs the pinned dependencies, bundles `js/index.js` with esbuild, and copies the result
to `wesl.js`.

## Notes

- `New()` panics if the embedded JavaScript fails to load. Create the transpiler once and reuse it.
- The transpiler is not safe for concurrent use; create one per goroutine or guard it with a mutex.
- The implementation may switch to [wesl-rs](https://github.com/wgsl-tooling-wg/wesl-rs) compiled to
  WASM in the future, but this is not certain yet.
