package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadServerCredentials_MissingFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		certFile string
		keyFile  string
	}{
		{
			name:     "missing_cert",
			certFile: "/nonexistent/cert.pem",
			keyFile:  "/nonexistent/key.pem",
		},
		{
			name:     "missing_key",
			certFile: createTempFile(t, "cert.pem", "cert"),
			keyFile:  "/nonexistent/key.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt, err := LoadServerCredentials(tt.certFile, tt.keyFile)
			require.Error(t, err)
			assert.Nil(t, opt)
		})
	}
}

func TestLoadClientCredentials_MissingFile(t *testing.T) {
	t.Parallel()

	creds, err := LoadClientCredentials("/nonexistent/ca.pem", "localhost")
	require.Error(t, err)
	assert.Nil(t, creds)
}

func createTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
