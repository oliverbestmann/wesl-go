//go:build !js

package wesl

import (
	"io/fs"
	"maps"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// bykeShaders loads the byke mesh shader with its library from the sibling
// byke checkout, as a realistic workload.
func bykeShaders(b *testing.B) Files {
	root := os.DirFS("../byke/byke2d")

	lib, err := fs.Sub(root, "shaders-lib")
	if err != nil {
		b.Skip("byke checkout not found")
	}

	files, err := FilesOf(lib)
	if err != nil || len(files) == 0 {
		b.Skip("byke checkout not found")
	}

	main, err := fs.ReadFile(root, "mesh-standard-material.wgsl")
	require.NoError(b, err)

	files = maps.Clone(files)
	files["main.wesl"] = string(main)

	return files
}

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		New()
	}
}

func BenchmarkTranspile(b *testing.B) {
	testShaders, _ := fs.Sub(shaders, "test/shaders")
	testFiles, err := FilesOf(testShaders)
	require.NoError(b, err)

	workloads := map[string]Options{
		"test": {
			Files:      testFiles,
			Constants:  map[string]any{"Foobar": 123},
			Conditions: map[string]bool{"MOBILE": true},
		},
		"byke-mesh": {
			Files:       bykeShaders(b),
			PackageName: "byke",
		},
	}

	for name, opts := range workloads {
		b.Run(name, func(b *testing.B) {
			tr := New()

			for b.Loop() {
				_, err := tr.Transpile("main.wesl", opts)
				require.NoError(b, err)
			}
		})
	}
}
