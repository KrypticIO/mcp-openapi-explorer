package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/krypticio/mcp-openapi-explorer/internal/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration for MCP OpenAPI Explorer",
	Long:  `Create, export, and manage configuration files for MCP OpenAPI Explorer.`,
}

// exportConfigCmd represents the config export command
var exportConfigCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export the current or default configuration to a file",
	Long: `Export the current or default configuration to a file.
If no configuration is loaded, a default configuration will be exported.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to ./config.yaml if no file specified
		outputPath := "config.yaml"
		if len(args) > 0 {
			outputPath = args[0]
		}

		// Ensure the directory exists
		dir := filepath.Dir(outputPath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// Export the configuration
		if err := config.WriteDefaultConfig(outputPath); err != nil {
			return fmt.Errorf("failed to export configuration: %w", err)
		}

		fmt.Printf("Configuration exported to %s\n", outputPath)
		return nil
	},
}

// printConfigCmd represents the config print command
var printConfigCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the current configuration",
	Long:  `Print the current configuration values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Current Configuration:")
		fmt.Println("======================")

		fmt.Printf("Logging:\n")
		fmt.Printf("  Level: %s\n", InternalConfig.Logging.Level)
		fmt.Printf("  Format: %s\n", InternalConfig.Logging.Format)

		fmt.Printf("\nServer:\n")
		fmt.Printf("  Port: %d\n", InternalConfig.Server.Port)
		fmt.Printf("  Specs Directory: %s\n", InternalConfig.Server.SpecsDir)

		fmt.Printf("\nGitHub:\n")
		if InternalConfig.GitHub.Token != "" {
			fmt.Printf("  Token: [REDACTED]\n")
		} else {
			fmt.Printf("  Token: <not set>\n")
		}

		fmt.Printf("\nSpecs to load:\n")
		if len(InternalConfig.Specs) == 0 {
			fmt.Printf("  <none>\n")
		} else {
			for i, spec := range InternalConfig.Specs {
				fmt.Printf("  %d: %s\n", i+1, spec)
			}
		}

		if InternalConfig.Path != "" {
			fmt.Printf("\nConfiguration loaded from: %s\n", InternalConfig.Path)
		}

		return nil
	},
}

// saveConfigCmd represents the config save command
var saveConfigCmd = &cobra.Command{
	Use:   "save [file]",
	Short: "Save the current configuration to a file",
	Long:  `Save the current configuration to a file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to ./config.yaml if no file specified
		outputPath := "config.yaml"
		if len(args) > 0 {
			outputPath = args[0]
		}

		// Update the path in the config
		InternalConfig.Path = outputPath

		// Save the configuration
		if err := InternalConfig.Save(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("Configuration saved to %s\n", outputPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(exportConfigCmd)
	configCmd.AddCommand(printConfigCmd)
	configCmd.AddCommand(saveConfigCmd)
}
