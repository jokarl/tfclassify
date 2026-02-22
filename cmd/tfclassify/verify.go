package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jokarl/tfclassify/internal/output"
)

var (
	verifyEvidenceFile string
	verifyPublicKey    string
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the signature of an evidence artifact",
	Long: `Verify the cryptographic signature of a tfclassify evidence artifact
using an Ed25519 public key. Exits 0 if valid, 1 if invalid.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVerify()
	},
}

func init() {
	verifyCmd.Flags().StringVarP(&verifyEvidenceFile, "evidence-file", "e", "", "Path to evidence artifact JSON (required)")
	verifyCmd.Flags().StringVarP(&verifyPublicKey, "public-key", "k", "", "Path to Ed25519 PEM public key (required)")
	_ = verifyCmd.MarkFlagRequired("evidence-file")
	_ = verifyCmd.MarkFlagRequired("public-key")
	rootCmd.AddCommand(verifyCmd)
}

func runVerify() error {
	artifactJSON, err := os.ReadFile(verifyEvidenceFile)
	if err != nil {
		return fmt.Errorf("reading evidence file: %w", err)
	}

	if err := output.VerifyEvidence(artifactJSON, verifyPublicKey); err != nil {
		fmt.Fprintln(os.Stderr, "Signature invalid.")
		os.Exit(1)
	}

	fmt.Println("Signature valid.")
	return nil
}
