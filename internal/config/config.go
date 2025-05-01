package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Logging struct {
		Level     string `mapstructure:"level" yaml:"level"`
		Format    string `mapstructure:"format" yaml:"format"`
		Debug     bool   `mapstructure:"debug" yaml:"debug"`
		Directory string `mapstructure:"directory" yaml:"directory"`
		Filename  string `mapstructure:"filename" yaml:"filename"`
	} `mapstructure:"logging" yaml:"logging"`

	Server struct {
		SpecsDir string `mapstructure:"specs_dir" yaml:"specs_dir"`
	} `mapstructure:"server" yaml:"server"`

	GitHub struct {
		Token string `mapstructure:"token" yaml:"token"`
	} `mapstructure:"github" yaml:"github"`

	Specs []string `mapstructure:"specs" yaml:"specs"`
	Path  string   `yaml:"-"` // Not saved to YAML, just for reference
}

// LoggerInterface defines the required logging methods
type LoggerInterface interface {
	Debugw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
}

// LoadConfig loads configuration from a file
func LoadConfig(configFile string, logger LoggerInterface, verbose bool) (*Config, error) {
	v := viper.New()

	// Initialize a new Config instance
	cfg := &Config{
		Path: configFile,
	}

	// Set defaults
	setDefaults(v)

	if configFile != "" {
		// Use config file from the flag
		v.SetConfigFile(configFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to find home directory: %w", err)
		}

		// Add multiple search paths
		v.AddConfigPath(".")                                          // current directory
		v.AddConfigPath("./config")                                   // ./config directory
		v.AddConfigPath(filepath.Join(home, ".mcp-openapi-explorer")) // home directory

		// Add standard system config directories
		v.AddConfigPath("/etc/mcp-openapi-explorer") // Unix/Linux

		// Set name and type of the config file
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Read environment variables
	v.SetEnvPrefix("MCP_OPENAPI")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error occurred
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, but this is not an error
		// We'll just use defaults and environment variables
		if verbose {
			logger.Debugw("No config file found, using defaults and environment variables")
		}
	} else {
		// Config file found and successfully parsed
		if verbose {
			logger.Infow("Using config file", "path", v.ConfigFileUsed())
		}

		// If configFile wasn't explicitly provided, save the discovered path
		if configFile == "" {
			cfg.Path = v.ConfigFileUsed()
		}
	}

	// Unmarshal config into struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values in the viper instance
func setDefaults(v *viper.Viper) {
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.debug", false)
	v.SetDefault("logging.directory", "")
	v.SetDefault("logging.filename", "mcp-openapi-explorer.log")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.specs_dir", "./specs")
}

// GetGitHubToken returns the GitHub API token
func (c *Config) GetGitHubToken() string {
	return c.GitHub.Token
}

// GetSpecs returns the list of API specs to load
func (c *Config) GetSpecs() []string {
	return c.Specs
}

// ShouldUpdateConfig returns true if the config file should be updated
func (c *Config) ShouldUpdateConfig() bool {
	return c.Path != ""
}

// HasSpec returns true if the given URL is already in the config
func (c *Config) HasSpec(url string) bool {
	for _, spec := range c.Specs {
		if spec == url {
			return true
		}
	}
	return false
}

// AddSpec adds a spec to the config
func (c *Config) AddSpec(url string) {
	if !c.HasSpec(url) {
		c.Specs = append(c.Specs, url)
	}
}

// RemoveSpec removes a spec from the config
func (c *Config) RemoveSpec(url string) {
	newSpecs := make([]string, 0, len(c.Specs))
	for _, spec := range c.Specs {
		if spec != url {
			newSpecs = append(newSpecs, spec)
		}
	}
	c.Specs = newSpecs
}

// Save writes the config to disk
func (c *Config) Save() error {
	// If no config file is specified, do nothing
	if c.Path == "" {
		return nil
	}

	// Create the config directory if it doesn't exist
	configDir := filepath.Dir(c.Path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create a new Viper instance and set the values
	v := viper.New()
	v.Set("logging.level", c.Logging.Level)
	v.Set("logging.format", c.Logging.Format)
	v.Set("logging.debug", c.Logging.Debug)
	v.Set("logging.directory", c.Logging.Directory)
	v.Set("logging.filename", c.Logging.Filename)
	v.Set("server.specs_dir", c.Server.SpecsDir)
	v.Set("github.token", c.GitHub.Token)
	v.Set("specs", c.Specs)

	// Write the config to the specified path
	return v.WriteConfigAs(c.Path)
}

// WriteDefaultConfig writes a default configuration file to the specified path
func WriteDefaultConfig(path string) error {
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
	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}
