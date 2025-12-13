package docker

import "context"

// DockerClient defines the interface for Docker operations
// This interface enables mocking for testing
type DockerClient interface {
	// Ping verifies Docker daemon is reachable
	Ping(ctx context.Context) error

	// Close releases resources
	Close() error

	// ImageExists checks if an image with the given ref exists locally
	ImageExists(ctx context.Context, imageRef string) (bool, error)

	// BuildImage builds a Docker image from a Dockerfile string
	BuildImage(ctx context.Context, dockerfile string, imageRef string) error

	// FindContainer returns container ID if it exists, empty string otherwise
	FindContainer(ctx context.Context, name string) (string, error)

	// CreateContainer creates a new container
	CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)

	// StartContainer starts an existing container
	StartContainer(ctx context.Context, containerID string) error

	// StopContainer stops a running container
	StopContainer(ctx context.Context, containerID string) error

	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// IsContainerRunning checks if a container is currently running
	IsContainerRunning(ctx context.Context, containerID string) (bool, error)

	// GetContainerImage returns the image reference used by a container
	GetContainerImage(ctx context.Context, containerID string) (string, error)

	// Attach connects stdin/stdout to a container with TTY support
	Attach(ctx context.Context, containerID string, command []string) error
}

// ContainerConfig holds container creation options
type ContainerConfig struct {
	ImageRef      string            // Image reference (name:tag)
	ContainerName string            // Container name
	WorkDir       string            // Host directory to mount as /workspace
	Ports         []string          // Port mappings ("host:container" or "port")
	Env           map[string]string // Environment variables
	Command       []string          // Command to run
}
