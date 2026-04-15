package cli

import (
	"io"
	"net/http"
	"strings"
)

// parseServeAddr normalises a --serve value into host:port.
func parseServeAddr(raw string) string {
	if !strings.Contains(raw, ":") {
		return "127.0.0.1:" + raw
	}
	if strings.HasPrefix(raw, ":") {
		return "127.0.0.1" + raw
	}
	return raw
}

// newServeHandler returns an HTTP handler that accepts POST requests and
// delivers the body as a trigger message via WriteTrigger.
func newServeHandler(effectiveID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		if err := WriteTrigger(effectiveID, string(body)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}
