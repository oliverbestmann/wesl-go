package wegl

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed test/shaders
var shaders embed.FS

func TestTranspiler_Transpile(t *testing.T) {
	Stdout = os.Stdout

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
