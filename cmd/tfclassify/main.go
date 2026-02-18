// Package main provides the tfclassify CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/output"
	"github.com/jokarl/tfclassify/internal/plan"
	"github.com/jokarl/tfclassify/internal/plugin"
)

// Version is set at build time.
var Version = "dev"

var (
	planPath       string
	configPath     string
	outputFmt      string
	verbose        bool
	detailedExitCode bool
)

// builtinAnalyzers returns the default set of builtin analyzers.
func builtinAnalyzers() []classify.BuiltinAnalyzer {
	return []classify.BuiltinAnalyzer{
		&classify.DeletionAnalyzer{},
		&classify.ReplaceAnalyzer{},
		&classify.SensitiveAnalyzer{},
	}
}

func main() {
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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Install plugins declared in configuration",
	Long: `Downloads and installs plugin binaries from their declared sources.

Reads plugin declarations from .tfclassify.hcl and downloads external plugins
from their GitHub release pages.

The GITHUB_TOKEN environment variable is supported for authenticated
requests (to avoid rate limits).`,
	RunE: runInit,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check configuration file for errors",
	Long: `Validates .tfclassify.hcl for correctness without requiring a Terraform plan.

Checks HCL syntax, classification references, precedence ordering, glob pattern
syntax, plugin references, and emits warnings for unreachable rules, empty
classifications, and missing plugin binaries.

Exit codes:
  0  Configuration is valid (warnings may be printed to stderr)
  1  Configuration has errors`,
	RunE: runValidate,
}

func init() {
	// Root command flags
	rootCmd.Flags().StringVarP(&planPath, "plan", "p", "", "Path to Terraform plan file (JSON or binary)")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: json, text, github")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().BoolVarP(&detailedExitCode, "detailed-exitcode", "d", false, "Use classification-based exit codes (0=auto, 1+=higher precedence)")

	rootCmd.MarkFlagRequired("plan")

	// Init command flags
	initCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	// Validate command flags
	validateCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Warn about redundant not_resource usage in verbose mode
	if verbose {
		config.WarnRedundantNotResource(cfg, os.Stderr)
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

	// Run builtin analyzers (deletion, replace, sensitive detection)
	classifier.RunBuiltinAnalyzers(result, planResult.Changes, builtinAnalyzers())

	// Run external plugins (if any configured)
	if hasExternalPlugins(cfg) {
		selfPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}

		host := plugin.NewHost(cfg)
		defer host.Shutdown()

		if err := host.DiscoverAndStart(selfPath); err != nil {
			if missingErr, ok := err.(*plugin.PluginNotInstalledError); ok {
				return fmt.Errorf("plugin %q is enabled but not installed.\nRun \"tfclassify init\" to install plugins declared in your configuration", missingErr.PluginName)
			}
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
	// When --detailed-exitcode is set, use classification-based exit codes.
	// Otherwise, exit 0 for any successful classification (CI-friendly default).
	if detailedExitCode {
		os.Exit(result.OverallExitCode)
	}
	os.Exit(0)
	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Load configuration (runs all error-level validations)
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Validate glob patterns
	if err := config.ValidateGlobPatterns(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Run warning-level checks
	warnings := config.ValidateWarnings(cfg)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}

	fmt.Println("Configuration valid.")
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Installing plugins...")
	return plugin.InstallPlugins(cfg, os.Stdout)
}

// hasExternalPlugins returns true if the config has any enabled external plugins (with a source).
func hasExternalPlugins(cfg *config.Config) bool {
	for _, p := range cfg.Plugins {
		if p.Enabled && p.Source != "" {
			return true
		}
	}
	return false
}
