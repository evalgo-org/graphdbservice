// Package cmd provides the command-line interface for the GraphDB Service application.
//
// This package implements a cobra-based CLI with commands for:
//   - graphdb: Start the GraphDB service API server
//   - version: Display version and build information
//
// The CLI supports configuration via:
//   - Command-line flags
//   - Configuration files (YAML format)
//   - Environment variables
//
// Configuration File Locations:
//   - Specified via --config flag
//   - $HOME/.cobra.yaml (default)
//
// Author:
//
//	Francisc Simon <francisc.simon@pantopix.com>
//
// License:
//
//	Apache License 2.0
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	eve "eve.evalgo.org/common"
)

var (
	// cfgFile holds the path to the configuration file
	cfgFile string
	// userLicense stores the license name for the project
	userLicense string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "graphservice",
		Short: "GraphDB Service - API for GraphDB repository and graph management",
		Long: `GraphDB Service provides a RESTful API for managing GraphDB repositories and RDF graphs.

The service enables:
  - Repository migration between GraphDB instances
  - Named graph operations (create, import, export, delete, rename)
  - Repository management (create, delete, rename)
  - Data import/export in multiple RDF formats
  - Secure connectivity via Ziti zero-trust networking

Use "graphservice graphdb" to start the API server.`,
	}
)

// Execute executes the root command and returns any error that occurs.
// This is the main entry point for the CLI application.
func Execute() error {
	return rootCmd.Execute()
}

// init initializes the command-line interface.
// It sets up logging, configuration initialization, and command flags.
func init() {
	// Configure eve logger to split output to multiple destinations
	eve.Logger.SetOutput(&eve.OutputSplitter{})
	// Register configuration initialization callback
	cobra.OnInitialize(initConfig)

	// Define persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")

	// Bind flags to viper configuration
	_ = viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))

	// Set default configuration values
	viper.SetDefault("author", "Francisc Simon <francisc.simon@pantopix.com>")
	viper.SetDefault("license", "apache")
}

// initConfig reads in config file and environment variables if set.
// This function is called during cobra initialization before command execution.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cobra")
	}

	// Read environment variables that match config keys
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
