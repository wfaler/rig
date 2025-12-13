package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wfaler/rig/internal/config"
	"github.com/wfaler/rig/internal/docker"
	"github.com/wfaler/rig/internal/dockerfile"
	"github.com/wfaler/rig/internal/project"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Force a clean rebuild of the rig image",
	Long: `Removes the existing container and image, then rebuilds from scratch.

Use this when:
  - The image seems corrupted or outdated
  - You want to ensure a fresh build
  - Dockerfile template changes aren't being picked up`,
	RunE: runRebuild,
}

func init() {
	rootCmd.AddCommand(rebuildCmd)
}

func runRebuild(cmd *cobra.Command, args []string) error {
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

	// Generate project name and image reference
	projectName := project.GetProjectName(cwd)
	configHash, err := project.ComputeConfigHash(configPath)
	if err != nil {
		return fmt.Errorf("computing config hash: %w", err)
	}
	imageRef := project.ImageRef(projectName, configHash)
	containerName := project.ContainerName(projectName)
	imageName := project.ImageName(projectName)

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// Remove existing container
	containerID, err := dockerClient.FindContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("finding container: %w", err)
	}
	if containerID != "" {
		fmt.Printf("Removing container %s...\n", containerName)
		if err := dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
			return fmt.Errorf("removing container: %w", err)
		}
	}

	// Remove all images with this project name
	fmt.Printf("Removing images matching %s...\n", imageName)
	if err := dockerClient.RemoveImagesByName(ctx, imageName); err != nil {
		// Don't fail if images don't exist
		fmt.Printf("Note: %v\n", err)
	}

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

	fmt.Println("Rebuild complete! Run 'rig' to enter the container.")
	return nil
}
