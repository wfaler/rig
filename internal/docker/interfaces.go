package docker

import "context"

// EnvHostWorkdir is the container-side environment variable holding the host
// path mounted at /workspace. In-container tooling uses it to reference the
// project by its host path (e.g. agents spawning sibling herdr panes with the
// correct --cwd).
const EnvHostWorkdir = "RIG_HOST_WORKDIR"

// DockerClient defines the interface for Docker operations
// This interface enables mocking for testing
type DockerClient interface {
	// Ping verifies Docker daemon is reachable
	Ping(ctx context.Context) error

	// Close releases resources
	Close() error

	// ImageExists checks if an image with the given ref exists locally
	ImageExists(ctx context.Context, imageRef string) (bool, error)

	// BuildImage builds a Docker image from a Dockerfile string and optional extra files
	BuildImage(ctx context.Context, dockerfile string, imageRef string, extraFiles map[string][]byte) error

	// FindContainer returns container ID if it exists, empty string otherwise
	FindContainer(ctx context.Context, name string) (string, error)

	// CreateContainer creates a new container
	CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)

	// StartContainer starts an existing container
	StartContainer(ctx context.Context, containerID string) error

	// StopContainer stops a running container
	StopContainer(ctx context.Context, containerID string) error

	// WaitContainer waits for a container to stop
	WaitContainer(ctx context.Context, containerID string) error

	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// IsContainerRunning checks if a container is currently running
	IsContainerRunning(ctx context.Context, containerID string) (bool, error)

	// GetContainerImage returns the image reference used by a container
	GetContainerImage(ctx context.Context, containerID string) (string, error)

	// GetHerdrSocketHostPath returns the host source path of the herdr socket
	// bind mount for a container, or "" if the container has no such mount.
	GetHerdrSocketHostPath(ctx context.Context, containerID string) (string, error)

	// GetDockerSocketHostPath returns the host source path bind-mounted at
	// ContainerDockerSocket for a container, or "" if it has no such mount.
	GetDockerSocketHostPath(ctx context.Context, containerID string) (string, error)

	// GetContainerEnvValue returns the value of an environment variable baked
	// into a container's config at create time, or "" if unset.
	GetContainerEnvValue(ctx context.Context, containerID string, key string) (string, error)

	// Attach connects stdin/stdout to a container with TTY support, injecting
	// the given extra environment variables into the exec session.
	Attach(ctx context.Context, containerID string, command []string, env map[string]string) error
}

// ContainerConfig holds container creation options
type ContainerConfig struct {
	ImageRef      string            // Image reference (name:tag)
	ContainerName string            // Container name
	WorkDir       string            // Host directory to mount as /workspace
	Ports         []string          // Port mappings ("host:container" or "port")
	Env           map[string]string // Environment variables
	Command       []string          // Command to run

	// DockerSocketHostPath, when non-empty, is the path of the engine socket
	// to bind-mount at ContainerDockerSocket for Docker-in-Docker
	// (testcontainers). Resolve it with HostDockerSocket; an empty value
	// creates the container without a socket mount.
	DockerSocketHostPath string

	// HerdrSocketHostPath, when non-empty, is the host path of the herdr
	// control socket to bind-mount into the container at
	// herdr.ContainerSocketPath so the containerized agent can reach herdr.
	// Used on Linux, where unix sockets can cross the bind mount directly.
	HerdrSocketHostPath string

	// HerdrProxyPort, when non-zero, is the host TCP port bridging to the
	// herdr control socket. It is exported to the container as
	// herdr.EnvProxyPort so the entrypoint can re-expose it as a unix socket
	// via socat. Used on macOS, where sockets cannot cross the VM boundary.
	HerdrProxyPort int
}
