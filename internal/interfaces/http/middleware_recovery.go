package http

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recovery wraps an http.Handler with panic recovery. If a handler
// panics, the middleware logs the stack trace and returns 500 instead
// of crashing the API process. This is the outermost handler in the
// chain — see router.go.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
