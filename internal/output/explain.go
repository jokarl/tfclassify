// Package output provides output formatters for classification results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/plan"
)

// FormatExplain outputs the explain result in the configured format.
func (f *Formatter) FormatExplain(result *classify.ExplainResult) error {
	switch f.format {
	case FormatJSON:
		return f.formatExplainJSON(result)
	case FormatText:
		fallthrough
	default:
		return f.formatExplainText(result)
	}
}

func (f *Formatter) formatExplainText(result *classify.ExplainResult) error {
	var sb strings.Builder

	if result.NoChanges {
		sb.WriteString("No resource changes in plan.\n")
		_, err := io.WriteString(f.writer, sb.String())
		return err
	}

	for i, res := range result.Resources {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}

		fmt.Fprintf(&sb, "Resource: %s\n", res.Address)
		fmt.Fprintf(&sb, "Actions:  %v\n", res.Actions)
		if len(res.OriginalActions) > 0 {
			fmt.Fprintf(&sb, "          Originally %v, downgraded by ignore_attributes: %s\n",
				res.OriginalActions, strings.Join(res.IgnoredAttributes, ", "))
			for _, m := range res.IgnoreRuleMatches {
				fmt.Fprintf(&sb, "          Matched rule %q: %s\n", m.Name, m.Description)
			}
		}
		fmt.Fprintf(&sb, "Final:    %s (from %s)\n", res.FinalClassification, res.FinalSource)
		sb.WriteString("\n  Evaluation trace:\n")

		for j, entry := range res.Trace {
			var resultStr string
			if entry.Result == classify.TraceMatch {
				resultStr = "MATCH"
			} else {
				resultStr = "SKIP"
			}

			if entry.Reason != "" {
				fmt.Fprintf(&sb, "  %d. [%s] %s → %s (%s)\n",
					j+1, entry.Classification, entry.Rule, resultStr, entry.Reason)
			} else {
				fmt.Fprintf(&sb, "  %d. [%s] %s → %s\n",
					j+1, entry.Classification, entry.Rule, resultStr)
			}

			// Print metadata if present
			for k, v := range entry.Metadata {
				fmt.Fprintf(&sb, "     %s: %s\n", k, v)
			}
		}

		fmt.Fprintf(&sb, "\n  Winner: %s (%s)\n", res.FinalClassification, res.WinnerReason)
	}

	_, err := io.WriteString(f.writer, sb.String())
	return err
}

// ExplainJSONOutput represents the JSON output for explain.
type ExplainJSONOutput struct {
	Resources []ExplainJSONResource `json:"resources"`
}

// ExplainJSONResource represents a single resource in explain JSON output.
type ExplainJSONResource struct {
	Address             string                 `json:"address"`
	Type                string                 `json:"type"`
	Actions             []string               `json:"actions"`
	FinalClassification string                 `json:"final_classification"`
	FinalSource         string                 `json:"final_source"`
	Trace               []ExplainJSONTrace     `json:"trace"`
	WinnerReason        string                 `json:"winner_reason"`
	OriginalActions     []string               `json:"original_actions,omitempty"`
	IgnoredAttributes   []string               `json:"ignored_attributes,omitempty"`
	IgnoreRuleMatches   []plan.IgnoreRuleMatch `json:"ignore_rule_matches,omitempty"`
}

// ExplainJSONTrace represents a single trace entry in explain JSON output.
type ExplainJSONTrace struct {
	Classification string            `json:"classification"`
	Source         string            `json:"source"`
	Rule           string            `json:"rule"`
	Result         string            `json:"result"`
	Reason         string            `json:"reason,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

func (f *Formatter) formatExplainJSON(result *classify.ExplainResult) error {
	output := ExplainJSONOutput{
		Resources: make([]ExplainJSONResource, 0, len(result.Resources)),
	}

	for _, res := range result.Resources {
		jsonRes := ExplainJSONResource{
			Address:             res.Address,
			Type:                res.ResourceType,
			Actions:             res.Actions,
			FinalClassification: res.FinalClassification,
			FinalSource:         res.FinalSource,
			WinnerReason:        res.WinnerReason,
			Trace:               make([]ExplainJSONTrace, 0, len(res.Trace)),
			OriginalActions:     res.OriginalActions,
			IgnoredAttributes:   res.IgnoredAttributes,
			IgnoreRuleMatches:   res.IgnoreRuleMatches,
		}

		for _, entry := range res.Trace {
			jsonTrace := ExplainJSONTrace{
				Classification: entry.Classification,
				Source:         entry.Source,
				Rule:           entry.Rule,
				Result:         string(entry.Result),
				Reason:         entry.Reason,
			}
			if len(entry.Metadata) > 0 {
				jsonTrace.Metadata = entry.Metadata
			}
			jsonRes.Trace = append(jsonRes.Trace, jsonTrace)
		}

		output.Resources = append(output.Resources, jsonRes)
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
