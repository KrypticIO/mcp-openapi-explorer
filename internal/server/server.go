package server

import (
	"context"
	"os"

	"github.com/krypticio/mcp-openapi-explorer/internal/github"
	"github.com/mark3labs/mcp-go/server"
)

// Server represents the OpenAPI Explorer server
type Server struct {
	handler   *MCPHandler
	mcpServer *server.MCPServer
	logger    LoggerInterface
	config    ConfigInterface
}

// CreateServer creates a new Server instance
func CreateServer(specsDir string, logger LoggerInterface, config ConfigInterface) (*Server, error) {
	// Create the directory for specs if it doesn't exist
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		logger.Fatalw("Failed to create specs directory", "error", err)
		return nil, err
	}

	// Create MCP handler with access to OpenAPI specs
	handler := NewHandler(specsDir, logger, config)

	// Load specs from configuration if available
	if specs := config.GetSpecs(); len(specs) > 0 {
		logger.Infow("Loading specs from configuration", "count", len(specs))

		ctx := context.Background()
		for _, specURL := range specs {
			logger.Infow("Loading spec from configuration", "url", specURL)

			// Handle GitHub repositories
			if github.IsGitHubURL(specURL) {
				// Trim any '@' prefix that might be used
				path := github.TrimGitHubPrefix(specURL)

				// Use GitHub token from config
				ghToken := config.GetGitHubToken()

				// Convert github.com URL to raw.githubusercontent.com
				ghPath, err := github.ConvertGitHubURLToRaw(path, ghToken)
				if err != nil {
					logger.Warnw("Failed to process GitHub URL", "error", err, "path", path)
					continue
				}
				specURL = ghPath
				logger.Debugw("Converted GitHub URL", "path", specURL)
			}

			// Load the OpenAPI spec
			spec, err := handler.LoadOpenAPISpec(ctx, specURL)
			if err != nil {
				logger.Warnw("Failed to load spec from configuration", "error", err, "url", specURL)
				continue
			}

			// Generate a unique ID for this spec and store it
			specID := spec.Info.Title
			handler.AddSpec(specID, &APISpec{
				URL:  specURL,
				Spec: spec,
			})

			// Save the spec to a file for persistence
			if err := handler.SaveSpec(specID, handler.GetSpecs()[specID]); err != nil {
				logger.Warnw("Failed to save spec from configuration", "error", err, "specID", specID)
			}

			logger.Infow("Successfully loaded API spec from configuration",
				"title", spec.Info.Title,
				"version", spec.Info.Version,
				"endpoints", len(spec.Paths.Map()))
		}
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"OpenAPI Explorer",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	return &Server{
		handler:   handler,
		mcpServer: mcpServer,
		logger:    logger,
		config:    config,
	}, nil
}

// GetHandler returns the MCPHandler
func (s *Server) GetHandler() *MCPHandler {
	return s.handler
}

// GetMCPServer returns the MCPServer
func (s *Server) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}

// Start starts the server
func (s *Server) Start() error {
	// Start the stdio server
	s.logger.Infow("Starting OpenAPI Explorer MCP server via stdio...", "specsDir", SpecsDir)

	// Use a defer to catch any panics during server execution
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorw("Server panic", "error", r)
			// If we get here, something really bad happened
			os.Exit(1)
		}
	}()

	if err := server.ServeStdio(s.mcpServer); err != nil {
		s.logger.Fatalw("Server error", "error", err)
		return err
	}

	return nil
}

// StartServer starts the MCP OpenAPI explorer server (legacy function)
func StartServer(specsDir string, logger LoggerInterface, config ConfigInterface) error {
	server, err := CreateServer(specsDir, logger, config)
	if err != nil {
		return err
	}
	return server.Start()
}

// Helper function to check if a URL is a GitHub URL (deprecated, use github.IsGitHubURL instead)
func IsGitHubURL(url string) bool {
	return github.IsGitHubURL(url)
}

// Helper function to trim GitHub URL prefix (deprecated, use github.TrimGitHubPrefix instead)
func TrimGitHubPrefix(url string) string {
	return github.TrimGitHubPrefix(url)
}

// isGitHubURL is a wrapper for the github package
func isGitHubURL(url string) bool {
	return github.IsGitHubURL(url)
}

// trimGitHubPrefix is a wrapper for the github package
func trimGitHubPrefix(url string) string {
	return github.TrimGitHubPrefix(url)
}
