// Package main provides the bundled Terraform plugin for tfclassify.
package main

import "github.com/jokarl/tfclassify/sdk"

// Compile-time interface assertions to verify our analyzers implement the sdk.Analyzer interface.
var (
	_ sdk.Analyzer = (*DeletionAnalyzer)(nil)
	_ sdk.Analyzer = (*SensitiveAnalyzer)(nil)
	_ sdk.Analyzer = (*ReplaceAnalyzer)(nil)
)
