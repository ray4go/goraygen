package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/bytedance/gg/gslice"
	"golang.org/x/tools/go/packages"
)

// FindStruct finds the struct type in the package that has the specified comment pattern.
func FindStruct(pkg *packages.Package, commentPatten string) *ast.TypeSpec {
	var targetStruct *ast.TypeSpec
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			if targetStruct != nil {
				return false
			}

			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			if genDecl.Doc != nil {
				for _, comment := range genDecl.Doc.List {
					if strings.TrimSpace(comment.Text) == commentPatten {
						for _, spec := range genDecl.Specs {
							if typeSpec, ok := spec.(*ast.TypeSpec); ok {
								if _, ok := typeSpec.Type.(*ast.StructType); ok {
									targetStruct = typeSpec
									return false
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	return targetStruct
}

// Method represents an exported method of a struct.
// for non-variadic method:
// - ($ReceiverType) $Name ($Param[0].Name $Param[0].Type, , $Param[-1].Name $Param[-1].Type) ($Result[0].Type, , $Result[-1].Type)
// for variadic method:
// - ($ReceiverType) $Name ($Param[0].Name $Param[0].Type, , $Param[-1].Name ...$Param[-1].Type) ($Result[0].Type, , $Result[-1].Type)
type Method struct {
	ReceiverType string
	Name         string
	Params       []Param // for variadic, the last param.Type will be the slice element type (i.e. "int" for "...int")
	Results      []Result
	IsVariadic   bool
	Doc          string
}

type Param struct {
	Name string
	Type string // in "$packageName.$typeName" or built-in type like "int", "string" or composite type like "[]int", "map[string]pkg.MyType"
}

type Result struct {
	Type string // format same as Param.Type
}

func (m Method) String() string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		if m.IsVariadic && i == len(m.Params)-1 {
			params[i] = fmt.Sprintf("%s ...%s", p.Name, p.Type)
		} else {
			params[i] = fmt.Sprintf("%s %s", p.Name, p.Type)
		}
	}
	retruns := gslice.Map(m.Results, func(r Result) string {
		return r.Type
	})
	return fmt.Sprintf("%s(%s) (%s)", m.Name, strings.Join(params, ", "), strings.Join(retruns, ", "))
}

// FindMethods finds all exported methods of the given struct name in the package.
func FindMethods(pkg *packages.Package, structName string, importStore *ImportStore) []Method {
	var methods []Method

	// Get the struct type
	obj := pkg.Types.Scope().Lookup(structName)
	if obj == nil {
		return methods
	}

	named, ok := obj.Type().(*types.Named)
	if !ok {
		return methods
	}

	// Iterate through all methods
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if !method.Exported() {
			continue
		}

		sig := method.Type().(*types.Signature)
		m := Method{
			Name: method.Name(),
			Doc:  findFuncDoc(pkg, method.Pos()),
		}
		// fmt.Printf("method: %v Name: %v\n", method, method.Pkg())
		// fmt.Printf("sig.Recv: %v \n", sig.Recv())
		receiverType := sig.Recv().Type()

		// To get just the "*MyTask" part:
		// If it's a pointer, dereference it to get the named type
		receiverTypeStr := ""
		if ptr, ok := receiverType.(*types.Pointer); ok {
			if named, ok := ptr.Elem().(*types.Named); ok {
				receiverTypeStr = fmt.Sprintf("*%s", named.Obj().Name())
			}
		} else if named, ok := receiverType.(*types.Named); ok {
			// If it's not a pointer, it's already the named type
			receiverTypeStr = named.Obj().Name()
		}
		m.ReceiverType = receiverTypeStr

		// Process parameters
		params := sig.Params()
		for j := 0; j < params.Len(); j++ {
			param := params.At(j)

			paramName := param.Name()
			if paramName == "" {
				paramName = fmt.Sprintf("arg%d", j)
			}

			//paramTypeName = types.TypeString(param.Type(), types.RelativeTo(pkg.Types))
			typeName := getTypeName(param.Type(), pkg.Types.Path(), importStore)
			if j == params.Len()-1 && sig.Variadic() {
				// If the last parameter is variadic, remove the [] prefix
				typeName = strings.TrimPrefix(typeName, "[]")
			}
			m.Params = append(m.Params, Param{
				Name: paramName,
				Type: typeName,
			})
		}

		// Check if the last parameter is variadic
		m.IsVariadic = sig.Variadic()

		// Process results
		results := sig.Results()
		for j := 0; j < results.Len(); j++ {
			result := results.At(j)
			m.Results = append(m.Results, Result{
				Type: getTypeName(result.Type(), pkg.Types.Path(), importStore),
			})
		}

		methods = append(methods, m)
	}

	return methods
}

// Convert Go type names to more friendly identifier names
// Examples: []T -> sliceOfT; *T -> pointerOfT; map[K]V -> mapK2V; [n]T -> arrNT; ...
var (
	arrayRegex = regexp.MustCompile(`\[(\d+)\]`)
	mapRegex   = regexp.MustCompile(`map\[([^\]]+)\](.*)`)
	cleanRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func IdentifiableTypeName(typ string) string { // pure helper
	typ = strings.ReplaceAll(typ, "*", "pointerOf")
	typ = strings.ReplaceAll(typ, "[]", "sliceOf")
	typ = arrayRegex.ReplaceAllString(typ, "arr${1}Of")   // [n]T -> arrNT
	typ = mapRegex.ReplaceAllString(typ, "map${1}To${2}") // map[K]V -> mapKToV
	typ = strings.ReplaceAll(typ, "chan<-", "sendChanOf")
	typ = strings.ReplaceAll(typ, "<-chan", "recvChanOf")
	typ = strings.ReplaceAll(typ, "chan ", "chanOf")
	if strings.HasPrefix(typ, "func(") {
		typ = strings.ReplaceAll(typ, "func(", "funcWith")
		typ = strings.ReplaceAll(typ, ")", "")
	}
	typ = strings.ReplaceAll(typ, "interface{}", "any") // interface{} -> any
	typ = strings.ReplaceAll(typ, " ", "_")
	typ = strings.ReplaceAll(typ, ".", "_")
	// only keep alphanumeric + '_' chars
	typ = cleanRegex.ReplaceAllString(typ, "")
	return typ
}

// getTypeName extracts the type name (pkgName.typeName) from a types.Type variable.
// If the type is defined in currentPkgPath, the package name is omitted.
func getTypeName(typ types.Type, currentPkgPath string, importStore *ImportStore) string {
	var typeName string
	// Named type - the only case with an explicit package name.
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj() // Get the *types.TypeName object that defines this named type that defines this named type
		if obj != nil {
			// typeName is the name of this type (e.g., "MyStruct", "Reader")
			typeName = obj.Name()
			// Package() returns the package that defines this type; nil for predeclared types (e.g., int)
			if obj.Pkg() != nil {
				packagePath := obj.Pkg().Path() // Package import path (e.g., "fmt", "main")
				if packagePath != currentPkgPath {
					pkgName := importStore.AddImport(packagePath)
					typeName = pkgName + "." + typeName
				}
			}
		}
		return typeName
	}

	// Other *types.Type variants that don't have package names but do have type names.
	// For these types, packagePath will be an empty string.
	switch t := typ.(type) {
	case *types.Basic:
		// Basic types (int, string, bool, etc.)
		typeName = t.Name()
		if t.Kind() == types.UnsafePointer {
			typeName = importStore.AddImport("unsafe") + "." + typeName
		}
	case *types.Pointer:
		// Pointer types (*int, *MyStruct)
		// Type name is "*" + element type name
		// For more precise representation, can recursively call getPackageAndTypeName(t.Elem())
		elemTypeName := getTypeName(t.Elem(), currentPkgPath, importStore)
		typeName = "*" + elemTypeName
	case *types.Slice:
		// Slice types ([]int, []MyStruct)
		elemTypeName := getTypeName(t.Elem(), currentPkgPath, importStore)
		typeName = "[]" + elemTypeName
	case *types.Array:
		// Array types ([N]int, [N]MyStruct)
		elemTypeName := getTypeName(t.Elem(), currentPkgPath, importStore)
		typeName = fmt.Sprintf("[%d]%s", t.Len(), elemTypeName)
	case *types.Map:
		// Map types (map[string]int)
		keyTypeName := getTypeName(t.Key(), currentPkgPath, importStore)
		elemTypeName := getTypeName(t.Elem(), currentPkgPath, importStore)
		typeName = fmt.Sprintf("map[%s]%s", keyTypeName, elemTypeName)
	case *types.Chan:
		// Channel types (chan int, chan<- bool)
		elemTypeName := getTypeName(t.Elem(), currentPkgPath, importStore)
		dir := ""
		switch t.Dir() {
		case types.SendRecv:
			dir = "chan "
		case types.SendOnly:
			dir = "chan<- "
		case types.RecvOnly:
			dir = "<-chan "
		}
		typeName = dir + elemTypeName
	case *types.Signature:
		// Function or method signature types (func(int) string)
		// This is typically only useful when printing the complete function signature.
		// For package and type names, it doesn't usually have an independent "name".
		// Use t.String() if representation is needed.
		typeName = t.String()
	case *types.Struct:
		// Struct literal types (struct { Field int })
		// Similar to anonymous structs.
		typeName = t.String()
	case *types.Interface:
		// Interface literal types (interface { Method() })
		// Similar to anonymous interfaces.
		typeName = t.String()
	default:
		// For other unknown or uncommon types, use their String() method as the name
		typeName = typ.String()
	}

	return typeName
}

func findFuncDoc(pkg *packages.Package, pos token.Pos) string {
	for _, file := range pkg.Syntax {
		if file.Pos() <= pos && pos < file.End() {
			var doc string
			found := false
			ast.Inspect(file, func(n ast.Node) bool {
				if found {
					return false
				}
				if fd, ok := n.(*ast.FuncDecl); ok {
					if fd.Name.Pos() == pos {
						if fd.Doc != nil {
							var comments []string
							for _, c := range fd.Doc.List {
								comments = append(comments, c.Text)
							}
							doc = strings.Join(comments, "\n")
						}
						found = true
						return false
					}
				}
				return true
			})
			if found {
				return doc
			}
		}
	}
	return ""
}
