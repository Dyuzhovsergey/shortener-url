package middleware

import (
	"net"
	"net/http"
	"strings"
)

// TrustedSubnetMiddleware пропускает запрос только если X-Real-IP
// принадлежит доверенной подсети.
func TrustedSubnetMiddleware(trustedSubnet *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedSubnet == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
			if realIP == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			ip := net.ParseIP(realIP)
			if ip == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			if !trustedSubnet.Contains(ip) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
