// Package classify provides the core classification engine.
package classify

import (
	"strings"

	"github.com/jokarl/tfclassify/internal/plan"
)

// Result contains the classification results for a plan.
type Result struct {
	Overall            string             // highest-precedence classification
	OverallDescription string             // description of the overall classification
	OverallExitCode    int                // exit code corresponding to overall classification
	ResourceDecisions  []ResourceDecision // per-resource decisions
	NoChanges          bool               // true if plan has no resource changes
}

// ResourceDecision represents the classification decision for a single resource.
type ResourceDecision struct {
	Address                   string                 // resource address
	ResourceType              string                 // resource type
	Actions                   []string               // actions being performed
	Classification            string                 // the classification assigned
	ClassificationDescription string                 // description of the classification level
	MatchedRules              []string               // descriptions of which rules matched
	Metadata                  map[string]interface{} // plugin-provided metadata (optional)
	OriginalActions           []string               // set when actions were rewritten by ignore_attributes
	IgnoredAttributes         []string               // attribute paths covered by ignore_attributes
	IgnoreRuleMatches         []plan.IgnoreRuleMatch // scoped ignore_attribute rules (CR-0035) that contributed
}

// ExplainResult contains full trace data for explain output.
type ExplainResult struct {
	Resources []ResourceExplanation
	NoChanges bool
}

// ResourceExplanation holds the trace for a single resource.
type ResourceExplanation struct {
	Address             string
	ResourceType        string
	Actions             []string
	FinalClassification string
	FinalSource         string // "core-rule", "builtin: <name>", "plugin: <plugin>/<analyzer>"
	WinnerReason        string
	Trace               []TraceEntry
	OriginalActions     []string               // set when actions were rewritten by ignore_attributes
	IgnoredAttributes   []string               // attribute paths covered by ignore_attributes
	IgnoreRuleMatches   []plan.IgnoreRuleMatch // scoped ignore_attribute rules (CR-0035) that contributed
}

// TraceResult represents the evaluation outcome.
type TraceResult string

const (
	TraceMatch TraceResult = "match"
	TraceSkip  TraceResult = "skip"
)

// TraceEntry records one evaluation step in the classification pipeline.
type TraceEntry struct {
	Classification string            // which classification this entry belongs to
	Source         string            // "core-rule", "builtin: deletion", "plugin: azurerm/privilege-escalation"
	Rule           string            // rule description
	Result         TraceResult       // "match" or "skip"
	Reason         string            // skip reason or match context
	Metadata       map[string]string // plugin metadata (role_name, trigger, etc.)
}

// String returns a one-line representation of the trace entry.
func (t TraceEntry) String() string {
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(t.Classification)
	sb.WriteString("] ")
	sb.WriteString(t.Rule)

	if t.Result == TraceMatch {
		sb.WriteString(" → MATCH")
	} else {
		sb.WriteString(" → SKIP")
	}
	if t.Reason != "" {
		sb.WriteString(" (")
		sb.WriteString(t.Reason)
		sb.WriteString(")")
	}
	return sb.String()
}
