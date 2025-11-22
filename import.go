package main

import (
	"fmt"
	"strings"

	"github.com/bytedance/gg/gmap"
)

type ImportStore struct {
	importPath2pkgName map[string]string // the pkgName may be renamed (import alias)
	pkgName2importExpr map[string]string
}

func NewImportStore() *ImportStore {
	return &ImportStore{
		importPath2pkgName: make(map[string]string),
		pkgName2importExpr: make(map[string]string),
	}
}

// AddImport adds an import path to the store and returns the package name to be used in code.
func (store *ImportStore) AddImport(importPath string) string {
	if pkgName, ok := store.importPath2pkgName[importPath]; ok {
		return pkgName
	}
	pkgName := getPackageName(importPath)
	if _, ok := store.pkgName2importExpr[pkgName]; !ok {
		store.pkgName2importExpr[pkgName] = fmt.Sprintf(`"%s"`, importPath)
		store.importPath2pkgName[importPath] = pkgName
	} else { // name conflict
		i := 2
		newPkgName := fmt.Sprintf("%s%d", pkgName, i)
		for {
			if _, ok := store.pkgName2importExpr[newPkgName]; !ok {
				break
			}
			i++
			newPkgName = fmt.Sprintf("%s%d", pkgName, i)
		}
		store.pkgName2importExpr[newPkgName] = fmt.Sprintf(`"%s" %s`, importPath, newPkgName)
		store.importPath2pkgName[importPath] = newPkgName
	}
	return store.importPath2pkgName[importPath]
}

func (store *ImportStore) DumpImportExprs() []string {
	return gmap.Values(store.pkgName2importExpr)
}

func getPackageName(importPath string) string {
	sep := "/"
	lastIndex := strings.LastIndex(importPath, sep)
	if lastIndex == -1 {
		// If separator not found, return the original string as the only element
		return importPath
	}
	return importPath[lastIndex+len(sep):]
}
