package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jokarl/tfclassify/internal/scaffold"
)

var (
	scaffoldOutput string
	scaffoldForce  bool
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Generate a starter .tfclassify.hcl from terraform state list output",
	Long: `Generate a starter .tfclassify.hcl configuration file by analyzing the
resource types in your Terraform state.

Pipe terraform state list output to this command:

  terraform state list | tfclassify scaffold
  cat resources.txt | tfclassify scaffold

The generated configuration groups your resources into risk-based
classification rules that you can refine for your approval workflow.

Use -o to write to a file instead of stdout:

  terraform state list | tfclassify scaffold -o .tfclassify.hcl`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScaffold()
	},
}

func init() {
	scaffoldCmd.Flags().StringVarP(&scaffoldOutput, "out", "o", "", "Write config to file (default: stdout)")
	scaffoldCmd.Flags().BoolVarP(&scaffoldForce, "force", "f", false, "Overwrite existing config file")
	rootCmd.AddCommand(scaffoldCmd)
}

func runScaffold() error {
	// Check that stdin is piped
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("checking stdin: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("no input detected\n\nUsage:\n  terraform state list | tfclassify scaffold\n  cat resources.txt | tfclassify scaffold")
	}

	// Parse state list
	parsed, err := scaffold.ParseStateList(os.Stdin)
	if err != nil {
		return err
	}

	// Categorize resource types
	result := scaffold.Categorize(parsed)

	// Generate config
	out, err := scaffold.Generate(result)
	if err != nil {
		return err
	}

	// Write output
	if scaffoldOutput == "" {
		// Write to stdout
		_, err := os.Stdout.Write(out)
		return err
	}

	// Check if file exists
	if !scaffoldForce {
		if _, err := os.Stat(scaffoldOutput); err == nil {
			return fmt.Errorf("file %s already exists (use --force to overwrite)", scaffoldOutput)
		}
	}

	if err := os.WriteFile(scaffoldOutput, out, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Wrote %s (%d resource types)\n", scaffoldOutput, len(parsed.ResourceTypes))
	return nil
}
