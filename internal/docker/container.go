package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

// FindContainer returns container ID if it exists, empty string otherwise
func (c *Client) FindContainer(ctx context.Context, name string) (string, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return "", fmt.Errorf("listing containers: %w", err)
	}

	// Container names in Docker API are prefixed with "/"
	searchName := "/" + name
	for _, ctr := range containers {
		for _, n := range ctr.Names {
			if n == searchName {
				return ctr.ID, nil
			}
		}
	}
	return "", nil
}

// CreateContainer creates a new container with DinD support
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	// Parse port bindings
	exposedPorts, portBindings, err := parsePortMappings(cfg.Ports)
	if err != nil {
		return "", fmt.Errorf("parsing ports: %w", err)
	}

	// Build environment slice
	envSlice := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	// Container configuration
	containerCfg := &container.Config{
		Image:        cfg.ImageRef,
		Cmd:          cfg.Command,
		Env:          envSlice,
		ExposedPorts: exposedPorts,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/workspace",
	}

	// Host configuration with mounts
	hostCfg := &container.HostConfig{
		Binds: []string{
			// Mount project directory
			fmt.Sprintf("%s:/workspace:rw", cfg.WorkDir),
			// Docker socket for DinD (testcontainers support)
			"/var/run/docker.sock:/var/run/docker.sock",
		},
		PortBindings:  portBindings,
		Privileged:    false, // Socket mount doesn't need privileged mode
		NetworkMode:   "bridge",
		RestartPolicy: container.RestartPolicy{Name: "no"},
		// Add host.docker.internal for Linux (Docker Desktop on Mac/Windows adds this automatically)
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	resp, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, cfg.ContainerName)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts an existing container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	return nil
}

// StopContainer stops a running container
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	return nil
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if err := c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	return nil
}

// IsContainerRunning checks if a container is currently running
func (c *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("inspecting container: %w", err)
	}
	return info.State.Running, nil
}

// GetContainerImage returns the image reference used by a container
func (c *Client) GetContainerImage(ctx context.Context, containerID string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspecting container: %w", err)
	}
	return info.Config.Image, nil
}

// parsePortMappings converts port specs to Docker port structures
func parsePortMappings(ports []string) (nat.PortSet, nat.PortMap, error) {
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}

	for _, spec := range ports {
		parts := strings.Split(spec, ":")

		var hostPort, containerPort string
		switch len(parts) {
		case 1:
			// Single port: same on host and container
			hostPort = parts[0]
			containerPort = parts[0]
		case 2:
			// host:container mapping
			hostPort = parts[0]
			containerPort = parts[1]
		default:
			return nil, nil, fmt.Errorf("invalid port spec: %s", spec)
		}

		// Validate ports are numbers
		if _, err := strconv.Atoi(hostPort); err != nil {
			return nil, nil, fmt.Errorf("invalid host port: %s", hostPort)
		}
		if _, err := strconv.Atoi(containerPort); err != nil {
			return nil, nil, fmt.Errorf("invalid container port: %s", containerPort)
		}

		natPort := nat.Port(containerPort + "/tcp")
		exposedPorts[natPort] = struct{}{}
		portBindings[natPort] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}
	}

	return exposedPorts, portBindings, nil
}
