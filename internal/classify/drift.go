package classify

import (
	"fmt"

	"github.com/jokarl/tfclassify/internal/plan"
)

// DriftAnalyzer detects resources whose changes are drift corrections
// (changed outside Terraform) and classifies them at the configured drift level.
type DriftAnalyzer struct {
	// DriftClassification is the classification to assign to drift-corrected resources.
	// If empty, the analyzer produces no decisions.
	DriftClassification string
}

// Name returns the analyzer name.
func (a *DriftAnalyzer) Name() string {
	return "drift"
}

// AnalyzePlan checks which resources in the plan overlap with drift entries.
func (a *DriftAnalyzer) AnalyzePlan(result *plan.ParseResult) []ResourceDecision {
	if a.DriftClassification == "" {
		return nil
	}
	if len(result.DriftChanges) == 0 {
		return nil
	}

	// Build set of addresses that appear in resource_drift
	driftAddresses := make(map[string]struct{}, len(result.DriftChanges))
	for _, dc := range result.DriftChanges {
		driftAddresses[dc.Address] = struct{}{}
	}

	var decisions []ResourceDecision
	for _, change := range result.Changes {
		if _, isDrift := driftAddresses[change.Address]; isDrift {
			decisions = append(decisions, ResourceDecision{
				Address:        change.Address,
				ResourceType:   change.Type,
				Actions:        change.Actions,
				Classification: a.DriftClassification,
				MatchedRules: []string{
					fmt.Sprintf("builtin: drift - Resource %s is a drift correction (changed outside Terraform)", change.Address),
				},
			})
		}
	}

	return decisions
}

// DriftAddresses returns the set of resource addresses that appear in drift changes.
// This is used by BlastRadiusAnalyzer to exclude drift resources from counts.
func DriftAddresses(planResult *plan.ParseResult) map[string]struct{} {
	if planResult == nil || len(planResult.DriftChanges) == 0 {
		return nil
	}
	addrs := make(map[string]struct{}, len(planResult.DriftChanges))
	for _, dc := range planResult.DriftChanges {
		addrs[dc.Address] = struct{}{}
	}
	return addrs
}
