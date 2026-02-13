// Package main provides the tfclassify CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jokarl/tfclassify/pkg/classify"
	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/output"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/pkg/plugin"
)

// Version is set at build time.
var Version = "dev"

var (
	planPath            string
	configPath          string
	outputFmt           string
	verbose             bool
	noPlugins           bool
	actAsBundledPlugin  bool
)

func main() {
	// Check for bundled plugin mode before starting Cobra
	for _, arg := range os.Args[1:] {
		if arg == "--act-as-bundled-plugin" {
			runBundledPlugin()
			return
		}
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "tfclassify",
	Short:   "Classify Terraform plan changes",
	Version: Version,
	Long: `tfclassify analyzes Terraform plan output and classifies resource changes
based on organization-defined rules. It helps automate change approval
workflows by categorizing changes as critical, standard, or auto-approved.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVarP(&planPath, "plan", "p", "", "Path to Terraform plan JSON file (required)")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: json, text, github")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().BoolVar(&noPlugins, "no-plugins", false, "Disable plugin loading")
	rootCmd.Flags().BoolVar(&actAsBundledPlugin, "act-as-bundled-plugin", false, "Run as bundled plugin (internal use)")
	rootCmd.Flags().MarkHidden("act-as-bundled-plugin")

	rootCmd.MarkFlagRequired("plan")
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Parse plan
	planResult, err := plan.ParseFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to parse plan: %w", err)
	}

	// Create classifier
	classifier, err := classify.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create classifier: %w", err)
	}

	// Classify changes using core rules
	result := classifier.Classify(planResult.Changes)

	// Run plugins unless disabled
	if !noPlugins {
		selfPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}

		host := plugin.NewHost(cfg)
		defer host.Shutdown()

		if err := host.DiscoverAndStart(selfPath); err != nil {
			// Log warning but continue with core-only classification
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: plugin discovery failed: %v\n", err)
			}
		} else {
			pluginDecisions, err := host.RunAnalysis(planResult.Changes)
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: plugin analysis failed: %v\n", err)
				}
			} else if len(pluginDecisions) > 0 {
				classifier.AddPluginDecisions(result, pluginDecisions)
			}
		}
	}

	// Format and output results
	format := output.Format(outputFmt)
	formatter := output.NewFormatter(os.Stdout, format, verbose)
	if err := formatter.Format(result); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Exit with appropriate code
	os.Exit(result.OverallExitCode)
	return nil
}

// runBundledPlugin runs this binary as the bundled terraform plugin.
// This is called when --act-as-bundled-plugin is passed.
func runBundledPlugin() {
	// Import the bundled plugin module and serve it
	// This will be implemented in CR-0007
	fmt.Fprintln(os.Stderr, "Error: bundled plugin not available (this is a tfclassify plugin, not a standalone executable)")
	os.Exit(1)
}
