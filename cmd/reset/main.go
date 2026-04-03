// cmd/reset/main.go
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const marker = "generate:reset"

func main() {
	// Позволяем явно указать корень (удобно для отладки), но по умолчанию ищем go.mod вверх от cwd.
	rootFlag := flag.String("root", "", "path to module root (directory with go.mod)")
	flag.Parse()

	root, err := resolveModuleRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resetgen: %v\n", err)
		os.Exit(1)
	}

	pkgs, err := scanPackages(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resetgen: scan: %v\n", err)
		os.Exit(1)
	}

	// Генерируем reset.gen.go для пакетов, где есть структуры с маркером.
	var generated int
	for _, pkg := range pkgs {
		if len(pkg.Structs) == 0 {
			continue
		}
		if err := generateForPackage(pkg); err != nil {
			fmt.Fprintf(os.Stderr, "resetgen: generate %s: %v\n", pkg.Dir, err)
			os.Exit(1)
		}
		generated++
	}

	fmt.Printf("resetgen: done, packages updated: %d\n", generated)
}

type PackageInfo struct {
	Dir     string
	Name    string
	Structs []StructInfo
}

type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

type FieldInfo struct {
	Names []string // может быть несколько имён у одного типа: a, b int
	Type  ast.Expr
}

// resolveModuleRoot ищет папку с go.mod.
// Если -root задан, проверяем что там есть go.mod.
func resolveModuleRoot(rootFlag string) (string, error) {
	if rootFlag != "" {
		p := filepath.Clean(rootFlag)
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err != nil {
			return "", fmt.Errorf("go.mod not found in -root=%s", p)
		}
		return p, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("cannot find go.mod вверх от текущей директории")
		}
		dir = parent
	}
}

// scanPackages обходит директории, собирает пакеты и структуры с // generate:reset.
func scanPackages(root string) ([]PackageInfo, error) {
	// Мапа: dir -> PackageInfo (на случай повторов).
	pkgMap := make(map[string]*PackageInfo)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		// Пропуски “мусорных” директорий.
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return fs.SkipDir
		}
		if rel == "vendor" || strings.HasPrefix(rel, "vendor/") {
			return fs.SkipDir
		}
		if rel == "cmd/reset" || strings.HasPrefix(rel, "cmd/reset/") {
			// не пытаемся генерировать внутри генератора
			return fs.SkipDir
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			// скрытые каталоги
			return fs.SkipDir
		}

		hasGoFiles, err := dirHasGoFiles(path)
		if err != nil {
			return err
		}
		if !hasGoFiles {
			return nil
		}

		pkg, err := parsePackage(path)
		if err != nil {
			return err
		}
		if pkg != nil {
			pkgMap[path] = pkg
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var res []PackageInfo
	for _, p := range pkgMap {
		res = append(res, *p)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Dir < res[j].Dir })
	return res, nil
}

func dirHasGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go") &&
			name != "reset.gen.go" {
			return true, nil
		}
	}
	return false, nil
}

