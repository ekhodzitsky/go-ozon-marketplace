package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter(t *testing.T) http.Handler {
	flags, err := featureflags.New(nil)
	require.NoError(t, err)
	require.NoError(t, flags.Register(context.Background(), &featureflags.Flag{Name: "new-checkout-flow", Enabled: false, Strategy: "default"}))
	require.NoError(t, flags.Register(context.Background(), &featureflags.Flag{Name: "fast-search", Enabled: false, Strategy: "default"}))
	return admin.NewRouter(admin.NewHandler(flags), "")
}

func TestListFlags(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/flags", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var flags []featureflags.Flag
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &flags))
	assert.Len(t, flags, 2)
}

func TestEnableFlag(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/flags/fast-search/enable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "enabled")
}

func TestDisableFlag(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/flags/fast-search/disable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "disabled")
}

func TestSetPercentage(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/flags/fast-search/percentage/42", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "percentage_set")
	assert.Contains(t, rec.Body.String(), "42")
}

func TestSetPercentage_InvalidValue(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/flags/fast-search/percentage/abc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid percentage")
}

func TestUnknownRoute(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/flags/unknown/route", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInvalidMethod(t *testing.T) {
	router := setupRouter(t)
	req := httptest.NewRequest(http.MethodDelete, "/flags", bytes.NewReader(nil))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
