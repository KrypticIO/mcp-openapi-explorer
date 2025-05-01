package tools

import (
	"context"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterLoadAPISpec registers the load_api_spec tool
func RegisterLoadAPISpec(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface, config server.ConfigInterface) {
	// Create the load_api_spec tool
	loadSpecTool := mcp.NewTool(
		"load_api_spec",
		mcp.WithDescription("Load an OpenAPI specification from a URL or file path"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL or file path to the OpenAPI spec (e.g. 'https://petstore3.swagger.io/api/v3/openapi.json' or 'file:///path/to/spec.json')"),
		),
	)

	// Register the load_api_spec tool
	mcpServer.AddTool(loadSpecTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, ok := req.Params.Arguments["url"].(string)
		if !ok {
			return mcp.NewToolResultError("url parameter must be a string"), nil
		}

		// Handle GitHub repositories
		if server.IsGitHubURL(url) {
			// Trim any '@' prefix that might be used
			path := server.TrimGitHubPrefix(url)

			// Use GitHub token from config
			ghToken := config.GetGitHubToken()

			// Convert github.com URL to raw.githubusercontent.com
			ghPath, err := server.ConvertGitHubURLToRaw(path, ghToken)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to process GitHub URL: %v", err)), nil
			}
			url = ghPath
			logger.Debugw("Converted GitHub URL", "path", url)
		}

		// Load the OpenAPI spec
		spec, err := handler.LoadOpenAPISpec(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load API spec: %v", err)), nil
		}

		// Generate a unique ID for this spec
		specID := spec.Info.Title
		handler.AddSpec(specID, &server.APISpec{
			URL:  url,
			Spec: spec,
		})

		// Save the spec to a file for persistence
		if err := handler.SaveSpec(specID, handler.GetSpecs()[specID]); err != nil {
			logger.Warnw("Failed to save spec", "error", err, "specID", specID)
		}

		// Update configuration with the new spec if config supports it
		if config.ShouldUpdateConfig() {
			// Only add to config if not already there
			if !config.HasSpec(url) {
				config.AddSpec(url)

				// Save the updated configuration
				if err := config.Save(); err != nil {
					logger.Warnw("Failed to update config file with new spec", "error", err)
				} else {
					logger.Infow("Updated config file with new spec", "url", url)
				}
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully loaded API spec: %s (version %s)", spec.Info.Title, spec.Info.Version)), nil
	})
}
