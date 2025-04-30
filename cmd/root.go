package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger is the global logger instance
	Logger *zap.SugaredLogger

	// Flags
	verboseFlag bool
	configFlag  string // Config file flag

	// Global variables
	specsDir string // Directory to store downloaded API specs
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
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		if verboseFlag {
			config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		} else {
			config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		}

		zapLogger, err := config.Build()
		if err != nil {
			fmt.Printf("Failed to create logger: %v\n", err)
			os.Exit(1)
		}

		Logger = zapLogger.Sugar()
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
	// Set up the config file path
	cobra.OnInitialize(func() {
		// Store the config flag value in the global ConfigFile variable
		ConfigFile = configFlag

		// Initialize config using Viper
		if err := initConfig(); err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Set the global specsDir variable from config
		specsDir = Config.Server.SpecsDir
	})

	// Define persistent flags for all commands
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Path to configuration file")
}

// convertGitHubURLToRaw converts a GitHub URL to a raw.githubusercontent.com URL
// If token is provided, it uses it for private repositories
func convertGitHubURLToRaw(githubURL, token string) (string, error) {
	// Remove any protocol prefix if present
	githubURL = strings.TrimPrefix(githubURL, "https://")
	githubURL = strings.TrimPrefix(githubURL, "http://")

	// Ensure it's a GitHub URL
	if !strings.HasPrefix(githubURL, "github.com") {
		return "", fmt.Errorf("not a GitHub URL: %s", githubURL)
	}

	// Handle GitHub path
	parts := strings.Split(githubURL, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid GitHub URL format: %s", githubURL)
	}

	// Extract relevant parts
	owner := parts[1]
	repo := parts[2]

	// Determine if it's a blob URL or not
	var branch, path string
	if parts[3] == "blob" {
		branch = parts[4]
		path = strings.Join(parts[5:], "/")
	} else {
		branch = parts[3]
		path = strings.Join(parts[4:], "/")
	}

	// Construct raw URL
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)

	// Add token if provided
	if token != "" {
		rawURL = fmt.Sprintf("https://%s@raw.githubusercontent.com/%s/%s/%s/%s", token, owner, repo, branch, path)
	}

	return rawURL, nil
}
