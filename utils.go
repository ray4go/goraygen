package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

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
}

type Param struct {
	Name string
	Type string // in "$packageName.$typeName" or built-in type like "int", "string" or composite type like "[]int", "map[string]pkg.MyType"
}

type Result struct {
	Type string // format same as Param.Type
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
			typeName := getTypeName(param.Type(), importStore)
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
				Type: getTypeName(result.Type(), importStore),
			})
		}

		methods = append(methods, m)
	}

	return methods
}

// 将 Go 类型名转换为更友好的标识符名称
// 例如：[]T -> sliceOfT; *T -> pointerOfT; map[K]V -> mapK2V; [n]T -> arrNT; ...
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

// getTypeName 从 types.Type 变量中提取类型名 (pkgName.typeName)。
func getTypeName(typ types.Type, importStore *ImportStore) string {
	var typeName string
	// 具名类型 (Named type)，唯一有显式包名的情况。
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj() // 获取定义这个具名类型的 *types.TypeName 对象
		if obj != nil {
			// typeName 是这个类型的名称 (例如 "MyStruct", "Reader")
			typeName = obj.Name()
			// Package() 返回定义这个类型的包，如果类型是预声明的（如 int），则为 nil
			if obj.Pkg() != nil {
				packagePath := obj.Pkg().Path() // 包的导入路径 (例如 "fmt", "main")
				pkgName := importStore.AddImport(packagePath)
				typeName = pkgName + "." + typeName
			}
		}
		return typeName
	}

	// 其他类型的 *types.Type，它们本身没有包名，但有类型名。
	// 对于这些类型，packagePath 将为空字符串。
	switch t := typ.(type) {
	case *types.Basic:
		// 基本类型 (int, string, bool 等)
		typeName = t.Name()
		if t.Kind() == types.UnsafePointer {
			typeName = importStore.AddImport("unsafe") + "." + typeName
		}
	case *types.Pointer:
		// 指针类型 (*int, *MyStruct)
		// 类型名是 "ptrTo" + 元素类型名
		// 如果需要更精确的表示，可以递归调用 getPackageAndTypeName(t.Elem())
		elemTypeName := getTypeName(t.Elem(), importStore)
		typeName = "*" + elemTypeName
	case *types.Slice:
		// 切片类型 ([]int, []MyStruct)
		elemTypeName := getTypeName(t.Elem(), importStore)
		typeName = "[]" + elemTypeName
	case *types.Array:
		// 数组类型 ([N]int, [N]MyStruct)
		elemTypeName := getTypeName(t.Elem(), importStore)
		typeName = fmt.Sprintf("[%d]%s", t.Len(), elemTypeName)
	case *types.Map:
		// 映射类型 (map[string]int)
		keyTypeName := getTypeName(t.Key(), importStore)
		elemTypeName := getTypeName(t.Elem(), importStore)
		typeName = fmt.Sprintf("map[%s]%s", keyTypeName, elemTypeName)
	case *types.Chan:
		// 通道类型 (chan int, chan<- bool)
		elemTypeName := getTypeName(t.Elem(), importStore)
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
		// 函数或方法签名类型 (func(int) string)
		// 这通常只在需要打印完整的函数签名时有用。
		// 对于包名和类型名，它本身通常不具有一个独立的“名称”。
		// 如果需要表示，可以使用 t.String()。
		typeName = t.String()
	case *types.Struct:
		// 结构体字面量类型 (struct { Field int })
		// 类似于匿名结构体。
		typeName = t.String()
	case *types.Interface:
		// 接口字面量类型 (interface { Method() })
		// 类似于匿名接口。
		typeName = t.String()
	default:
		// 对于其他未知或不常见的类型，使用其 String() 方法作为名称
		typeName = typ.String()
	}

	return typeName
}
