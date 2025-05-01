package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/github"
	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Common errors for tool operations
var (
	ErrMissingParameter = errors.New("missing required parameter")
	ErrInvalidParameter = errors.New("invalid parameter value")
	ErrSpecLoading      = errors.New("failed to load API spec")
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

	logger.Infow("Registering load_api_spec tool")

	// Register the load_api_spec tool
	mcpServer.AddTool(loadSpecTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract URL parameter
		url, ok := req.Params.Arguments["url"].(string)
		if !ok {
			logger.Errorw("Missing required URL parameter")
			return mcp.NewToolResultError(fmt.Sprintf("%s: url parameter is required and must be a string", ErrMissingParameter)), nil
		}

		if url == "" {
			logger.Errorw("Empty URL parameter provided")
			return mcp.NewToolResultError(fmt.Sprintf("%s: url parameter cannot be empty", ErrInvalidParameter)), nil
		}

		logger.Infow("Processing load_api_spec request", "url", url)

		// Handle GitHub repositories
		if github.IsGitHubURL(url) {
			logger.Debugw("Detected GitHub URL",
				"url", url,
				"is_github", true)

			// Trim any '@' prefix that might be used
			path := github.TrimGitHubPrefix(url)

			// Use GitHub token from config
			ghToken := config.GetGitHubToken()
			hasToken := ghToken != ""

			logger.Debugw("Preparing to convert GitHub URL",
				"original_url", path,
				"token_available", hasToken)

			// Convert github.com URL to raw.githubusercontent.com
			ghPath, err := github.ConvertGitHubURLToRaw(path, ghToken)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to process GitHub URL: %v", err)
				logger.Errorw(errMsg,
					"error", err,
					"url", url,
					"path", path)
				return mcp.NewToolResultError(errMsg), nil
			}

			url = ghPath
			logger.Infow("Converted GitHub URL",
				"original_url", path,
				"converted_url", url)
		}

		// Load the OpenAPI spec
		logger.Infow("Loading OpenAPI spec", "url", url)
		spec, err := handler.LoadOpenAPISpec(ctx, url)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to load API spec: %v", err)
			logger.Errorw(errMsg,
				"error", err,
				"url", url)
			return mcp.NewToolResultError(errMsg), nil
		}

		// Generate a unique ID for this spec
		specID := spec.Info.Title
		logger.Infow("Loaded spec successfully",
			"url", url,
			"spec_id", specID,
			"title", spec.Info.Title,
			"version", spec.Info.Version,
			"endpoints", len(spec.Paths.Map()))

		// Add to the handler
		handler.AddSpec(specID, &server.APISpec{
			URL:  url,
			Spec: spec,
		})

		// Save the spec to a file for persistence
		if err := handler.SaveSpec(specID, handler.GetSpecs()[specID]); err != nil {
			logger.Warnw("Failed to save spec",
				"error", err,
				"spec_id", specID,
				"url", url)
		} else {
			logger.Infow("Saved spec to disk",
				"spec_id", specID,
				"url", url)
		}

		// Update configuration with the new spec if config supports it
		if config.ShouldUpdateConfig() {
			// Only add to config if not already there
			if !config.HasSpec(url) {
				logger.Debugw("Adding spec to configuration",
					"url", url,
					"spec_id", specID)

				config.AddSpec(url)

				// Save the updated configuration
				if err := config.Save(); err != nil {
					logger.Warnw("Failed to update config file with new spec",
						"error", err,
						"url", url,
						"spec_id", specID)
				} else {
					logger.Infow("Updated config file with new spec",
						"url", url,
						"spec_id", specID)
				}
			} else {
				logger.Debugw("Spec already exists in configuration",
					"url", url,
					"spec_id", specID)
			}
		} else {
			logger.Debugw("Configuration update not required",
				"url", url,
				"spec_id", specID)
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully loaded API spec: %s (version %s)", spec.Info.Title, spec.Info.Version)), nil
	})

	logger.Infow("Successfully registered load_api_spec tool")
}
