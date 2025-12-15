package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"
	"github.com/wfaler/rig/internal/docker"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running rig containers",
	Long: `Lists all running rig containers.

Shows the container name, status, and image for each running rig container.`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// List all rig containers
	containers, err := dockerClient.ListRigContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No running rig containers")
		return nil
	}

	// Print in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tIMAGE")
	for _, c := range containers {
		// Extract project name from container name (remove "rig-" prefix)
		name := c.Name
		if len(name) > 4 {
			name = name[4:] // Remove "rig-" prefix for cleaner display
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, c.Status, c.Image)
	}
	w.Flush()

	return nil
}
