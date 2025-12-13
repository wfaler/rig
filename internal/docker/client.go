package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client and implements DockerClient interface
type Client struct {
	cli *client.Client
}

// Ensure Client implements DockerClient
var _ DockerClient = (*Client)(nil)

// New creates a new Docker client from environment
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close releases resources
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping verifies Docker daemon is reachable
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pinging docker daemon: %w", err)
	}
	return nil
}

// Raw returns the underlying Docker SDK client for advanced operations
func (c *Client) Raw() *client.Client {
	return c.cli
}
