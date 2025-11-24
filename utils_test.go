package main

import (
	"fmt"
	"testing"

	"github.com/bytedance/gg/gmap"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

const goModCentent = `
module %s

go 1.24

require (
	"github.com/ray4go/go-ray/ray" v1.0.0
)

`

func makePkgFromSource(t *testing.T, sources map[string]string, pkgPath string) *packages.Package {
	t.Helper()
	pkgDir := t.TempDir()
	goModFile := pkgDir + "/go.mod"
	goModContent := fmt.Sprintf(goModCentent, pkgPath)
	files := gmap.Map(sources, func(filename, content string) (string, []byte) {
		return fmt.Sprintf("%s/%s.go", pkgDir, filename), []byte(content)
	})
	files[goModFile] = []byte(goModContent)

	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
		Dir:     pkgDir,
		Overlay: files,
	}

	pkgs, err := packages.Load(cfg, ".")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		for _, e := range pkg.Errors {
			t.Logf("Package error: %v", e)
		}
	}
	return pkg
}

var getTypeNameTestCases = []struct {
	code           string
	expectTypeName string
}{
	{`
import (
	"bytes"
	"time"
)
var T map[time.Duration][]*bytes.Buffer`, "map[time.Duration][]*bytes.Buffer",
	},

	{`
import (
	"bytes"
	"time"
)

type MyDuration time.Duration
var T map[MyDuration][]*bytes.Buffer`, "map[MyDuration][]*bytes.Buffer",
	},
}

func TestGetTypeName(t *testing.T) {
	assert := require.New(t)

	pkgPath := "example.com/mypkg"
	for _, tc := range getTypeNameTestCases {
		code := "package mypkg\n" + tc.code
		pkg := makePkgFromSource(t, map[string]string{"foo": code}, pkgPath)
		obj := pkg.Types.Scope().Lookup("T")
		assert.NotNil(obj, "type T not found")
		typ := obj.Type()

		importStore := NewImportStore()
		result := getTypeName(typ, pkgPath, importStore)
		assert.Equal(tc.expectTypeName, result, "getTypeName code:\n%s", tc.code)
	}
}

func TestFindMethodsWithDoc(t *testing.T) {
	code := `package mypkg

// raytasks
type MyTasks struct{}

// Foo does something.
// It returns error.
func (t *MyTasks) Foo() error { return nil }

// Bar does something else.
func (t *MyTasks) Bar() {}
`
	pkg := makePkgFromSource(t, map[string]string{"tasks": code}, "example.com/mypkg")
	importStore := NewImportStore()
	methods := FindMethods(pkg, "MyTasks", importStore)

	require.Len(t, methods, 2)

	var foo, bar Method
	for _, m := range methods {
		if m.Name == "Foo" {
			foo = m
		} else if m.Name == "Bar" {
			bar = m
		}
	}

	require.Equal(t, "Foo", foo.Name)
	require.Equal(t, "// Foo does something.\n// It returns error.", foo.Doc)

	require.Equal(t, "Bar", bar.Name)
	require.Equal(t, "// Bar does something else.", bar.Doc)
}
