package classify

import (
	"fmt"

	"github.com/jokarl/tfclassify/pkg/plan"
)

// ReplaceAnalyzer detects resource replacements (destroy + create).
type ReplaceAnalyzer struct{}

// Name returns the analyzer name.
func (a *ReplaceAnalyzer) Name() string {
	return "replace"
}

// Analyze inspects resources for replacements.
func (a *ReplaceAnalyzer) Analyze(changes []plan.ResourceChange) []ResourceDecision {
	var decisions []ResourceDecision

	for _, change := range changes {
		if isReplace(change.Actions) {
			decisions = append(decisions, ResourceDecision{
				Address:      change.Address,
				ResourceType: change.Type,
				Actions:      change.Actions,
				MatchedRule:  fmt.Sprintf("builtin: replace - Resource %s will be replaced (destroy and recreate)", change.Address),
			})
		}
	}

	return decisions
}

// isReplace returns true if the actions indicate a resource replacement.
func isReplace(actions []string) bool {
	hasDelete := false
	hasCreate := false

	for _, action := range actions {
		if action == "delete" {
			hasDelete = true
		}
		if action == "create" {
			hasCreate = true
		}
	}

	return hasDelete && hasCreate
}
