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
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var portMu sync.Mutex

func StartPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
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
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}
	return connStr
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

// GetFreePort returns a free TCP port.
// The mutex protects against races between parallel tests in the same process.
// Note: there is still a narrow TOCTOU window between closing the probe
// listener and binding the port in the spawned service. For a robust fix
// services should support binding to ":0" and reporting the actual port.
func GetFreePort(t *testing.T) int {
	t.Helper()
	portMu.Lock()
	defer portMu.Unlock()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func StartService(t *testing.T, serviceDir string, env []string) *exec.Cmd {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "service")
	build := exec.Command("go", "build", "-o", bin, "./cmd/main.go")
	build.Dir = serviceDir
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", serviceDir, err, out)
	}

	cmd := exec.Command(bin)
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start service %s: %v", serviceDir, err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd
}

// WaitForGRPC waits for a raw TCP connection with exponential backoff.
// Prefer WaitForGRPCHealth for real readiness checks.
func WaitForGRPC(t *testing.T, addr string) {
	t.Helper()
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 500 * time.Millisecond
	b.MaxElapsedTime = 10 * time.Second

	err := backoff.Retry(func() error {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}, b)
	if err != nil {
		t.Fatalf("gRPC server did not start at %s", addr)
	}
}

// WaitForGRPCHealth performs a gRPC health check with exponential backoff.
func WaitForGRPCHealth(t *testing.T, addr string) {
	t.Helper()
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 500 * time.Millisecond
	b.MaxElapsedTime = 10 * time.Second

	err := backoff.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer conn.Close()
		client := healthpb.NewHealthClient(conn)
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			return err
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("health status: %v", resp.Status)
		}
		return nil
	}, b)
	if err != nil {
		t.Fatalf("gRPC health check failed at %s: %v", addr, err)
	}
}

func WaitForHTTP(t *testing.T, url string) {
	t.Helper()
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 500 * time.Millisecond
	b.MaxElapsedTime = 10 * time.Second

	err := backoff.Retry(func() error {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		return nil
	}, b)
	if err != nil {
		t.Fatalf("HTTP server did not start at %s", url)
	}
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

// StartRedis starts a Redis container and returns its address (host:port).
func StartRedis(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start redis: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	return fmt.Sprintf("%s:%s", host, port.Port())
}

// StartKafka starts a Redpanda container and returns its external Kafka bootstrap address.
func StartKafka(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "redpandadata/redpanda:v24.1.1",
		ExposedPorts: []string{"9092/tcp"},
		Cmd: []string{
			"redpanda", "start",
			"--smp", "1",
			"--memory", "1G",
			"--overprovisioned",
			"--kafka-addr", "PLAINTEXT://0.0.0.0:9092",
			"--advertise-kafka-addr", "PLAINTEXT://127.0.0.1:9092",
		},
		WaitingFor: wait.ForListeningPort("9092/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start kafka: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get kafka host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9092")
	if err != nil {
		t.Fatalf("failed to get kafka port: %v", err)
	}

	return fmt.Sprintf("%s:%s", host, port.Port())
}

// StartClickHouse starts a ClickHouse container and returns its DSN (tcp://host:port?database=default).
func StartClickHouse(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:24.3",
		ExposedPorts: []string{"8123/tcp", "9000/tcp"},
		Env: map[string]string{
			"CLICKHOUSE_DB":       "marketplace",
			"CLICKHOUSE_USER":     "ozon",
			"CLICKHOUSE_PASSWORD": "ozonpass",
		},
		WaitingFor: wait.ForHTTP("/").WithPort("8123/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start clickhouse: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get clickhouse host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("failed to get clickhouse port: %v", err)
	}

	return fmt.Sprintf("tcp://%s:%s?database=marketplace&username=ozon&password=ozonpass", host, port.Port())
}

// Cluster holds shared test infrastructure for integration/e2e tests.
type Cluster struct {
	T           *testing.T
	PostgresDSN string
	RedisAddr   string
	ESURL       string
	CHDSN       string
	KafkaAddr   string
}

// NewCluster starts Postgres and optional containers based on the test needs.
// It does NOT start application services; use StartService for that.
func NewCluster(t *testing.T) *Cluster {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	dsn := StartPostgres(ctx, t)
	return &Cluster{
		T:           t,
		PostgresDSN: dsn,
	}
}

// WithRedis starts Redis and attaches it to the cluster.
func (c *Cluster) WithRedis() *Cluster {
	c.T.Helper()
	c.RedisAddr = StartRedis(context.Background(), c.T)
	return c
}

// WithElasticsearch starts Elasticsearch and attaches it to the cluster.
func (c *Cluster) WithElasticsearch() *Cluster {
	c.T.Helper()
	ctx := context.Background()
	url, cleanup := StartElasticsearch(ctx, c.T)
	c.T.Cleanup(cleanup)
	c.ESURL = url
	return c
}

// WithClickHouse starts ClickHouse and attaches it to the cluster.
func (c *Cluster) WithClickHouse() *Cluster {
	c.T.Helper()
	c.CHDSN = StartClickHouse(context.Background(), c.T)
	return c
}

// WithKafka starts Kafka (Redpanda) and attaches it to the cluster.
func (c *Cluster) WithKafka() *Cluster {
	c.T.Helper()
	c.KafkaAddr = StartKafka(context.Background(), c.T)
	return c
}

// RunMigrations is a convenience wrapper around the package-level RunMigrations.
func (c *Cluster) RunMigrations(dirs ...string) {
	c.T.Helper()
	RunMigrations(context.Background(), c.T, c.PostgresDSN, dirs...)
}
