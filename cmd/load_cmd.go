package cmd

import (
	"context"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
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
	// Create the server handler
	handler := server.NewHandler(specsDir, Logger, InternalConfig)

	// Process GitHub URL if applicable
	if server.IsGitHubURL(url) {
		// Trim any '@' prefix that might be used
		path := server.TrimGitHubPrefix(url)

		// Convert github.com URL to raw.githubusercontent.com
		ghPath, err := server.ConvertGitHubURLToRaw(path, InternalConfig.GitHub.Token)
		if err != nil {
			Logger.Fatalw("Failed to process GitHub URL", "error", err, "url", url)
		}
		url = ghPath
		Logger.Debugw("Converted GitHub URL", "url", url)
	}

	// Load the spec
	Logger.Infow("Loading OpenAPI spec", "url", url)
	spec, err := handler.LoadOpenAPISpec(context.Background(), url)
	if err != nil {
		Logger.Fatalw("Failed to load OpenAPI spec", "error", err, "url", url)
	}

	// Generate a unique ID for this spec
	specID := spec.Info.Title // Use title instead of filename for better identification
	handler.AddSpec(specID, &server.APISpec{
		URL:  url,
		Spec: spec,
	})

	// Save the spec to disk
	if err := handler.SaveSpec(specID, handler.GetSpecs()[specID]); err != nil {
		Logger.Warnw("Failed to save spec", "error", err, "specID", specID)
	}

	Logger.Infow("Successfully loaded spec",
		"id", specID,
		"title", spec.Info.Title,
		"version", spec.Info.Version)

	if spec.Paths != nil {
		paths := spec.Paths.Map()
		Logger.Infow("Found endpoints", "count", len(paths))

		// Print a few sample endpoints
		count := 0
		for path, item := range paths {
			for method, operation := range item.Operations() {
				Logger.Infow("Endpoint",
					"method", method,
					"path", path,
					"summary", operation.Summary)
				count++
				if count >= 5 {
					Logger.Info("... and more endpoints")
					return
				}
			}
		}
	} else {
		Logger.Warn("No paths found in spec")
	}
}
