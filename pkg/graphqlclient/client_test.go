package graphqlclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Do_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req request
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, `query { me { id } }`, req.Query)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"me":{"id":"123"}}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	var result struct {
		Me struct {
			ID string `json:"id"`
		} `json:"me"`
	}

	err := client.Do(context.Background(), `query { me { id } }`, &result)
	require.NoError(t, err)
	assert.Equal(t, "123", result.Me.ID)
}

func TestClient_Do_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"},{"message":"bad request"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Do(context.Background(), `query { me { id } }`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
	assert.Contains(t, err.Error(), "bad request")
}

func TestClient_Do_NonOKStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Do(context.Background(), `query { me { id } }`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Do_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Do(context.Background(), `query { me { id } }`, nil)
	require.Error(t, err)
}
