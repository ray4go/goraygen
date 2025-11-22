package main

import (
	"fmt"
	"testing"

	"github.com/bytedance/gg/gmap"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func makePkgFromSource(t *testing.T, sources map[string]string, pkgPath string) *packages.Package {
	t.Helper()
	pkgDir := t.TempDir()
	goModFile := pkgDir + "/go.mod"
	goModContent := fmt.Sprintf("module %s\n\ngo 1.24\n", pkgPath)
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

	{`
import (
	"github.com/ray4go/go-ray/ray"
)
var T ray.ObjectRef`, "ray.ObjectRef",
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
