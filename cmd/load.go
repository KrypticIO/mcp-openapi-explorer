package cmd

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load [url]",
	Short: "Load an OpenAPI specification from a URL",
	Long: `Load an OpenAPI specification from a URL and register it with the MCP server.
The URL can be:
- HTTP/HTTPS URL (e.g., https://petstore3.swagger.io/api/v3/openapi.json)
- GitHub URL (e.g., https://github.com/BACtrack/bacstack-api/blob/main/apps/core-api/openapi/api.yaml)
- Local file path (prefix with file:// for absolute paths)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		loadSpec(url)
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}

func loadSpec(url string) {
	// Create the MCP handler
	handler := NewMCPHandler()

	// Load the spec
	Logger.Infof("Loading OpenAPI spec from URL: %s", url)
	spec, err := handler.loadOpenAPISpec(context.Background(), url)
	if err != nil {
		Logger.Fatalf("Failed to load OpenAPI spec: %v", err)
	}

	// Generate a unique ID for this spec
	specID := filepath.Base(url)
	handler.specs[specID] = &APISpec{
		URL:  url,
		Spec: spec,
	}

	Logger.Infof("Successfully loaded spec with ID: %s", specID)
	Logger.Infof("Title: %s", spec.Info.Title)
	Logger.Infof("Version: %s", spec.Info.Version)

	if spec.Paths != nil {
		paths := spec.Paths.Map()
		Logger.Infof("Found %d endpoints", len(paths))

		// Print a few sample endpoints
		count := 0
		for path, item := range paths {
			for method, operation := range item.Operations() {
				Logger.Infof("Endpoint: %s %s", method, path)
				Logger.Infof("  Summary: %s", operation.Summary)
				count++
				if count >= 5 {
					Logger.Infof("... and more endpoints")
					return
				}
			}
		}
	} else {
		Logger.Warn("No paths found in spec")
	}
}
