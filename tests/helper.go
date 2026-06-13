package tests

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var portMu sync.Mutex

// AuthContext returns a gRPC outgoing context with a signed JWT bearer token.
// The claims mirror the ones validated by the service auth middleware.
func AuthContext(ctx context.Context, userID, secret string) context.Context {
	now := time.Now().UTC()
	claims := middleware.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Role: string(middleware.RoleUser),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(fmt.Sprintf("failed to sign token: %v", err))
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
}

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

// CreateDatabase creates a new database on the Postgres server identified by adminDSN
// and returns a DSN for the new database. It drops the database if it already exists.
func CreateDatabase(ctx context.Context, t *testing.T, adminDSN, dbName string) string {
	t.Helper()

	pool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatalf("failed to connect to admin database: %v", err)
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %q", dbName))
	_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", dbName))
	if err != nil {
		t.Fatalf("failed to create database %s: %v", dbName, err)
	}

	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("failed to parse admin dsn: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
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

			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("failed to begin transaction for %s: %v", file, err)
			}
			_, err = tx.Exec(ctx, string(content))
			if err != nil {
				_ = tx.Rollback(ctx)
				t.Fatalf("failed to execute migration %s: %v", file, err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatalf("failed to commit migration %s: %v", file, err)
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
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func streamToLog(t *testing.T, wg *sync.WaitGroup, r io.ReadCloser) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t.Log(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Logf("service log scanner error: %v", err)
	}
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe for %s: %v", serviceDir, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe for %s: %v", serviceDir, err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start service %s: %v", serviceDir, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamToLog(t, &wg, stdout)
	go streamToLog(t, &wg, stderr)

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
		wg.Wait()
	})
	return cmd
}

// WaitForGRPC waits for a gRPC server to become ready.
// It prefers a health check and falls back to a raw TCP connection for
// servers that do not implement the gRPC health protocol.
func WaitForGRPC(t *testing.T, addr string) {
	t.Helper()
	WaitForGRPCHealth(t, addr)
}

// WaitForGRPCHealth performs a gRPC health check with exponential backoff.
// If the server does not implement health checks, it falls back to a raw
// TCP connection so that mocks and minimal servers still work.
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
		defer func() { _ = conn.Close() }()
		client := healthpb.NewHealthClient(conn)
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Unimplemented {
				tcpConn, err := net.Dial("tcp", addr)
				if err != nil {
					return err
				}
				_ = tcpConn.Close()
				return nil
			}
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
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
		return nil
	}, b)
	if err != nil {
		t.Fatalf("HTTP server did not start at %s: %v", url, err)
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