// parsePackage парсит пакет в директории и возвращает PackageInfo,
// если в пакете есть хотя бы одна структура с // generate:reset.
func parsePackage(dir string) (*PackageInfo, error) {
	// packages.Load корректно учитывает build tags и собирает только те файлы,
	// которые реально входят в пакет при текущей конфигурации сборки.
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedSyntax |
			packages.NeedCompiledGoFiles,
		Tests: false, // тестовые пакеты нам не нужны
	}

	loaded, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return nil, nil
	}

	// Если есть ошибки загрузки пакета — вернём их одной ошибкой.
	// Это удобнее, чем молча генерировать неполный файл.
	var loadErrs []string
	for _, p := range loaded {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	}
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("packages.Load errors:\n%s", strings.Join(loadErrs, "\n"))
	}

	// Обычно в директории один пакет (loaded[0]).
	// Берём первый “нормальный” пакет.
	pkg := loaded[0]
	pkgName := pkg.Name
	if pkgName == "" {
		return nil, nil
	}

	var structs []StructInfo

	// pkg.Syntax соответствует pkg.CompiledGoFiles по индексам.
	for i, f := range pkg.Syntax {
		if i < len(pkg.CompiledGoFiles) {
			// Игнорируем reset.gen.go (чтобы не парсить уже сгенерённое).
			if filepath.Base(pkg.CompiledGoFiles[i]) == "reset.gen.go" {
				continue
			}
			// На всякий случай игнорируем тестовые файлы (обычно их и так не будет).
			if strings.HasSuffix(pkg.CompiledGoFiles[i], "_test.go") {
				continue
			}
		}

		// Ищем TypeSpec со StructType и проверяем комментарий.
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}

			declHasMarker := hasMarkerInCommentGroup(gd.Doc)

			for _, sp := range gd.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}

				specHasMarker := declHasMarker || hasMarkerInCommentGroup(ts.Doc)
				if !specHasMarker {
					continue
				}

				si := StructInfo{Name: ts.Name.Name}
				si.Fields = extractFields(st)
				structs = append(structs, si)
			}
		}
	}

	if len(structs) == 0 {
		return nil, nil
	}

	sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })

	return &PackageInfo{
		Dir:     dir,
		Name:    pkgName,
		Structs: structs,
	}, nil
}

func hasMarkerInCommentGroup(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		// c.Text включает // или /* */
		txt := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		txt = strings.TrimSpace(txt)
		if txt == marker {
			return true
		}
	}
	return false
}

func extractFields(st *ast.StructType) []FieldInfo {
	if st.Fields == nil || len(st.Fields.List) == 0 {
		return nil
	}

	var fields []FieldInfo
	for _, f := range st.Fields.List {
		// Пропускаем embedded-поля (без имени), чтобы не ломать генерацию.
		if len(f.Names) == 0 {
			continue
		}

		var names []string
		for _, n := range f.Names {
			names = append(names, n.Name)
		}

		fields = append(fields, FieldInfo{
			Names: names,
			Type:  f.Type,
		})
	}
	return fields
}

// generateForPackage создаёт/перезаписывает reset.gen.go в директории пакета.
func generateForPackage(pkg PackageInfo) error {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by reset generator; DO NOT EDIT.\n")
	buf.WriteString("// This file was generated by cmd/reset.\n\n")
	buf.WriteString("package " + pkg.Name + "\n\n")

	for _, st := range pkg.Structs {
		writeResetMethod(&buf, st, pkg.Name)
		buf.WriteString("\n")
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		// Если форматирование упало — полезно увидеть сырой код.
		return fmt.Errorf("format.Source: %w\n--- raw ---\n%s", err, buf.String())
	}

	outPath := filepath.Join(pkg.Dir, "reset.gen.go")
	return os.WriteFile(outPath, src, 0o644)
}

func writeResetMethod(buf *bytes.Buffer, st StructInfo, _ string) {
	recv := receiverName(st.Name)

	fmt.Fprintf(buf, "func (%s *%s) Reset() {\n", recv, st.Name)
	fmt.Fprintf(buf, "\tif %s == nil {\n\t\treturn\n\t}\n\n", recv)

	for _, f := range st.Fields {
		for _, name := range f.Names {
			writeResetForField(buf, recv, name, f.Type)
		}
	}

	buf.WriteString("}\n")
}

func receiverName(typeName string) string {
	// Простейший вариант: первая буква в нижний регистр.
	// Чтобы не конфликтовать с ключевыми словами, добавим "r" если пусто.
	if typeName == "" {
		return "r"
	}
	r := strings.ToLower(typeName[:1])
	if r == "_" {
		return "r"
	}
	return r
}

