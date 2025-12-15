package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wfaler/rig/internal/docker"
	"github.com/wfaler/rig/internal/project"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [name]",
	Short: "Stop container and remove all associated images",
	Long: `Completely removes the rig container and all associated images.

If [name] is provided, destroys the container and images for that project.
Otherwise, destroys the container and images for the current directory.

This is a destructive operation - the container state will be lost and
images will need to be rebuilt on next 'rig up'.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDestroy,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

func runDestroy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var projectName string
	if len(args) > 0 {
		// Use provided name
		projectName = args[0]
	} else {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		projectName = project.GetProjectName(cwd)
	}

	containerName := project.ContainerName(projectName)
	imageName := project.ImageName(projectName)

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// Find and remove container
	containerID, err := dockerClient.FindContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("finding container: %w", err)
	}

	if containerID != "" {
		// Check if running and stop first
		running, err := dockerClient.IsContainerRunning(ctx, containerID)
		if err != nil {
			return fmt.Errorf("checking container status: %w", err)
		}
		if running {
			fmt.Printf("Stopping container %s...\n", containerName)
			if err := dockerClient.StopContainer(ctx, containerID); err != nil {
				return fmt.Errorf("stopping container: %w", err)
			}
			// Wait for container to fully stop before removing
			_ = dockerClient.WaitContainer(ctx, containerID)
		}

		fmt.Printf("Removing container %s...\n", containerName)
		if err := dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
			return fmt.Errorf("removing container: %w", err)
		}
	} else {
		fmt.Printf("No container found for project %s\n", projectName)
	}

	// Remove all images with this project name
	fmt.Printf("Removing images matching %s...\n", imageName)
	if err := dockerClient.RemoveImagesByName(ctx, imageName); err != nil {
		fmt.Printf("Note: %v\n", err)
	}

	fmt.Printf("Project %s destroyed.\n", projectName)
	return nil
}
