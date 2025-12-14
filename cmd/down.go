package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wfaler/rig/internal/docker"
	"github.com/wfaler/rig/internal/project"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the rig container without destroying it",
	Long: `Stops the running container while preserving its state.

The container is stopped but not removed, so any state inside the container
(installed packages, files outside /workspace, etc.) will be preserved.

Use 'rig' to start the container again.
Use 'rig rebuild' if you want to completely remove and rebuild the container.`,
	RunE: runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Generate project name and container name
	projectName := project.GetProjectName(cwd)
	containerName := project.ContainerName(projectName)

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// Find existing container
	containerID, err := dockerClient.FindContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("finding container: %w", err)
	}

	if containerID == "" {
		fmt.Printf("No container found for project %s\n", projectName)
		return nil
	}

	// Check if container is running
	running, err := dockerClient.IsContainerRunning(ctx, containerID)
	if err != nil {
		return fmt.Errorf("checking container status: %w", err)
	}

	if !running {
		fmt.Printf("Container %s is already stopped\n", containerName)
		return nil
	}

	// Stop the container
	fmt.Printf("Stopping container %s...\n", containerName)
	if err := dockerClient.StopContainer(ctx, containerID); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	// Wait for container to fully stop
	if err := dockerClient.WaitContainer(ctx, containerID); err != nil {
		// Ignore wait errors - container may have already stopped
		_ = err
	}

	fmt.Printf("Container %s stopped. Run 'rig' to start it again.\n", containerName)
	return nil
}
