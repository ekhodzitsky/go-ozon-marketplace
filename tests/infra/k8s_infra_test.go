package infra_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var repoRoot = filepath.Join("..", "..")

type serviceDef struct {
	name         string
	usesPostgres bool
}

var services = []serviceDef{
	{name: "api-gateway", usesPostgres: false},
	{name: "user-service", usesPostgres: true},
	{name: "catalog-service", usesPostgres: true},
	{name: "inventory-service", usesPostgres: true},
	{name: "payment-service", usesPostgres: true},
	{name: "order-service", usesPostgres: true},
	{name: "notification-service", usesPostgres: false},
	{name: "analytics-service", usesPostgres: false},
}

func decodeDocs(t *testing.T, data []byte) []map[string]interface{} {
	t.Helper()
	var docs []map[string]interface{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc map[string]interface{}
		if err := dec.Decode(&doc); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode yaml: %v", err)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs
}

func kindDocs(docs []map[string]interface{}, kind string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, d := range docs {
		if k, ok := d["kind"].(string); ok && k == kind {
			out = append(out, d)
		}
	}
	return out
}

func firstContainer(t *testing.T, deployment map[string]interface{}) map[string]interface{} {
	t.Helper()
	spec := deployment["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	podSpec := template["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	require.NotEmpty(t, containers, "expected at least one container")
	return containers[0].(map[string]interface{})
}

func envByName(container map[string]interface{}, name string) (string, bool) {
	env, ok := container["env"].([]interface{})
	if !ok {
		return "", false
	}
	for _, e := range env {
		entry := e.(map[string]interface{})
		if entry["name"] == name {
			v, _ := entry["value"].(string)
			return v, true
		}
	}
	return "", false
}

func portByName(container map[string]interface{}, name string) (int64, bool) {
	ports, ok := container["ports"].([]interface{})
	if !ok {
		return 0, false
	}
	for _, p := range ports {
		port := p.(map[string]interface{})
		if port["name"] == name {
			switch v := port["containerPort"].(type) {
			case int64:
				return v, true
			case int:
				return int64(v), true
			case float64:
				return int64(v), true
			}
		}
	}
	return 0, false
}

func servicePortByName(svc map[string]interface{}, name string) (int64, bool) {
	spec := svc["spec"].(map[string]interface{})
	ports, ok := spec["ports"].([]interface{})
	if !ok {
		return 0, false
	}
	for _, p := range ports {
		port := p.(map[string]interface{})
		if port["name"] == name {
			switch v := port["port"].(type) {
			case int64:
				return v, true
			case int:
				return int64(v), true
			case float64:
				return int64(v), true
			}
		}
	}
	return 0, false
}

func TestHelmServiceTemplates(t *testing.T) {
	t.Parallel()
	for _, svc := range services {
		svc := svc
		t.Run(svc.name, func(t *testing.T) {
			t.Parallel()
			chartPath := filepath.Join(repoRoot, "infra", "k8s", "helm-charts", svc.name)
			cmd := exec.Command("helm", "template", svc.name, chartPath,
				"--set", "image.tag=ci",
				"--set", "secrets.jwtSecret=dummy",
				"--set", "secrets.postgresDSN=dummy",
			)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "helm template failed: %s", string(out))

			docs := decodeDocs(t, out)

			// Deployment checks.
			deps := kindDocs(docs, "Deployment")
			require.Len(t, deps, 1, "expected exactly one Deployment")
			container := firstContainer(t, deps[0])

			metricsPort, ok := portByName(container, "metrics")
			require.True(t, ok, "metrics port not exposed in Deployment")
			grpcPort, ok := portByName(container, "grpc")
			if !ok {
				// api-gateway uses the same port for http/grpc.
				grpcPort, ok = portByName(container, "http")
				require.True(t, ok, "neither grpc nor http port exposed")
			}
			require.Equal(t, grpcPort+1000, metricsPort, "metricsPort must be grpcPort+1000")

			met, ok := envByName(container, "METRICS_PORT")
			require.True(t, ok, "METRICS_PORT env not set")
			require.Equal(t, fmt.Sprintf("%d", metricsPort), met)

			otel, ok := envByName(container, "OTEL_EXPORTER_OTLP_ENDPOINT")
			require.True(t, ok, "OTEL_EXPORTER_OTLP_ENDPOINT env not set")
			require.Equal(t, "http://jaeger.monitoring.svc.cluster.local:4318", otel)

			// Service checks.
			svcs := kindDocs(docs, "Service")
			require.Len(t, svcs, 1, "expected exactly one Service")
			svcSpec := svcs[0]
			annotations := svcSpec["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
			require.Equal(t, "true", annotations["prometheus.io/scrape"], "prometheus scrape annotation missing")
			require.Equal(t, fmt.Sprintf("%d", metricsPort), annotations["prometheus.io/port"], "prometheus port annotation mismatch")

			svcMetricsPort, ok := servicePortByName(svcSpec, "metrics")
			require.True(t, ok, "metrics port not exposed in Service")
			require.Equal(t, metricsPort, svcMetricsPort)

			// Secret checks.
			secrets := kindDocs(docs, "Secret")
			if !svc.usesPostgres {
				require.Len(t, secrets, 1, "expected exactly one Secret")
				stringData, ok := secrets[0]["stringData"].(map[string]interface{})
				require.True(t, ok)
				_, found := stringData["POSTGRES_DSN"]
				require.False(t, found, "POSTGRES_DSN should not be present for %s", svc.name)
			} else {
				require.NotEmpty(t, secrets, "expected a Secret for postgres-backed service")
			}
		})
	}
}

func TestArgoCDApplicationSet(t *testing.T) {
	t.Parallel()
	path := filepath.Join(repoRoot, "infra", "k8s", "argocd", "applicationset-marketplace.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var app map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &app))
	require.Equal(t, "ApplicationSet", app["kind"])
	metadata := app["metadata"].(map[string]interface{})
	require.Equal(t, "marketplace", metadata["name"])
	require.Equal(t, "argocd", metadata["namespace"])

	spec := app["spec"].(map[string]interface{})
	generators := spec["generators"].([]interface{})
	require.NotEmpty(t, generators)
	list := generators[0].(map[string]interface{})["list"].(map[string]interface{})
	elements := list["elements"].([]interface{})
	require.Len(t, elements, len(services))

	template := spec["template"].(map[string]interface{})
	templateSpec := template["spec"].(map[string]interface{})
	dest := templateSpec["destination"].(map[string]interface{})
	require.Equal(t, "marketplace-staging", dest["namespace"])

	source := templateSpec["source"].(map[string]interface{})
	require.Equal(t, "infra/k8s/helm-charts/{{ .service }}", source["path"])
}

func TestNetworkPoliciesNamespace(t *testing.T) {
	t.Parallel()
	npDir := filepath.Join(repoRoot, "infra", "k8s", "network-policies")
	entries, err := os.ReadDir(npDir)
	require.NoError(t, err)

	var foundEgress bool
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(npDir, e.Name()))
		require.NoError(t, err)
		docs := decodeDocs(t, data)
		for _, d := range docs {
			if k, _ := d["kind"].(string); k != "NetworkPolicy" {
				continue
			}
			meta := d["metadata"].(map[string]interface{})
			require.Equal(t, "marketplace-staging", meta["namespace"], "%s: namespace mismatch", e.Name())
			if name, _ := meta["name"].(string); name == "services-egress" {
				foundEgress = true
				spec := d["spec"].(map[string]interface{})
				require.Contains(t, spec["policyTypes"], "Egress")
			}
		}
	}
	require.True(t, foundEgress, "services-egress NetworkPolicy not found")
}

func TestJaegerOTLPPorts(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"deployment.yaml", "service.yaml"} {
		path := filepath.Join(repoRoot, "infra", "k8s", "monitoring", "jaeger", file)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		docs := decodeDocs(t, data)
		require.Len(t, docs, 1)

		var ports []interface{}
		switch file {
		case "deployment.yaml":
			container := firstContainer(t, docs[0])
			ports = container["ports"].([]interface{})
		case "service.yaml":
			spec := docs[0]["spec"].(map[string]interface{})
			ports = spec["ports"].([]interface{})
		}

		names := make(map[string]bool)
		for _, p := range ports {
			port := p.(map[string]interface{})
			names[port["name"].(string)] = true
		}
		require.True(t, names["otlp-grpc"], "%s: otlp-grpc port missing", file)
		require.True(t, names["otlp-http"], "%s: otlp-http port missing", file)
	}
}

