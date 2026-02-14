package classify

import (
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/pkg/plan"
)

// SensitiveAnalyzer detects changes to attributes marked as sensitive by Terraform.
type SensitiveAnalyzer struct{}

// Name returns the analyzer name.
func (a *SensitiveAnalyzer) Name() string {
	return "sensitive"
}

// Analyze inspects resources for sensitive attribute changes.
func (a *SensitiveAnalyzer) Analyze(changes []plan.ResourceChange) []ResourceDecision {
	var decisions []ResourceDecision

	for _, change := range changes {
		sensitiveAttrs := findSensitiveChanges(change)
		if len(sensitiveAttrs) > 0 {
			reason := fmt.Sprintf("builtin: sensitive - Resource %s has changes to sensitive attributes: %s",
				change.Address, strings.Join(sensitiveAttrs, ", "))

			decisions = append(decisions, ResourceDecision{
				Address:      change.Address,
				ResourceType: change.Type,
				Actions:      change.Actions,
				MatchedRule:  reason,
			})
		}
	}

	return decisions
}

// findSensitiveChanges walks the sensitive markers and returns attribute names
// that have sensitive values that changed.
func findSensitiveChanges(change plan.ResourceChange) []string {
	var sensitiveAttrs []string

	beforeSens := asBoolMap(change.BeforeSensitive)
	afterSens := asBoolMap(change.AfterSensitive)

	// Find all sensitive attributes from before
	for attr := range beforeSens {
		if beforeSens[attr] == true {
			if hasAttributeChanged(attr, change.Before, change.After) {
				sensitiveAttrs = append(sensitiveAttrs, attr)
			}
		}
	}

	// Find sensitive attributes that are newly sensitive in after
	for attr := range afterSens {
		if afterSens[attr] == true && beforeSens[attr] != true {
			if hasAttributeChanged(attr, change.Before, change.After) {
				found := false
				for _, existing := range sensitiveAttrs {
					if existing == attr {
						found = true
						break
					}
				}
				if !found {
					sensitiveAttrs = append(sensitiveAttrs, attr)
				}
			}
		}
	}

	return sensitiveAttrs
}

// asBoolMap converts an interface{} to map[string]interface{} for walking.
func asBoolMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// hasAttributeChanged checks if an attribute value changed between before and after.
func hasAttributeChanged(attr string, before, after map[string]interface{}) bool {
	if before == nil && after == nil {
		return false
	}

	beforeVal, beforeOk := before[attr]
	afterVal, afterOk := after[attr]

	// Attribute added
	if !beforeOk && afterOk {
		return true
	}

	// Attribute removed
	if beforeOk && !afterOk {
		return true
	}

	// Attribute changed (simple comparison, not deep)
	return fmt.Sprintf("%v", beforeVal) != fmt.Sprintf("%v", afterVal)
}
