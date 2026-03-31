package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns a middleware that catches panics, logs them with a full
// stack trace, and responds 500 Internal Server Error.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.ErrorContext(r.Context(), "panic recovered",
						"error", fmt.Sprintf("%v", v),
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"ok":false,"error":"internal server error"}`)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
