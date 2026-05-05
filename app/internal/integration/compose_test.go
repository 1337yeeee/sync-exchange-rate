package integration

import (
	"os"
	"strings"
	"testing"
)

func TestComposeDefinesIndependentAppSchedulerAndPostgres(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("../../../docker-compose.yml")
	if err != nil {
		t.Fatalf("ReadFile(docker-compose.yml) error = %v", err)
	}

	compose := string(content)
	requiredFragments := []string{
		"  postgres:",
		"  app:",
		"  scheduler:",
		"command: [\"/usr/local/bin/scheduler\"]",
		"postgres_data:",
		"condition: service_healthy",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(compose, fragment) {
			t.Fatalf("docker-compose.yml does not contain %q", fragment)
		}
	}
}
