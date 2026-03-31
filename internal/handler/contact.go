package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ekm/mailbot/internal/mailer"
	"github.com/ekm/mailbot/internal/store"
	"github.com/ekm/mailbot/internal/submission"
)

const maxBodyBytes = 64 * 1024 // 64 KB

// ContactHandler handles POST /contact requests.
// It is intentionally thin: decode → validate → dispatch side effects → respond.
type ContactHandler struct {
	mailer mailer.Mailer
	store  store.Store
	logger *slog.Logger
}

// New constructs a ContactHandler with the provided dependencies.
func New(m mailer.Mailer, s store.Store, logger *slog.Logger) *ContactHandler {
	return &ContactHandler{mailer: m, store: s, logger: logger}
}

// contactRequest is the expected JSON request body.
type contactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Reason  string `json:"reason"`
}

func (h *ContactHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req contactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request body"))
		return
	}

	sub := submission.New(req.Name, req.Email, req.Phone, req.Subject, req.Body, req.Reason, time.Now().UTC())

	if ve := submission.Validate(sub); ve != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "errors": map[string]string(ve)})
		return
	}

	// Both side effects are always attempted; neither suppresses the other.
	var (
		storeErr error
		mailErr  error
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() { defer wg.Done(); storeErr = h.store.Save(r.Context(), sub) }()
	go func() { defer wg.Done(); mailErr = h.mailer.Send(r.Context(), sub) }()
	wg.Wait()

	if storeErr != nil {
		h.logger.ErrorContext(r.Context(), "store error", "error", storeErr)
	}
	if mailErr != nil {
		h.logger.ErrorContext(r.Context(), "mailer error", "error", mailErr)
	}
	if storeErr != nil || mailErr != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal server error"))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

type responseBody struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func errBody(msg string) responseBody {
	return responseBody{OK: false, Error: msg}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// compile-time interface check
var _ http.Handler = (*ContactHandler)(nil)
