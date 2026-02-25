package classify

import (
	"fmt"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// BlastRadiusAnalyzer detects when a plan exceeds configured change thresholds.
// Unlike other builtin analyzers that inspect individual resources, this analyzer
// counts actions across the entire plan. When any threshold for a classification
// is exceeded, it emits a decision for every resource in the plan.
type BlastRadiusAnalyzer struct {
	// thresholds maps classification name to its blast radius config.
	thresholds map[string]*config.BlastRadiusConfig
	// driftAddresses is the set of resource addresses that are drift corrections.
	// When exclude_drift is enabled for a threshold, these addresses are excluded from counts.
	driftAddresses map[string]struct{}
}

// NewBlastRadiusAnalyzer creates a BlastRadiusAnalyzer from the classification configs.
func NewBlastRadiusAnalyzer(classifications []config.ClassificationConfig) *BlastRadiusAnalyzer {
	thresholds := make(map[string]*config.BlastRadiusConfig)
	for _, c := range classifications {
		if c.BlastRadius != nil {
			thresholds[c.Name] = c.BlastRadius
		}
	}
	return &BlastRadiusAnalyzer{thresholds: thresholds}
}

// SetDriftAddresses provides the set of drift addresses for exclude_drift support.
func (a *BlastRadiusAnalyzer) SetDriftAddresses(addrs map[string]struct{}) {
	a.driftAddresses = addrs
}

// Name returns the analyzer name.
func (a *BlastRadiusAnalyzer) Name() string {
	return "blast_radius"
}

// Analyze counts deletions, replacements, and total changes across the plan,
// then checks each classification's blast radius thresholds. When exceeded,
// emits a decision for every resource at that classification level.
func (a *BlastRadiusAnalyzer) Analyze(changes []plan.ResourceChange) []ResourceDecision {
	if len(a.thresholds) == 0 || len(changes) == 0 {
		return nil
	}

	// Check each classification's thresholds and collect decisions per classification
	type classDecision struct {
		classification string
		reasons        []string
	}
	var triggered []classDecision

	for classificationName, br := range a.thresholds {
		excludeDrift := br.ExcludeDrift != nil && *br.ExcludeDrift

		// Single pass to count actions (per threshold, since exclude_drift may differ)
		var deletions, replacements, totalChanges int
		for _, change := range changes {
			if excludeDrift && a.isDrift(change.Address) {
				continue
			}

			hasDelete := false
			hasCreate := false
			hasNonNoOp := false

			for _, action := range change.Actions {
				if action == "delete" {
					hasDelete = true
				}
				if action == "create" {
					hasCreate = true
				}
				if action != "no-op" {
					hasNonNoOp = true
				}
			}

			if hasDelete && !hasCreate {
				deletions++
			}
			if hasDelete && hasCreate {
				replacements++
			}
			if hasNonNoOp {
				totalChanges++
			}
		}

		var reasons []string
		if br.MaxDeletions != nil && deletions > *br.MaxDeletions {
			reasons = append(reasons, fmt.Sprintf(
				"builtin: blast_radius - %d deletions exceeded max_deletions threshold of %d",
				deletions, *br.MaxDeletions))
		}
		if br.MaxReplacements != nil && replacements > *br.MaxReplacements {
			reasons = append(reasons, fmt.Sprintf(
				"builtin: blast_radius - %d replacements exceeded max_replacements threshold of %d",
				replacements, *br.MaxReplacements))
		}
		if br.MaxChanges != nil && totalChanges > *br.MaxChanges {
			reasons = append(reasons, fmt.Sprintf(
				"builtin: blast_radius - %d total changes exceeded max_changes threshold of %d",
				totalChanges, *br.MaxChanges))
		}

		if len(reasons) > 0 {
			triggered = append(triggered, classDecision{
				classification: classificationName,
				reasons:        reasons,
			})
		}
	}

	if len(triggered) == 0 {
		return nil
	}

	// Emit a decision for every changed resource for each triggered classification.
	// No-op resources are excluded — they didn't change and shouldn't receive
	// a blast radius classification.
	var decisions []ResourceDecision
	for _, t := range triggered {
		for _, change := range changes {
			if isNoOp(change.Actions) {
				continue
			}
			decisions = append(decisions, ResourceDecision{
				Address:        change.Address,
				ResourceType:   change.Type,
				Actions:        change.Actions,
				Classification: t.classification,
				MatchedRules:   t.reasons,
			})
		}
	}

	return decisions
}

// isDrift returns true if the address is a drift-corrected resource.
func (a *BlastRadiusAnalyzer) isDrift(address string) bool {
	if a.driftAddresses == nil {
		return false
	}
	_, ok := a.driftAddresses[address]
	return ok
}

// isNoOp returns true if the resource's actions consist only of "no-op".
func isNoOp(actions []string) bool {
	for _, a := range actions {
		if a != "no-op" {
			return false
		}
	}
	return true
}
