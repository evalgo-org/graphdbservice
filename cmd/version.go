package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	goVersion "go.hein.dev/go-version"
)

var (
	// shortened controls whether to output just the version number or full build info
	shortened = false
	// version is the application version, set at build time via -ldflags
	version = "dev"
	// commit is the git commit hash, set at build time via -ldflags
	commit = "none"
	// date is the build date, set at build time via -ldflags
	date = "unknown"
	// output specifies the output format (json or yaml)
	output = "json"
	// versionCmd represents the version command
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Display version and build information",
		Long: `Display the version, git commit hash, and build date for this binary.

The version information can be displayed in two formats:
  - JSON (default): Structured output with all build details
  - YAML: Human-readable format with all build details

Examples:
  # Display version in JSON format
  graphservice version

  # Display version in YAML format
  graphservice version --output yaml

  # Display only the version number
  graphservice version --short

The version, commit, and date values are set at build time using Go's
-ldflags flag. For example:

  go build -ldflags "-X evalgo.org/graphservice/cmd.version=v1.0.0 \
    -X evalgo.org/graphservice/cmd.commit=$(git rev-parse HEAD) \
    -X evalgo.org/graphservice/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"`,
		Run: func(_ *cobra.Command, _ []string) {
			resp := goVersion.FuncWithOutput(shortened, version, commit, date, output)
			fmt.Print(resp)
		},
	}
)

// init registers the version command and its flags
func init() {
	versionCmd.Flags().BoolVarP(&shortened, "short", "s", true, "Print just the version number.")
	versionCmd.Flags().StringVarP(&output, "output", "o", "json", "Output format. One of 'yaml' or 'json'.")
	rootCmd.AddCommand(versionCmd)
}
