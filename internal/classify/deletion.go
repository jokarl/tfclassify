package classify

import (
	"fmt"

	"github.com/jokarl/tfclassify/internal/plan"
)

// DeletionAnalyzer detects standalone resource deletions (delete without create).
type DeletionAnalyzer struct{}

// Name returns the analyzer name.
func (a *DeletionAnalyzer) Name() string {
	return "deletion"
}

// Analyze inspects resources for standalone deletions.
func (a *DeletionAnalyzer) Analyze(changes []plan.ResourceChange) []ResourceDecision {
	var decisions []ResourceDecision

	for _, change := range changes {
		if isStandaloneDelete(change.Actions) {
			decisions = append(decisions, ResourceDecision{
				Address:      change.Address,
				ResourceType: change.Type,
				Actions:      change.Actions,
				MatchedRule:  fmt.Sprintf("builtin: deletion - Resource %s is being deleted", change.Address),
			})
		}
	}

	return decisions
}

// isStandaloneDelete returns true if the actions indicate a standalone delete
// (not part of a replace operation).
func isStandaloneDelete(actions []string) bool {
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

	return hasDelete && !hasCreate
}
