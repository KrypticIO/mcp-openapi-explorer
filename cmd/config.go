package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// AppConfig is the main application configuration structure
type AppConfig struct {
	Logging struct {
		Level     string `mapstructure:"level"`
		Format    string `mapstructure:"format"`
		Debug     bool   `mapstructure:"debug"`
		Directory string `mapstructure:"directory"`
		Filename  string `mapstructure:"filename"`
	} `mapstructure:"logging"`

	Server struct {
		Port     int    `mapstructure:"port"`
		SpecsDir string `mapstructure:"specs_dir"`
	} `mapstructure:"server"`

	GitHub struct {
		Token string `mapstructure:"token"`
	} `mapstructure:"github"`

	Specs []string `mapstructure:"specs"`
}

var (
	// Config is the global configuration instance
	Config AppConfig

	// ConfigFile is the path to the configuration file
	ConfigFile string
)

// initConfig reads in config file and ENV variables if set
func initConfig() error {
	if ConfigFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(ConfigFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to find home directory: %w", err)
		}

		// Add multiple search paths
		viper.AddConfigPath(".")                                          // current directory
		viper.AddConfigPath("./config")                                   // ./config directory
		viper.AddConfigPath(filepath.Join(home, ".mcp-openapi-explorer")) // home directory

		// Add standard system config directories
		viper.AddConfigPath("/etc/mcp-openapi-explorer") // Unix/Linux

		// Set name and type of the config file
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Read environment variables
	viper.SetEnvPrefix("MCP_OPENAPI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.debug", false)
	viper.SetDefault("logging.directory", "")
	viper.SetDefault("logging.filename", "mcp-openapi-explorer.log")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.specs_dir", "./specs")

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error occurred
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, but this is not an error
		// We'll just use defaults and environment variables
		if verboseFlag {
			Logger.Debugw("No config file found, using defaults and environment variables")
		}
	} else {
		// Config file found and successfully parsed
		if verboseFlag {
			Logger.Infow("Using config file", "path", viper.ConfigFileUsed())
		}
	}

	// Unmarshal config into struct
	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// WriteConfigFile writes the current configuration to a file
func WriteConfigFile(path string) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set the config to match current settings
	viper.Set("logging.level", Config.Logging.Level)
	viper.Set("logging.format", Config.Logging.Format)
	viper.Set("logging.debug", Config.Logging.Debug)
	viper.Set("logging.directory", Config.Logging.Directory)
	viper.Set("logging.filename", Config.Logging.Filename)
	viper.Set("server.port", Config.Server.Port)
	viper.Set("server.specs_dir", Config.Server.SpecsDir)
	viper.Set("github.token", Config.GitHub.Token)
	viper.Set("specs", Config.Specs)

	// Write config file
	return viper.WriteConfigAs(path)
}

// ExportDefaultConfig writes a default configuration file to the specified path
func ExportDefaultConfig(path string) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config to file
	defaultConfig := `# MCP OpenAPI Explorer Configuration

# Logging configuration
logging:
  # Log level (debug, info, warn, error)
  level: info
  # Log format (json or text)
  format: json
  # Enable debug logging to file
  debug: false
  # Directory to store log files (if empty, file logging is disabled)
  directory: ""
  # Filename for log files (default: mcp-openapi-explorer.log)
  filename: "mcp-openapi-explorer.log"

# Server configuration
server:
  # Server port
  port: 8080
  # Directory to store downloaded API specs
  specs_dir: ./specs

# GitHub configuration
github:
  # GitHub token for accessing private repositories
  # You can also use the MCP_OPENAPI_GITHUB_TOKEN environment variable
  token: ""

# List of OpenAPI specifications to load at startup
# Each entry can be a URL, GitHub repository, or file path
specs:
  # - https://petstore3.swagger.io/api/v3/openapi.json
  # - @github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml
  # - ./openapi.yaml
`

	// Write to file
	err := os.WriteFile(path, []byte(defaultConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}
