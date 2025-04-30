package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var (
	// Flags
	specsDir string
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP OpenAPI explorer server",
	Long: `Start the MCP OpenAPI explorer server that provides:
- Context about API endpoints from OpenAPI specifications`,
	// Don't run the command when --help is provided
	RunE: func(cmd *cobra.Command, args []string) error {
		startServer()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&specsDir, "specs-dir", "d", "./specs", "Directory to store downloaded API specs")
}

func startServer() {
	// Create the directory for specs if it doesn't exist
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		Logger.Fatalf("Failed to create specs directory: %v", err)
	}

	// Create MCP handler with access to OpenAPI specs
	handler := NewMCPHandler()

	// If a spec path is provided via command line, load it immediately
	if specPath != "" {
		Logger.Infow("Loading initial OpenAPI spec from provided path", "path", specPath)

		// Handle GitHub repositories
		if strings.HasPrefix(specPath, "github.com") || strings.HasPrefix(specPath, "@github.com") {
			// Trim any '@' prefix that might be used
			path := strings.TrimPrefix(specPath, "@")

			// Convert github.com URL to raw.githubusercontent.com
			ghPath, err := convertGitHubURLToRaw(path, githubToken)
			if err != nil {
				Logger.Fatalw("Failed to process GitHub URL", "error", err, "path", path)
			}
			specPath = ghPath
			Logger.Debugw("Converted GitHub URL", "path", specPath)
		}

		// Load the OpenAPI spec
		ctx := context.Background()
		spec, err := handler.loadOpenAPISpec(ctx, specPath)
		if err != nil {
			Logger.Fatalw("Failed to load initial OpenAPI spec", "error", err, "path", specPath)
		}

		// Generate a unique ID for this spec and store it
		specID := spec.Info.Title
		handler.specs[specID] = &APISpec{
			URL:  specPath,
			Spec: spec,
		}

		// Save the spec to a file for persistence
		if err := handler.saveSpec(specID, handler.specs[specID]); err != nil {
			Logger.Warnw("Failed to save initial spec", "error", err, "specID", specID)
		}

		Logger.Infow("Successfully loaded initial API spec",
			"title", spec.Info.Title,
			"version", spec.Info.Version,
			"endpoints", len(spec.Paths.Map()))
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"OpenAPI Explorer",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Add tool to get API information
	// This is the main tool that provides context about API endpoints based on user queries
	openAPITool := mcp.NewTool(
		"get_api_info",
		mcp.WithDescription("Get comprehensive information about API endpoints from loaded OpenAPI specifications"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query about API endpoints (e.g. 'How do I create a new user?', 'What endpoints are available for pet management?')"),
		),
	)

	// Register the get_api_info tool
	mcpServer.AddTool(openAPITool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, ok := req.Params.Arguments["query"].(string)
		if !ok {
			return mcp.NewToolResultError("query parameter must be a string"), nil
		}

		// Use our handler to process the query and generate context about API endpoints
		mcpReq := &MCPRequest{Query: query}
		resp, err := handler.handleMCPRequest(ctx, mcpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error processing query: %v", err)), nil
		}

		// Return the generated context to the LLM
		// The LLM will use its own capabilities to find relevant information within this context
		return mcp.NewToolResultText(resp.Context), nil
	})

	// Add a tool to load API specs
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
		if strings.HasPrefix(url, "github.com") || strings.HasPrefix(url, "@github.com") {
			// Trim any '@' prefix that might be used
			path := strings.TrimPrefix(url, "@")

			// Convert github.com URL to raw.githubusercontent.com
			ghPath, err := convertGitHubURLToRaw(path, githubToken)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to process GitHub URL: %v", err)), nil
			}
			url = ghPath
			Logger.Debugw("Converted GitHub URL", "path", url)
		}

		// Load the OpenAPI spec
		spec, err := handler.loadOpenAPISpec(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load API spec: %v", err)), nil
		}

		// Generate a unique ID for this spec
		specID := spec.Info.Title
		handler.specs[specID] = &APISpec{
			URL:  url,
			Spec: spec,
		}

		// Save the spec to a file for persistence
		if err := handler.saveSpec(specID, handler.specs[specID]); err != nil {
			Logger.Warnw("Failed to save spec", "error", err, "specID", specID)
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully loaded API spec: %s (version %s)", spec.Info.Title, spec.Info.Version)), nil
	})

	// Add a tool to list loaded API specs
	listSpecsTool := mcp.NewTool(
		"list_api_specs",
		mcp.WithDescription("List all loaded OpenAPI specifications"),
	)

	// Register the list_api_specs tool
	mcpServer.AddTool(listSpecsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if len(handler.specs) == 0 {
			return mcp.NewToolResultText("No API specs have been loaded yet. Use the load_api_spec tool to load an OpenAPI spec."), nil
		}

		var result string
		result = "Loaded API Specifications:\n\n"
		for id, spec := range handler.specs {
			result += fmt.Sprintf("- %s\n", id)
			result += fmt.Sprintf("  Title: %s\n", spec.Spec.Info.Title)
			result += fmt.Sprintf("  Version: %s\n", spec.Spec.Info.Version)
			result += fmt.Sprintf("  URL: %s\n\n", spec.URL)
		}

		return mcp.NewToolResultText(result), nil
	})

	// Add a resource that provides system information
	systemResource := mcp.NewResource(
		"openapi://system",
		"System Information",
		mcp.WithResourceDescription("Information about the OpenAPI Explorer system"),
		mcp.WithMIMEType("text/plain"),
	)

	// Register the resource
	mcpServer.AddResource(systemResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		info := "OpenAPI Explorer MCP Server\n\n"
		info += fmt.Sprintf("Specs directory: %s\n", specsDir)
		info += fmt.Sprintf("Number of loaded specs: %d\n", len(handler.specs))

		if len(handler.specs) > 0 {
			info += "\nLoaded Specifications:\n"
			for id, spec := range handler.specs {
				info += fmt.Sprintf("- %s (Title: %s, Version: %s)\n", id, spec.Spec.Info.Title, spec.Spec.Info.Version)
			}
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "openapi://system",
				MIMEType: "text/plain",
				Text:     info,
			},
		}, nil
	})

	// Start the stdio server
	// This approach uses stdin/stdout for MCP communication which is simpler than HTTP/SSE
	// and works well with command-line tools and LLM integrations
	Logger.Infow("Starting OpenAPI Explorer MCP server via stdio...", "specsDir", specsDir)
	if err := server.ServeStdio(mcpServer); err != nil {
		Logger.Fatalw("Server error", "error", err)
	}
}
