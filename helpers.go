package main

import (
	"context"
	"runtime"
	"testing"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var ESDBcontainerEnv = map[string]string{
	"EVENTSTORE_CLUSTER_SIZE":               "1",
	"EVENTSTORE_RUN_PROJECTIONS":            "All",
	"EVENTSTORE_START_STANDARD_PROJECTIONS": "true",
	"EVENTSTORE_HTTP_PORT":                  "2113",
	"EVENTSTORE_INSECURE":                   "true",
}

type TestSetup struct {
	Container  testcontainers.Container
	MappedPort nat.Port
}

func Setup(t *testing.T, reuse bool) *TestSetup {
	ctx := context.Background()
	network := "customNetwork"
	container, mappedPort, _ := SetupESDBcontainer(t, ctx, &network)

	return &TestSetup{
		Container:  container,
		MappedPort: mappedPort,
	}
}

func SetupESDBcontainer(t *testing.T, ctx context.Context, networkName *string) (testcontainers.Container, nat.Port, string) {
	ESDBcontainerName := "ESDBCONTAINER"
	var networkAliases map[string][]string
	networkAlias := ESDBcontainerName
	networkAliases = make(map[string][]string)
	networkAliases[*networkName] = []string{networkAlias}

	req := testcontainers.ContainerRequest{
		Networks:       []string{*networkName},
		NetworkAliases: networkAliases,
		Image:          GenerateDockerImageName(),
		ExposedPorts:   []string{"2113/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForHTTP("/gossip").WithPort(nat.Port("2113/tcp")).WithStatusCodeMatcher(
				func(status int) bool { return status == 200 }),
			wait.ForHTTP("/stats").WithPort(nat.Port("2113/tcp")).WithStatusCodeMatcher(
				func(status int) bool { return status == 200 }),
		),
		Env:  ESDBcontainerEnv,
		Name: ESDBcontainerName,
	}

	container, containerErr := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            false,
	})

	if containerErr != nil {
		if containerErr != nil {
			t.Fatal("Container error: ", containerErr)
		}
	}

	// Get port
	mappedPort, mappedPortErr := container.MappedPort(ctx, "2113")
	if mappedPortErr != nil {
		t.Fatal("mappedPortErr error: ", mappedPortErr)
	}

	return container, mappedPort, ESDBcontainerName
}

func GenerateDockerImageName() string {
	// Get the current runtime architecture
	arch := runtime.GOARCH

	// Check if the architecture is ARMv8
	isARMv8 := arch == "arm64" || arch == "aarch64"
	var dockertag string
	if isARMv8 {
		dockertag = "24.2.0-alpha-arm64v8"
	} else {
		dockertag = "24.2.0-buster-slim"
	}
	return "eventstore/eventstore:" + dockertag
}

func SetupESDBtestingClient(t *testing.T, mappedPort string) *esdb.Client {
	conf, parseErr := esdb.ParseConnectionString("esdb://localhost:" + mappedPort + "?tls=false")
	if parseErr != nil {
		t.Fatal("parseErr errorr: ", parseErr)
	}

	client, clientErr := esdb.NewClient(conf)
	if clientErr != nil {
		t.Fatal("clientErr error: ", clientErr)
	}

	return client
}