func writeResetForField(buf *bytes.Buffer, recv, fieldName string, fieldType ast.Expr) {
	fieldSel := fmt.Sprintf("%s.%s", recv, fieldName)

	switch t := fieldType.(type) {
	case *ast.Ident:
		// примитивы и “простые” имена
		if zero, ok := zeroLiteralForBuiltin(t.Name); ok {
			fmt.Fprintf(buf, "\t%s = %s\n", fieldSel, zero)
			return
		}
		// Для прочих именованных типов — присваиваем нулевое значение через var.
		writeZeroAssignViaVar(buf, fieldSel, fieldType)
		return

	case *ast.ArrayType:
		if t.Len == nil {
			// slice
			// nil-слайс нельзя слайсить, поэтому проверяем на nil
			fmt.Fprintf(buf, "\tif %s != nil {\n", fieldSel)
			fmt.Fprintf(buf, "\t\t%s = %s[:0]\n", fieldSel, fieldSel)
			fmt.Fprintf(buf, "\t}\n")
			return
		}
		// array — нулевое значение
		writeZeroAssignViaVar(buf, fieldSel, fieldType)
		return

	case *ast.MapType:
		// clear работает и на nil map
		fmt.Fprintf(buf, "\tclear(%s)\n", fieldSel)
		return

	case *ast.StarExpr:
		// pointer
		writeResetForPointer(buf, fieldSel, t.X)
		return

	case *ast.StructType:
		// анонимная struct — попробуем вызвать Reset, иначе занулить
		writeResetForStructValue(buf, fieldSel, fieldType)
		return

	case *ast.SelectorExpr:
		// pkg.Type — не знаем, примитив или нет; работаем как с “прочими”
		// Но если это slice/map/ptr — сюда не попадёт, они обрабатываются выше.
		writeZeroAssignViaVar(buf, fieldSel, fieldType)
		return

	case *ast.InterfaceType, *ast.FuncType, *ast.ChanType:
		// нулевое для ссылочных/интерфейсных — nil
		fmt.Fprintf(buf, "\t%s = nil\n", fieldSel)
		return

	default:
		// на всякий случай — нулим через var
		writeZeroAssignViaVar(buf, fieldSel, fieldType)
		return
	}
}

func writeResetForPointer(buf *bytes.Buffer, ptrExpr string, elemType ast.Expr) {
	// ptrExpr — это выражение указателя (например: r.child)
	// Сбрасываем только если он не nil.
	fmt.Fprintf(buf, "\tif %s != nil {\n", ptrExpr)

	// Если элемент — struct или именованный тип, попробуем вызвать Reset()
	// через runtime type assertion (без анализа методов по всему проекту).
	// Если Reset нет — применяем “сброс по правилам” к *ptrExpr.
	fmt.Fprintf(buf, "\t\tif resetter, ok := any(%s).(interface{ Reset() }); ok {\n", ptrExpr)
	fmt.Fprintf(buf, "\t\t\tresetter.Reset()\n")
	fmt.Fprintf(buf, "\t\t} else {\n")
	writeResetForDerefValue(buf, "\t\t\t", "*"+ptrExpr, elemType)
	fmt.Fprintf(buf, "\t\t}\n")

	fmt.Fprintf(buf, "\t}\n")
}

