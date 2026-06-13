package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCreds_WithCertPath(t *testing.T) {
	tmp := t.TempDir()
	generateTestCerts(t, tmp)

	cfg := &config.Config{CertPath: tmp}
	creds, err := clientCreds(cfg, "localhost:50051")
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func generateTestCerts(t *testing.T, dir string) {
	t.Helper()

	caKey := filepath.Join(dir, "ca-key.pem")
	caCert := filepath.Join(dir, "ca-cert.pem")
	serverKey := filepath.Join(dir, "server-key.pem")
	serverCert := filepath.Join(dir, "server-cert.pem")
	serverCSR := filepath.Join(dir, "server.csr")

	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048",
		"-keyout", caKey, "-out", caCert,
		"-days", "1", "-nodes",
		"-subj", "/CN=Test CA", "-batch")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("openssl", "req", "-newkey", "rsa:2048",
		"-keyout", serverKey, "-out", serverCSR,
		"-nodes", "-subj", "/CN=localhost", "-batch")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("openssl", "x509", "-req", "-in", serverCSR,
		"-CA", caCert, "-CAkey", caKey,
		"-CAcreateserial", "-out", serverCert, "-days", "1")
	require.NoError(t, cmd.Run())

	require.NoError(t, os.Remove(serverCSR))
}
