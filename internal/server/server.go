package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/krypticio/mcp-openapi-explorer/internal/github"
	"github.com/mark3labs/mcp-go/server"
)

// Common errors for server operations
var (
	ErrServerCreation = errors.New("failed to create server")
	ErrServerStart    = errors.New("failed to start server")
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
		logger.Errorw("Failed to create specs directory",
			"error", err,
			"directory", specsDir)
		return nil, fmt.Errorf("%w: could not create specs directory: %v", ErrServerCreation, err)
	}

	logger.Infow("Created specs directory", "directory", specsDir)

	// Create MCP handler with access to OpenAPI specs
	handler := NewHandler(specsDir, logger, config)

	// Load specs from configuration if available
	specs := config.GetSpecs()
	if len(specs) > 0 {
		logger.Infow("Loading specs from configuration",
			"count", len(specs),
			"specs_dir", specsDir)

		ctx := context.Background()
		loadedCount := 0
		failedCount := 0

		for i, specURL := range specs {
			logger.Infow("Loading spec from configuration",
				"url", specURL,
				"index", i+1,
				"total", len(specs))

			// Handle GitHub repositories
			if github.IsGitHubURL(specURL) {
				// Trim any '@' prefix that might be used
				path := github.TrimGitHubPrefix(specURL)

				// Use GitHub token from config
				ghToken := config.GetGitHubToken()

				// Convert github.com URL to raw.githubusercontent.com
				ghPath, err := github.ConvertGitHubURLToRaw(path, ghToken)
				if err != nil {
					logger.Warnw("Failed to process GitHub URL",
						"error", err,
						"path", path,
						"index", i+1)
					failedCount++
					continue
				}
				specURL = ghPath
				logger.Debugw("Converted GitHub URL",
					"original", path,
					"converted", specURL)
			}

			// Load the OpenAPI spec
			spec, err := handler.LoadOpenAPISpec(ctx, specURL)
			if err != nil {
				logger.Warnw("Failed to load spec from configuration",
					"error", err,
					"url", specURL,
					"index", i+1)
				failedCount++
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
				logger.Warnw("Failed to save spec from configuration",
					"error", err,
					"spec_id", specID,
					"url", specURL)
			}

			loadedCount++
			logger.Infow("Successfully loaded API spec from configuration",
				"title", spec.Info.Title,
				"version", spec.Info.Version,
				"endpoints", len(spec.Paths.Map()),
				"url", specURL,
				"spec_id", specID)
		}

		logger.Infow("Completed loading specs from configuration",
			"loaded", loadedCount,
			"failed", failedCount,
			"total", len(specs))
	} else {
		logger.Infow("No API specs defined in configuration")
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"OpenAPI Explorer",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
	)

	logger.Infow("Created MCP server",
		"name", "OpenAPI Explorer",
		"version", "1.0.0")

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
	s.logger.Infow("Starting OpenAPI Explorer MCP server via stdio...",
		"specs_dir", SpecsDir,
		"specs_loaded", len(s.handler.GetSpecs()))

	// Use a defer to catch any panics during server execution
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorw("Server panic",
				"error", r,
				"specs_loaded", len(s.handler.GetSpecs()))
			// If we get here, something really bad happened
			os.Exit(1)
		}
	}()

	if err := server.ServeStdio(s.mcpServer); err != nil {
		s.logger.Errorw("Server error", "error", err)
		return fmt.Errorf("%w: %v", ErrServerStart, err)
	}

	return nil
}

// StartServer starts the MCP OpenAPI explorer server (legacy function)
func StartServer(specsDir string, logger LoggerInterface, config ConfigInterface) error {
	logger.Infow("Creating server using legacy function", "specs_dir", specsDir)
	s, err := CreateServer(specsDir, logger, config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	return s.Start()
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
