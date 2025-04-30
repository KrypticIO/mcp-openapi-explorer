package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Create a bootstrap stderr-only logger for initialization errors
// that won't interfere with stdout/JSON-RPC communication
func initErrorf(format string, args ...interface{}) {
	// Write directly to stderr to avoid any stdout interference
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

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
		var cores []zapcore.Core
		var logLevel zapcore.Level

		// Set log level based on verbose flag or config
		if verboseFlag {
			logLevel = zap.DebugLevel
		} else if Config.Logging.Level != "" {
			switch strings.ToLower(Config.Logging.Level) {
			case "debug":
				logLevel = zap.DebugLevel
			case "info":
				logLevel = zap.InfoLevel
			case "warn":
				logLevel = zap.WarnLevel
			case "error":
				logLevel = zap.ErrorLevel
			default:
				logLevel = zap.InfoLevel
			}
		} else {
			logLevel = zap.InfoLevel
		}

		// Create encoder based on format
		var encoder zapcore.Encoder
		if strings.ToLower(Config.Logging.Format) == "text" {
			encoderConfig := zap.NewDevelopmentEncoderConfig()
			encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		} else {
			encoderConfig := zap.NewProductionEncoderConfig()
			encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}

		// Logger should output to stderr, not stdout to avoid interfering with JSON-RPC
		stderrSyncer := zapcore.AddSync(os.Stderr)
		cores = append(cores, zapcore.NewCore(encoder, stderrSyncer, logLevel))

		// Add file logging if debug=true and directory is specified
		if Config.Logging.Debug && Config.Logging.Directory != "" {
			// Ensure the log directory exists
			if err := os.MkdirAll(Config.Logging.Directory, 0755); err != nil {
				initErrorf("Failed to create log directory: %v", err)
			}

			// Set up file logging
			logPath := filepath.Join(Config.Logging.Directory, Config.Logging.Filename)
			fileSyncer, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				initErrorf("Failed to open log file: %v", err)
			}

			// Create a separate encoder for file logs (always JSON for better parsing)
			fileEncoderConfig := zap.NewProductionEncoderConfig()
			fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

			cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(fileSyncer), logLevel))
			// No longer directly writing to stdout
		}

		// Combine cores and create the logger
		core := zapcore.NewTee(cores...)
		zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))

		Logger = zapLogger.Sugar()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
			initErrorf("Error loading configuration: %v", err)
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
