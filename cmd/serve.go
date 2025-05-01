package cmd

import (
	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/krypticio/mcp-openapi-explorer/internal/tools"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP OpenAPI explorer server",
	Long: `Start the MCP OpenAPI explorer server that provides:
- Context about API endpoints from OpenAPI specifications`,
	// Don't run the command when --help is provided
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func startServer() error {
	// Create a server instance
	s, err := server.CreateServer(specsDir, Logger, InternalConfig)
	if err != nil {
		return err
	}

	// Register all tools and resources
	tools.RegisterEverything(s.GetMCPServer(), s.GetHandler(), Logger, InternalConfig)

	// Start the server
	return s.Start()
}
