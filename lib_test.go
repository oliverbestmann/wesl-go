package wesl

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

//go:embed test/shaders
var shaders embed.FS

func fstestFS() fs.FS {
	return fstest.MapFS{
		"a.wesl":     {Data: []byte("fn a() {}")},
		"sub/b.wgsl": {Data: []byte("fn b() {}")},
		"notes.txt":  {Data: []byte("ignore me")},
	}
}

func TestTranspiler_Transpile(t *testing.T) {
	var w io.Writer = os.Stdout
	Stdout.Store(&w)

	shaders, _ := fs.Sub(shaders, "test/shaders")

	files, err := FilesOf(shaders)
	require.NoError(t, err)

	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: files,
		Constants: map[string]any{
			"Foobar": 123,
		},
		Conditions: map[string]bool{
			"MOBILE": true,
		},
	})
	require.NoError(t, err)

	fmt.Println(out)
}

func TestTranspile_Simple(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "@compute @workgroup_size(1) fn main() { let x = 1.0; }",
		},
	})
	require.NoError(t, err)
	require.Contains(t, out, "fn main")
}

func TestTranspile_LocalImport(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "import package::util::double;\n@compute @workgroup_size(1) fn main() { let x = double(2.0); }",
			"util.wesl": "fn double(x: f32) -> f32 { return x * 2.0; }",
		},
	})
	require.NoError(t, err)
	require.Contains(t, out, "return x * 2.0")
}

func TestTranspile_Constants(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "import constants::Answer;\n@compute @workgroup_size(1) fn main() { let x = Answer; }",
		},
		Constants: map[string]any{
			"Answer": 42,
		},
	})
	require.NoError(t, err)
	require.Contains(t, out, "42")
}

func TestTranspile_Conditions(t *testing.T) {
	src := "@compute @workgroup_size(1) fn main() {\n  @if(MOBILE) let mobile = true;\n  @if(!MOBILE) let desktop = true;\n}"

	tr := New()

	mobile, err := tr.Transpile("main.wesl", Options{
		Files:      Files{"main.wesl": src},
		Conditions: map[string]bool{"MOBILE": true},
	})
	require.NoError(t, err)
	require.Contains(t, mobile, "mobile")
	require.NotContains(t, mobile, "desktop")

	desktop, err := tr.Transpile("main.wesl", Options{
		Files:      Files{"main.wesl": src},
		Conditions: map[string]bool{"MOBILE": false},
	})
	require.NoError(t, err)
	require.Contains(t, desktop, "desktop")
	require.NotContains(t, desktop, "mobile")
}

func TestTranspile_PackageName(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "import mypkg::util::triple;\n@compute @workgroup_size(1) fn main() { let x = triple(2.0); }",
			"util.wesl": "fn triple(x: f32) -> f32 { return x * 3.0; }",
		},
		PackageName: "mypkg",
	})
	require.NoError(t, err)
	require.Contains(t, out, "return x * 3.0")
}

func TestTranspile_Bundle(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "import mylib::double;\n@compute @workgroup_size(1) fn main() { let x = double(2.0); }",
		},
		Libs: []Bundle{
			{
				Name:    "mylib",
				Edition: "2026_pre",
				Modules: map[string]string{
					"./lib.wesl": "fn double(x: f32) -> f32 { return x * 2.0; }",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, out, "return x * 2.0")
}

func TestTranspile_BundleWithDependency(t *testing.T) {
	tr := New()

	out, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "import top::entry;\n@compute @workgroup_size(1) fn main() { let x = entry(2.0); }",
		},
		Libs: []Bundle{
			{
				Name:    "top",
				Edition: "2026_pre",
				Modules: map[string]string{
					"./lib.wesl": "import base::square;\nfn entry(x: f32) -> f32 { return square(x); }",
				},
				Dependencies: []Bundle{
					{
						Name:    "base",
						Edition: "2026_pre",
						Modules: map[string]string{
							"./lib.wesl": "fn square(x: f32) -> f32 { return x * x; }",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, out, "return x * x")
}

func TestTranspile_ParseError(t *testing.T) {
	tr := New()

	_, err := tr.Transpile("main.wesl", Options{
		Files: Files{
			"main.wesl": "fn main( { this is not valid wesl",
		},
	})
	require.Error(t, err)
}

func TestTranspile_UnknownEntrypoint(t *testing.T) {
	tr := New()

	_, err := tr.Transpile("missing.wesl", Options{
		Files: Files{
			"main.wesl": "@compute @workgroup_size(1) fn main() {}",
		},
	})
	require.Error(t, err)
}

func TestTranspile_Reusable(t *testing.T) {
	tr := New()

	for range 3 {
		out, err := tr.Transpile("main.wesl", Options{
			Files: Files{
				"main.wesl": "@compute @workgroup_size(1) fn main() {}",
			},
		})
		require.NoError(t, err)
		require.Contains(t, out, "fn main")
	}
}

func TestFilesOf(t *testing.T) {
	files, err := FilesOf(fstestFS())
	require.NoError(t, err)

	require.Contains(t, files, "a.wesl")
	require.Contains(t, files, "sub/b.wgsl")
	require.NotContains(t, files, "notes.txt")
}

func TestBundle_toJS(t *testing.T) {
	b := Bundle{
		Name:    "top",
		Edition: "2026_pre",
		Modules: map[string]string{"./lib.wesl": "// code"},
		Dependencies: []Bundle{
			{Name: "base", Edition: "2026_pre", Modules: map[string]string{"./lib.wesl": "// base"}},
		},
	}

	js := b.toJS()
	require.Equal(t, "top", js["name"])
	require.Equal(t, "2026_pre", js["edition"])
	require.Equal(t, b.Modules, js["modules"])

	deps, ok := js["dependencies"].([]any)
	require.True(t, ok)
	require.Len(t, deps, 1)

	dep, ok := deps[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "base", dep["name"])
}

func TestBundle_toJS_NoDependencies(t *testing.T) {
	b := Bundle{Name: "solo", Edition: "2026_pre", Modules: map[string]string{}}
	js := b.toJS()

	_, hasDeps := js["dependencies"]
	require.False(t, hasDeps, "dependencies key must be omitted when there are none")
}

func TestStdout_Discarded(t *testing.T) {
	// When Stdout is unset (the default) console output from the transpiler must
	// not panic and is silently dropped.
	Stdout.Store(nil)

	tr := New()
	_, err := tr.Transpile("main.wesl", Options{
		Files: Files{"main.wesl": "@compute @workgroup_size(1) fn main() {}"},
	})
	require.NoError(t, err)
}

func TestStdout_Captured(t *testing.T) {
	var buf strings.Builder
	var w io.Writer = &buf
	Stdout.Store(&w)
	t.Cleanup(func() { Stdout.Store(nil) })

	tr := New()
	_, err := tr.Transpile("main.wesl", Options{
		Files: Files{"main.wesl": "@compute @workgroup_size(1) fn main() {}"},
	})
	require.NoError(t, err)
	// We don't assert on buf contents (the transpiler may or may not log),
	// only that capturing is wired up without error.
	_ = buf.String()
}
