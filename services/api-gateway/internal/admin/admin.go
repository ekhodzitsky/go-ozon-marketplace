package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
)

// Handler provides admin endpoints for feature flags.
type Handler struct {
	engine *featureflags.Engine
}

// NewHandler creates a new admin handler.
func NewHandler(engine *featureflags.Engine) *Handler {
	return &Handler{engine: engine}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/flags")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		h.ListFlags(w, r)
	case strings.HasSuffix(path, "/enable") && r.Method == http.MethodPost:
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			h.enableFlag(w, r, parts[0])
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	case strings.HasSuffix(path, "/disable") && r.Method == http.MethodPost:
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			h.disableFlag(w, r, parts[0])
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	case strings.Contains(path, "/percentage/") && r.Method == http.MethodPost:
		parts := strings.Split(path, "/")
		if len(parts) == 3 && parts[1] == "percentage" {
			value, err := strconv.Atoi(parts[2])
			if err != nil {
				http.Error(w, "invalid percentage", http.StatusBadRequest)
				return
			}
			h.setPercentage(w, r, parts[0], value)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// ListFlags returns all registered feature flags.
func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags := h.engine.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(flags)
}

func (h *Handler) enableFlag(w http.ResponseWriter, r *http.Request, name string) {
	if err := h.engine.SetEnabled(name, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

func (h *Handler) disableFlag(w http.ResponseWriter, r *http.Request, name string) {
	if err := h.engine.SetEnabled(name, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

func (h *Handler) setPercentage(w http.ResponseWriter, r *http.Request, name string, value int) {
	if err := h.engine.SetPercentage(name, value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "percentage_set", "percentage": value})
}
