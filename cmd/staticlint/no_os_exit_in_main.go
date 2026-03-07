package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// noOsExitInMainAnalyzer проверяет, что:
// - os.Exit(...)
// - panic(...)
// - log.Fatal(...), log.Fatalf(...), log.Fatalln(...)
// не вызываются вне функции main.
var noOsExitInMainAnalyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  "forbids os.Exit, panic and log.Fatal* outside main function",
	Run:  runNoOsExitInMain,
}

// runNoOsExitInMain проверяет, что вне func main() нет прямых вызовов
// os.Exit(...), panic(...), log.Fatal(...), log.Fatalf(...), log.Fatalln(...).
func runNoOsExitInMain(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}

			funcName := fd.Name.Name

			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Внутри main разрешаем такие вызовы.
				if funcName == "main" {
					return true
				}

				switch {
				case isPanicCall(pass, call):
					pass.Reportf(call.Pos(), "panic must not be called outside main")
				case isOSExitCall(pass, call):
					pass.Reportf(call.Pos(), "os.Exit must not be called outside main")
				case isLogFatalCall(pass, call):
					pass.Reportf(call.Pos(), "log.Fatal/Fatalf/Fatalln must not be called outside main")
				}

				return true
			})
		}
	}

	return nil, nil
}

// isPanicCall проверяет вызов встроенной функции panic(...).
func isPanicCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name != "panic" {
		return false
	}

	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}

	builtin, ok := obj.(*types.Builtin)
	if !ok {
		return false
	}

	return builtin.Name() == "panic"
}

// isOSExitCall проверяет вызов os.Exit(...).
func isOSExitCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "Exit" {
		return false
	}

	pkgName, ok := importedPkgName(pass, sel.X)
	if !ok {
		return false
	}

	return pkgName.Imported().Path() == "os"
}

// isLogFatalCall проверяет вызовы log.Fatal(...), log.Fatalf(...), log.Fatalln(...).
func isLogFatalCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	switch sel.Sel.Name {
	case "Fatal", "Fatalf", "Fatalln":
	default:
		return false
	}

	pkgName, ok := importedPkgName(pass, sel.X)
	if !ok {
		return false
	}

	return pkgName.Imported().Path() == "log"
}

// importedPkgName возвращает импортированный пакет, если выражение — это имя импортированного пакета.
func importedPkgName(pass *analysis.Pass, expr ast.Expr) (*types.PkgName, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil, false
	}

	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return nil, false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return nil, false
	}

	return pkgName, true
}