func writeResetForDerefValue(buf *bytes.Buffer, indent, derefExpr string, elemType ast.Expr) {
	// derefExpr — это уже разыменованное значение, например "*r.strP"
	// elemType — тип после *
	switch t := elemType.(type) {
	case *ast.Ident:
		if zero, ok := zeroLiteralForBuiltin(t.Name); ok {
			fmt.Fprintf(buf, "%s%s = %s\n", indent, derefExpr, zero)
			return
		}
		writeZeroAssignViaVarWithIndent(buf, indent, derefExpr, elemType)
		return

	case *ast.ArrayType:
		if t.Len == nil {
			// *ptr — slice
			fmt.Fprintf(buf, "%sif %s != nil {\n", indent, derefExpr)
			fmt.Fprintf(buf, "%s\t%s = %s[:0]\n", indent, derefExpr, derefExpr)
			fmt.Fprintf(buf, "%s}\n", indent)
			return
		}
		writeZeroAssignViaVarWithIndent(buf, indent, derefExpr, elemType)
		return

	case *ast.MapType:
		fmt.Fprintf(buf, "%sclear(%s)\n", indent, derefExpr)
		return

	case *ast.StarExpr:
		// pointer to pointer
		fmt.Fprintf(buf, "%sif %s != nil {\n", indent, derefExpr)
		fmt.Fprintf(buf, "%s\tif resetter, ok := any(%s).(interface{ Reset() }); ok {\n", indent, derefExpr)
		fmt.Fprintf(buf, "%s\t\tresetter.Reset()\n", indent)
		fmt.Fprintf(buf, "%s\t} else {\n", indent)
		writeResetForDerefValue(buf, indent+"\t\t", "*"+derefExpr, t.X)
		fmt.Fprintf(buf, "%s\t}\n", indent)
		fmt.Fprintf(buf, "%s}\n", indent)
		return

	case *ast.StructType:
		// анонимная struct
		writeResetForStructValueIndented(buf, indent, derefExpr, elemType)
		return

	default:
		writeZeroAssignViaVarWithIndent(buf, indent, derefExpr, elemType)
		return
	}
}

func writeResetForStructValue(buf *bytes.Buffer, valueExpr string, typeExpr ast.Expr) {
	writeResetForStructValueIndented(buf, "\t", valueExpr, typeExpr)
}

func writeResetForStructValueIndented(buf *bytes.Buffer, indent, valueExpr string, typeExpr ast.Expr) {
	// Для valueExpr (структурного значения) делаем:
	// if resetter, ok := any(&valueExpr).(interface{ Reset() }); ok { resetter.Reset() } else { valueExpr = zero }
	fmt.Fprintf(buf, "%sif resetter, ok := any(&%s).(interface{ Reset() }); ok {\n", indent, valueExpr)
	fmt.Fprintf(buf, "%s\tresetter.Reset()\n", indent)
	fmt.Fprintf(buf, "%s} else {\n", indent)
	writeZeroAssignViaVarWithIndent(buf, indent+"\t", valueExpr, typeExpr)
	fmt.Fprintf(buf, "%s}\n", indent)
}

func zeroLiteralForBuiltin(name string) (string, bool) {
	switch name {
	case "string":
		return `""`, true
	case "bool":
		return "false", true
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"byte", "rune",
		"float32", "float64",
		"complex64", "complex128":
		return "0", true
	default:
		return "", false
	}
}

func writeZeroAssignViaVar(buf *bytes.Buffer, lhs string, t ast.Expr) {
	writeZeroAssignViaVarWithIndent(buf, "\t", lhs, t)
}

func writeZeroAssignViaVarWithIndent(buf *bytes.Buffer, indent, lhs string, t ast.Expr) {
	// var zero <T>; lhs = zero
	typeStr := exprToString(t)
	zeroName := "zero"

	// чтобы избежать конфликтов, делаем имя переменной зависимым от lhs
	// (быстро и просто)
	suffix := sanitizeIdent(lhs)
	if suffix != "" {
		zeroName = "zero_" + suffix
	}

	fmt.Fprintf(buf, "%svar %s %s\n", indent, zeroName, typeStr)
	fmt.Fprintf(buf, "%s%s = %s\n", indent, lhs, zeroName)
}

func exprToString(e ast.Expr) string {
	var b bytes.Buffer
	_ = format.Node(&b, token.NewFileSet(), e)
	return b.String()
}

func sanitizeIdent(s string) string {
	// превращаем "r.Field" в "r_Field"
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "[", "_")
	s = strings.ReplaceAll(s, "]", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "-", "_")

	// оставляем только буквы/цифры/_
	var out strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
