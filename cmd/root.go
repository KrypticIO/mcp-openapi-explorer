package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Logger is the global logger instance
	Logger = logrus.New()

	// Flags
	verboseFlag bool
	portFlag    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcp-openapi-explorer",
	Short: "An MCP server for exploring and understanding OpenAPI specifications",
	Long: `MCP OpenAPI Explorer is a Model Context Protocol (MCP) server that analyzes 
OpenAPI specifications and provides context about interacting with APIs.

It can load OpenAPI specifications from various sources (GitHub, local files, HTTP URLs),
register multiple API specifications, and provide context about API interactions.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setup logger based on verbose flag
		if verboseFlag {
			Logger.SetLevel(logrus.DebugLevel)
		} else {
			Logger.SetLevel(logrus.InfoLevel)
		}

		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Define persistent flags for all commands
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&portFlag, "port", "p", "8080", "Port for the server to listen on")
}
