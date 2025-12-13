package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wfaler/rig/internal/config"
	"github.com/wfaler/rig/internal/docker"
	"github.com/wfaler/rig/internal/dockerfile"
	"github.com/wfaler/rig/internal/project"
)

const configFileName = ".rig.yml"

// runSession handles the complete flow of loading config, building image,
// creating container, and attaching to run a command
func runSession(command []string) error {
	ctx := context.Background()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Load config
	configPath := filepath.Join(cwd, configFileName)
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Expand environment variables
	cfg.ExpandEnvVars()

	// Use configured shell if no command specified
	if len(command) == 0 {
		command = []string{"/bin/" + cfg.GetShell()}
	}

	// Generate project name and image reference
	projectName := project.GetProjectName(cwd)
	configHash, err := project.ComputeConfigHash(configPath)
	if err != nil {
		return fmt.Errorf("computing config hash: %w", err)
	}
	imageRef := project.ImageRef(projectName, configHash)
	containerName := project.ContainerName(projectName)

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// Check if image exists
	imageExists, err := dockerClient.ImageExists(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("checking image: %w", err)
	}

	if !imageExists {
		// Generate Dockerfile
		fmt.Printf("Building image %s...\n", imageRef)
		dockerfileContent, err := dockerfile.Generate(cfg)
		if err != nil {
			return fmt.Errorf("generating dockerfile: %w", err)
		}

		// Build image
		if err := dockerClient.BuildImage(ctx, dockerfileContent, imageRef); err != nil {
			return fmt.Errorf("building image: %w", err)
		}
		fmt.Println("Image built successfully")
	}

	// Find or create container
	containerID, err := dockerClient.FindContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("finding container: %w", err)
	}

	// Check if container exists but with different image
	if containerID != "" {
		currentImage, err := dockerClient.GetContainerImage(ctx, containerID)
		if err != nil {
			return fmt.Errorf("getting container image: %w", err)
		}

		if currentImage != imageRef {
			// Remove old container to recreate with new image
			fmt.Printf("Config changed, recreating container...\n")
			if err := dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
				return fmt.Errorf("removing old container: %w", err)
			}
			containerID = ""
		}
	}

	if containerID == "" {
		// Create new container
		fmt.Printf("Creating container %s...\n", containerName)
		containerID, err = dockerClient.CreateContainer(ctx, docker.ContainerConfig{
			ImageRef:      imageRef,
			ContainerName: containerName,
			WorkDir:       cwd,
			Ports:         cfg.GetAllPorts(),
			Env:           cfg.Env,
			Command:       command,
		})
		if err != nil {
			return fmt.Errorf("creating container: %w", err)
		}
	}

	// Start container if not running
	running, err := dockerClient.IsContainerRunning(ctx, containerID)
	if err != nil {
		return fmt.Errorf("checking container status: %w", err)
	}

	if !running {
		fmt.Printf("Starting container...\n")
		if err := dockerClient.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("starting container: %w", err)
		}
	}

	// Attach to container
	if err := dockerClient.Attach(ctx, containerID, command); err != nil {
		return fmt.Errorf("attaching to container: %w", err)
	}

	return nil
}
