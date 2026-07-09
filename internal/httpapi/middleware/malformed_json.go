package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// MalformedJSON rejects application/json bodies that are not valid JSON with a
// Rails-compatible JSend fail response.
func MalformedJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !expectsJSONBody(r) {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			writeJSendFail(w, http.StatusBadRequest, "Malformed request body")
			return
		}

		if len(bytes.TrimSpace(body)) > 0 && !json.Valid(body) {
			writeJSendFail(w, http.StatusBadRequest, "Malformed request body")
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func expectsJSONBody(r *http.Request) bool {
	if r.Body == nil || r.Body == http.NoBody {
		return false
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return false
	}
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func writeJSendFail(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "fail",
		"message": message,
	})
}
