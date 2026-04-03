package main

import (
	"fmt"

	"github.com/Dyuzhovsergey/shortener-url/internal/linter/noexit"

	"github.com/gordonklaus/ineffassign/pkg/ineffassign"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	// Анализаторы из golang.org/x/tools/go/analysis/passes
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	hclint "honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func lintToAnalysis(list []*hclint.Analyzer) []*analysis.Analyzer {
	res := make([]*analysis.Analyzer, 0, len(list))
	for _, a := range list {
		if a == nil || a.Analyzer == nil {
			continue
		}
		res = append(res, a.Analyzer)
	}
	return res
}

func mustFindByName(list []*hclint.Analyzer, name string) *analysis.Analyzer {
	for _, a := range list {
		if a != nil && a.Analyzer != nil && a.Analyzer.Name == name {
			return a.Analyzer
		}
	}
	panic(fmt.Sprintf("staticlint: required analyzer %q not found", name))
}

func main() {
	// Набор анализаторов из x/tools
	analyzers := []*analysis.Analyzer{
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	}

	// Все SA-анализаторы staticcheck.
	analyzers = append(analyzers, lintToAnalysis(staticcheck.Analyzers)...)

	// Анализатор из других классов
	// Берём ST1000 — проверка наличия package comment.
	analyzers = append(analyzers, mustFindByName(stylecheck.Analyzers, "ST1000"))

	// Два публичных анализатора на выбор.
	analyzers = append(analyzers, ineffassign.Analyzer)
	analyzers = append(analyzers, bodyclose.Analyzer)

	// noOsExitInMainAnalyzer анализатор.
	analyzers = append(analyzers, noexit.Analyzer)

	multichecker.Main(analyzers...)
}
