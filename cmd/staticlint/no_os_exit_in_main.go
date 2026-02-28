package main

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// noOsExitInMainAnalyzer — анализатор проекта, запрещающий прямой вызов os.Exit
// внутри func main() пакета main.
//
// Назначение: принудить выносить логику запуска в отдельную функцию (например, run() error),
// чтобы main содержал только обработку ошибки и инициализацию.
var noOsExitInMainAnalyzer = &analysis.Analyzer{
	Name: "noosexitmain",
	Doc:  "запрещает прямой вызов os.Exit в функции main пакета main (только в модуле проекта)",
	Run:  runNoOsExitInMain,
}

// runNoOsExitInMain проверяет, что в func main() пакета main нет прямого вызова os.Exit(...).
func runNoOsExitInMain(pass *analysis.Pass) (any, error) {
	// Интересует только пакет main.
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	// Ограничиваем анализ только пакетами твоего модуля, чтобы не ловить чужие main.
	const modulePath = "github.com/Dyuzhovsergey/shortener-url"
	if !strings.HasPrefix(pass.Pkg.Path(), modulePath) {
		return nil, nil
	}

	if pass.TypesInfo == nil {
		return nil, nil
	}

	for _, f := range pass.Files {
		// Иногда при анализе тестов/go/packages могут попадаться сгенерированные файлы из go-build cache.
		// Там реально есть os.Exit в test main — нам это не интересно.
		if pass.Fset != nil {
			tf := pass.Fset.File(f.Pos())
			if tf != nil {
				name := tf.Name()
				if strings.Contains(name, "go-build") || strings.Contains(name, ".cache/go-build") {
					continue
				}
			}
		}

		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Нужна именно функция main() без ресивера.
			if fd.Recv != nil || fd.Name == nil || fd.Name.Name != "main" || fd.Body == nil {
				continue
			}

			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isOsExitCall(pass, call) {
					pass.Reportf(call.Pos(), "запрещён прямой вызов os.Exit в func main; вынесите логику в отдельную функцию и возвращайте ошибку")
				}
				return true
			})
		}
	}

	return nil, nil
}

// isOsExitCall определяет, является ли вызов вызовом os.Exit(...) (включая вариант с dot-import).
func isOsExitCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// os.Exit(...)
		obj := pass.TypesInfo.Uses[fun.Sel]
		return isOsExitObject(obj)

	case *ast.Ident:
		// Exit(...) при dot-import os (редко, но обработаем)
		obj := pass.TypesInfo.Uses[fun]
		return isOsExitObject(obj)
	}

	return false
}

func isOsExitObject(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	pkg := fn.Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == "os" && fn.Name() == "Exit"
}
