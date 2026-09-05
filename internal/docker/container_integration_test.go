//go:build integration

package docker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// engineImage is a small image the local engine is expected to have or be able
// to pull; it only needs a shell.
const engineImage = "debian:bookworm-slim"

// TestCreateContainerMountsResolvedSocket verifies against the real engine
// (Docker or Podman, rootful or rootless) that the socket path resolved by
// HostDockerSocket can actually be bind-mounted and the container started.
// Mounting a socket the engine cannot statfs — the rootless Podman case, where
// /var/run/docker.sock symlinks to the root-owned /run/podman/podman.sock —
// fails container creation with "permission denied".
func TestCreateContainerMountsResolvedSocket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := New()
	require.NoError(t, err, "creating engine client")
	defer client.Close()
	require.NoError(t, client.Ping(ctx), "engine must be reachable")

	exists, err := client.ImageExists(ctx, engineImage)
	require.NoError(t, err)
	if !exists {
		t.Skipf("image %s not present locally; pull it to run this test", engineImage)
	}

	hostSocket := HostDockerSocket()
	require.NotEmpty(t, hostSocket, "no reachable engine socket resolved despite a live engine")

	const name = "rig-integration-socket-test"
	if existing, findErr := client.FindContainer(ctx, name); findErr == nil && existing != "" {
		require.NoError(t, client.RemoveContainer(ctx, existing, true))
	}

	containerID, err := client.CreateContainer(ctx, ContainerConfig{
		ImageRef:             engineImage,
		ContainerName:        name,
		WorkDir:              t.TempDir(),
		Command:              []string{"sleep", "60"},
		DockerSocketHostPath: hostSocket,
	})
	require.NoError(t, err, "creating container with socket mount %s", hostSocket)
	t.Cleanup(func() {
		_ = client.RemoveContainer(context.Background(), containerID, true)
	})

	require.NoError(t, client.StartContainer(ctx, containerID), "starting container")

	running, err := client.IsContainerRunning(ctx, containerID)
	require.NoError(t, err)
	assert.True(t, running, "container should be running")

	mounted, err := client.GetDockerSocketHostPath(ctx, containerID)
	require.NoError(t, err)
	assert.Equal(t, hostSocket, mounted, "inspected mount should match the resolved socket")
}

// TestCreateContainerWithoutSocketMount verifies a container still comes up
// when no engine socket is available (DinD simply unavailable inside).
func TestCreateContainerWithoutSocketMount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := New()
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.Ping(ctx))

	exists, err := client.ImageExists(ctx, engineImage)
	require.NoError(t, err)
	if !exists {
		t.Skipf("image %s not present locally; pull it to run this test", engineImage)
	}

	const name = "rig-integration-nosocket-test"
	if existing, findErr := client.FindContainer(ctx, name); findErr == nil && existing != "" {
		require.NoError(t, client.RemoveContainer(ctx, existing, true))
	}

	containerID, err := client.CreateContainer(ctx, ContainerConfig{
		ImageRef:      engineImage,
		ContainerName: name,
		WorkDir:       t.TempDir(),
		Command:       []string{"sleep", "60"},
		// DockerSocketHostPath deliberately empty.
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.RemoveContainer(context.Background(), containerID, true)
	})

	require.NoError(t, client.StartContainer(ctx, containerID))

	mounted, err := client.GetDockerSocketHostPath(ctx, containerID)
	require.NoError(t, err)
	assert.Empty(t, mounted, "no socket mount should be present")
}
