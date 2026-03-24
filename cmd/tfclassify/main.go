// Package main provides the tfclassify CLI entry point.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

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
	planPath         string
	configPath       string
	outputFmt        string
	verbose          bool
	detailedExitCode bool
	resourceFilters  []string
	evidenceFile     string
)

// builtinAnalyzers returns the default set of builtin analyzers.
// planResult is used to provide drift data for blast radius drift exclusion.
func builtinAnalyzers(cfg *config.Config, planResult *plan.ParseResult) []classify.BuiltinAnalyzer {
	br := classify.NewBlastRadiusAnalyzer(cfg.Classifications)
	br.SetDriftAddresses(classify.DriftAddresses(planResult))
	return []classify.BuiltinAnalyzer{
		&classify.DeletionAnalyzer{},
		&classify.ReplaceAnalyzer{},
		&classify.SensitiveAnalyzer{},
		br,
	}
}

// planAwareAnalyzers returns the set of plan-aware analyzers.
func planAwareAnalyzers(cfg *config.Config) []classify.PlanAwareAnalyzer {
	var analyzers []classify.PlanAwareAnalyzer
	if cfg.Defaults != nil && cfg.Defaults.DriftClassification != "" {
		analyzers = append(analyzers, &classify.DriftAnalyzer{
			DriftClassification: cfg.Defaults.DriftClassification,
		})
	}
	topoAnalyzer := classify.NewTopologyAnalyzer(cfg.Classifications)
	if len(topoAnalyzer.Thresholds()) > 0 {
		analyzers = append(analyzers, topoAnalyzer)
	}
	return analyzers
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

var explainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Trace classification decisions for plan resources",
	Long: `Shows why each resource was classified the way it was by tracing through the
full classification pipeline: core rules, builtin analyzers, and external plugins.

Produces a per-resource trace showing every rule evaluated, whether it matched
or was skipped, and how the final classification was determined via precedence.

Use --resource / -r (repeatable) to filter to specific resource addresses.`,
	RunE: runExplain,
}

