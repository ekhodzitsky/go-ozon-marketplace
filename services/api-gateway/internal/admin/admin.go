package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/go-chi/chi/v5"
)

// Handler provides admin endpoints for feature flags.
type Handler struct {
	flags *featureflags.FeatureFlags
}

// NewHandler creates a new admin handler.
func NewHandler(flags *featureflags.FeatureFlags) *Handler {
	return &Handler{flags: flags}
}

// ListFlags returns all registered feature flags.
func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.flags.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(flags)
}

// EnableFlag enables a feature flag.
func (h *Handler) EnableFlag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.flags.SetEnabled(r.Context(), name, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "enabled"})
}

// DisableFlag disables a feature flag.
func (h *Handler) DisableFlag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.flags.SetEnabled(r.Context(), name, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "disabled"})
}

// SetPercentage sets a percentage rollout for a feature flag.
func (h *Handler) SetPercentage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	value, err := strconv.Atoi(chi.URLParam(r, "value"))
	if err != nil {
		http.Error(w, "invalid percentage", http.StatusBadRequest)
		return
	}
	if err := h.flags.SetPercentage(r.Context(), name, value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, map[string]interface{}{"status": "percentage_set", "percentage": value})
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
