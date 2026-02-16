package handler

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// mountPprof добавляет pprof-ручки по пути /debug/pprof.
func mountPprof(r chi.Router) {
	r.Route("/debug/pprof", func(r chi.Router) {
		r.Get("/", pprof.Index)
		r.Get("/cmdline", pprof.Cmdline)
		r.Get("/profile", pprof.Profile)
		r.Get("/symbol", pprof.Symbol)
		r.Post("/symbol", pprof.Symbol)
		r.Get("/trace", pprof.Trace)

		r.Handle("/allocs", pprof.Handler("allocs"))
		r.Handle("/block", pprof.Handler("block"))
		r.Handle("/goroutine", pprof.Handler("goroutine"))
		r.Handle("/heap", pprof.Handler("heap"))
		r.Handle("/mutex", pprof.Handler("mutex"))
		r.Handle("/threadcreate", pprof.Handler("threadcreate"))

		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		})
	})
}