func init() {
	// Root command flags
	rootCmd.Flags().StringVarP(&planPath, "plan", "p", "", "Path to Terraform plan file (JSON or binary)")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: json, text, github, sarif")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().BoolVarP(&detailedExitCode, "detailed-exitcode", "d", false, "Use classification-based exit codes (0=auto, 1+=higher precedence)")
	rootCmd.Flags().StringVar(&evidenceFile, "evidence-file", "", "Write evidence artifact to file")

	// MarkFlagRequired only errors if flag doesn't exist; safe to ignore for known flags
	_ = rootCmd.MarkFlagRequired("plan")

	// Init command flags
	initCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	// Validate command flags
	validateCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	// Explain command flags
	explainCmd.Flags().StringVarP(&planPath, "plan", "p", "", "Path to Terraform plan file (JSON or binary)")
	explainCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	explainCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: json, text")
	explainCmd.Flags().StringArrayVarP(&resourceFilters, "resource", "r", nil, "Resource address to explain (repeatable)")
	_ = explainCmd.MarkFlagRequired("plan")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(explainCmd)
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
	planResult, err := plan.ParseFile(cmd.Context(), planPath)
	if err != nil {
		return fmt.Errorf("failed to parse plan: %w", err)
	}

	// Preprocess: downgrade cosmetic-only updates (e.g., tag-only changes) to no-op
	if cfg.Defaults != nil && len(cfg.Defaults.IgnoreAttributes) > 0 {
		classify.FilterCosmeticChanges(planResult.Changes, cfg.Defaults.IgnoreAttributes)
	}

	// Create classifier
	classifier, err := classify.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create classifier: %w", err)
	}

	// Classify changes using core rules
	result := classifier.Classify(planResult.Changes)

	// Run builtin analyzers (deletion, replace, sensitive detection)
	classifier.RunBuiltinAnalyzers(result, planResult, builtinAnalyzers(cfg, planResult), planAwareAnalyzers(cfg))

	// Run external plugins (if any configured)
	if hasExternalPlugins(cfg) {
		selfPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}

		host := plugin.NewHost(cfg)
		defer host.Shutdown()

		if err := host.DiscoverAndStart(cmd.Context(), selfPath); err != nil {
			var missingErr *plugin.PluginNotInstalledError
			if errors.As(err, &missingErr) {
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
	formatter := output.NewFormatter(os.Stdout, format, verbose,
		output.WithVersion(Version),
		output.WithSARIFLevels(buildSARIFLevels(cfg)),
	)
	if err := formatter.Format(result); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Generate evidence artifact if configured or requested
	if err := generateEvidence(cfg, result, planResult, classifier); err != nil {
		return err
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

// generateEvidence creates and writes the evidence artifact if configured.
func generateEvidence(cfg *config.Config, result *classify.Result, planResult *plan.ParseResult, classifier *classify.Classifier) error {
	evidencePath := evidenceFile
	hasEvidenceConfig := cfg.Evidence != nil

	if evidencePath == "" && !hasEvidenceConfig {
		return nil
	}

	// Determine output path
	if evidencePath == "" {
		// Default filename with timestamp
		ts := time.Now().UTC().Format("20060102T150405Z")
		evidencePath = fmt.Sprintf("tfclassify-evidence-%s.json", ts)
		fmt.Fprintf(os.Stderr, "Warning: --evidence-file not provided, writing %s to current directory\n", evidencePath)
	}

	// Resolve config file path for hashing
	resolvedConfigPath, err := config.Discover(configPath)
	if err != nil {
		return fmt.Errorf("failed to discover config path for evidence: %w", err)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	opts := output.ResolveEvidenceOptions(cfg, Version, timestamp, planPath, resolvedConfigPath)

	// Collect explain trace if requested
	var explainResult *classify.ExplainResult
	if opts.IncludeTrace {
		explainResult = classifier.ExplainClassify(planResult.Changes)
		classifier.AddExplainBuiltinAnalyzers(explainResult, planResult, builtinAnalyzers(cfg, planResult), planAwareAnalyzers(cfg))
		classifier.FinalizeExplanation(explainResult)
	}

	artifact, err := output.BuildEvidence(result, explainResult, opts)
	if err != nil {
		return fmt.Errorf("failed to build evidence: %w", err)
	}

	// Sign if configured
	if opts.SigningKeyPath != "" {
		if err := output.SignEvidence(artifact, opts.SigningKeyPath); err != nil {
			return fmt.Errorf("failed to sign evidence: %w", err)
		}
	}

	if err := output.WriteEvidence(artifact, evidencePath); err != nil {
		return fmt.Errorf("failed to write evidence: %w", err)
	}

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

func runExplain(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Parse plan
	planResult, err := plan.ParseFile(cmd.Context(), planPath)
	if err != nil {
		return fmt.Errorf("failed to parse plan: %w", err)
	}

	// Preprocess: downgrade cosmetic-only updates (e.g., tag-only changes) to no-op
	if cfg.Defaults != nil && len(cfg.Defaults.IgnoreAttributes) > 0 {
		classify.FilterCosmeticChanges(planResult.Changes, cfg.Defaults.IgnoreAttributes)
	}

	// Create classifier
	classifier, err := classify.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create classifier: %w", err)
	}

	// Run explain classification (evaluates all rules, no short-circuit)
	result := classifier.ExplainClassify(planResult.Changes)

	// Run builtin analyzers with trace collection
	builtinDecisions := classifier.AddExplainBuiltinAnalyzers(result, planResult, builtinAnalyzers(cfg, planResult), planAwareAnalyzers(cfg))

	// Run external plugins (if any configured)
	if hasExternalPlugins(cfg) {
		selfPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}

		host := plugin.NewHost(cfg)
		defer host.Shutdown()

		if err := host.DiscoverAndStart(cmd.Context(), selfPath); err != nil {
			var missingErr *plugin.PluginNotInstalledError
			if errors.As(err, &missingErr) {
				fmt.Fprintf(os.Stderr, "Warning: %v\nPlugin decisions will not appear in trace.\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: plugin discovery failed: %v\n", err)
			}
		} else {
			pluginDecisions, err := host.RunAnalysis(planResult.Changes)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plugin analysis failed: %v\n", err)
			} else if len(pluginDecisions) > 0 {
				classifier.AddExplainPluginDecisions(result, pluginDecisions)
			}
		}
	}

	// Merge builtin decisions for precedence calculation
	_ = builtinDecisions

	// Finalize: determine winner for each resource across all trace entries
	classifier.FinalizeExplanation(result)

	// Apply resource filter
	if len(resourceFilters) > 0 {
		filterSet := make(map[string]bool, len(resourceFilters))
		for _, r := range resourceFilters {
			filterSet[r] = true
		}

		filtered := make([]classify.ResourceExplanation, 0)
		for _, res := range result.Resources {
			if filterSet[res.Address] {
				filtered = append(filtered, res)
			}
		}

		if len(filtered) == 0 {
			fmt.Fprintln(os.Stderr, "No matching resources found for the specified --resource filter(s).")
		}

		result.Resources = filtered
	}

	// Format and output
	format := output.Format(outputFmt)
	formatter := output.NewFormatter(os.Stdout, format, false)
	return formatter.FormatExplain(result)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Installing plugins...")
	return plugin.InstallPlugins(cmd.Context(), cfg, os.Stdout)
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

// buildSARIFLevels constructs a classification-name → SARIF level map from config.
// Uses explicit sarif_level when configured, otherwise defaults to "error" for the
// highest-precedence classification and "warning" for all others.
// The no_changes default classification gets "none".
func buildSARIFLevels(cfg *config.Config) map[string]string {
	levels := make(map[string]string, len(cfg.Classifications))

	// Set defaults based on precedence order
	for i, name := range cfg.Precedence {
		if i == 0 {
			levels[name] = "error"
		} else {
			levels[name] = "warning"
		}
	}

	// Override with no_changes default → "none"
	if cfg.Defaults != nil && cfg.Defaults.NoChanges != "" {
		levels[cfg.Defaults.NoChanges] = "none"
	}

	// Apply explicit sarif_level overrides
	for _, c := range cfg.Classifications {
		if c.SARIFLevel != "" {
			levels[c.Name] = c.SARIFLevel
		}
	}

	return levels
}
