package tests

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPostgres(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("marketplace"),
		postgres.WithUsername("ozon"),
		postgres.WithPassword("ozonpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	cleanup := func() {
		_ = container.Terminate(ctx)
	}

	return connStr, cleanup
}

func RunMigrations(ctx context.Context, t *testing.T, dsn string, migrationDirs ...string) {
	t.Helper()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	for _, dir := range migrationDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read migration dir %s: %v", dir, err)
		}

		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
				files = append(files, entry.Name())
			}
		}
		sort.Strings(files)

		for _, file := range files {
			content, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				t.Fatalf("failed to read migration %s: %v", file, err)
			}

			_, err = pool.Exec(ctx, string(content))
			if err != nil {
				t.Fatalf("failed to execute migration %s: %v", file, err)
			}
		}
	}
}

func GetFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func StartService(t *testing.T, serviceDir string, env []string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/main.go")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start service %s: %v", serviceDir, err)
	}
	return cmd
}

func WaitForGRPC(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("gRPC server did not start at %s", addr)
}

func WaitForHTTP(t *testing.T, url string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("HTTP server did not start at %s", url)
}

func StartElasticsearch(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.11.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
		},
		WaitingFor: wait.ForHTTP("/_cluster/health").WithPort("9200/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start elasticsearch: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to get es host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9200")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to get es port: %v", err)
	}

	url := fmt.Sprintf("http://%s:%s", host, port.Port())
	cleanup := func() {
		_ = container.Terminate(ctx)
	}

	return url, cleanup
}