func TestDockerComposeJaegerAndOTEL(t *testing.T) {
	t.Parallel()
	composeDir := filepath.Join(repoRoot, "infra", "docker")
	cmd := exec.Command("docker", "compose",
		"-f", "docker-compose.yml",
		"-f", "docker-compose.dev.yml",
		"config",
	)
	cmd.Dir = composeDir
	cmd.Env = append(os.Environ(),
		"JWT_SECRET=dummy",
		"POSTGRES_PASSWORD=dummy",
		"GF_ADMIN_PASSWORD=dummy",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "docker compose config failed: %s", string(out))

	var cfg map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &cfg))
	servicesMap := cfg["services"].(map[string]interface{})

	jaeger, ok := servicesMap["jaeger"].(map[string]interface{})
	require.True(t, ok, "jaeger service missing")
	jaegerPorts := jaeger["ports"].([]interface{})
	targets := make(map[int64]bool)
	for _, p := range jaegerPorts {
		port := p.(map[string]interface{})
		switch v := port["target"].(type) {
		case int64:
			targets[v] = true
		case int:
			targets[int64(v)] = true
		case float64:
			targets[int64(v)] = true
		}
	}
	require.True(t, targets[4317], "jaeger OTLP gRPC port 4317 missing")
	require.True(t, targets[4318], "jaeger OTLP HTTP port 4318 missing")

	for _, svc := range services {
		svcCfg, ok := servicesMap[svc.name].(map[string]interface{})
		require.True(t, ok, "%s service missing in compose config", svc.name)
		env, ok := svcCfg["environment"].(map[string]interface{})
		require.True(t, ok, "%s: environment missing", svc.name)
		require.Equal(t, "http://jaeger:4318", env["OTEL_EXPORTER_OTLP_ENDPOINT"], "%s: OTEL endpoint mismatch", svc.name)
	}
}
