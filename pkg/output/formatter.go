// Package output provides output formatters for classification results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jokarl/tfclassify/pkg/classify"
)

// Format represents the output format type.
type Format string

const (
	FormatJSON   Format = "json"
	FormatText   Format = "text"
	FormatGitHub Format = "github"
)

// Formatter outputs classification results.
type Formatter struct {
	writer  io.Writer
	format  Format
	verbose bool
}

// NewFormatter creates a new Formatter.
func NewFormatter(w io.Writer, format Format, verbose bool) *Formatter {
	return &Formatter{
		writer:  w,
		format:  format,
		verbose: verbose,
	}
}

// Format outputs the classification result in the configured format.
func (f *Formatter) Format(result *classify.Result) error {
	switch f.format {
	case FormatJSON:
		return f.formatJSON(result)
	case FormatGitHub:
		return f.formatGitHub(result)
	case FormatText:
		fallthrough
	default:
		return f.formatText(result)
	}
}

// JSONOutput represents the JSON output structure.
type JSONOutput struct {
	Overall            string         `json:"overall"`
	OverallDescription string         `json:"overall_description,omitempty"`
	ExitCode           int            `json:"exit_code"`
	NoChanges          bool           `json:"no_changes"`
	Resources          []JSONResource `json:"resources"`
}

// JSONResource represents a single resource in JSON output.
type JSONResource struct {
	Address                   string   `json:"address"`
	Type                      string   `json:"type"`
	Actions                   []string `json:"actions"`
	Classification            string   `json:"classification"`
	ClassificationDescription string   `json:"classification_description,omitempty"`
	MatchedRule               string   `json:"matched_rule"`
}

func (f *Formatter) formatJSON(result *classify.Result) error {
	output := JSONOutput{
		Overall:            result.Overall,
		OverallDescription: result.OverallDescription,
		ExitCode:           result.OverallExitCode,
		NoChanges:          result.NoChanges,
		Resources:          make([]JSONResource, 0, len(result.ResourceDecisions)),
	}

	for _, decision := range result.ResourceDecisions {
		output.Resources = append(output.Resources, JSONResource{
			Address:                   decision.Address,
			Type:                      decision.ResourceType,
			Actions:                   decision.Actions,
			Classification:            decision.Classification,
			ClassificationDescription: decision.ClassificationDescription,
			MatchedRule:               decision.MatchedRule,
		})
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (f *Formatter) formatText(result *classify.Result) error {
	var sb strings.Builder

	// Overall classification
	sb.WriteString(fmt.Sprintf("Classification: %s\n", result.Overall))
	if f.verbose && result.OverallDescription != "" {
		sb.WriteString(fmt.Sprintf("  %s\n", result.OverallDescription))
	}
	sb.WriteString(fmt.Sprintf("Exit code: %d\n", result.OverallExitCode))

	if result.NoChanges {
		sb.WriteString("No resource changes in plan.\n")
	} else {
		sb.WriteString(fmt.Sprintf("Resources: %d\n", len(result.ResourceDecisions)))
		sb.WriteString("\n")

		if f.verbose {
			// Group by classification
			byClassification := make(map[string][]classify.ResourceDecision)
			classificationOrder := make([]string, 0)
			for _, decision := range result.ResourceDecisions {
				if _, seen := byClassification[decision.Classification]; !seen {
					classificationOrder = append(classificationOrder, decision.Classification)
				}
				byClassification[decision.Classification] = append(
					byClassification[decision.Classification], decision)
			}

			for _, classification := range classificationOrder {
				decisions := byClassification[classification]
				sb.WriteString(fmt.Sprintf("[%s] (%d resources)\n", classification, len(decisions)))
				// Show classification description if available (from first decision)
				if len(decisions) > 0 && decisions[0].ClassificationDescription != "" {
					sb.WriteString(fmt.Sprintf("  %s\n", decisions[0].ClassificationDescription))
				}
				for _, decision := range decisions {
					sb.WriteString(fmt.Sprintf("  - %s (%s) %v\n",
						decision.Address, decision.ResourceType, decision.Actions))
					sb.WriteString(fmt.Sprintf("    Rule: %s\n", decision.MatchedRule))
				}
				sb.WriteString("\n")
			}
		} else {
			// Compact output
			for _, decision := range result.ResourceDecisions {
				sb.WriteString(fmt.Sprintf("  [%s] %s\n",
					decision.Classification, decision.Address))
			}
		}
	}

	_, err := f.writer.Write([]byte(sb.String()))
	return err
}

func (f *Formatter) formatGitHub(result *classify.Result) error {
	var sb strings.Builder

	// Set output variables using GITHUB_OUTPUT file format
	sb.WriteString(fmt.Sprintf("classification=%s\n", result.Overall))
	sb.WriteString(fmt.Sprintf("exit_code=%d\n", result.OverallExitCode))
	sb.WriteString(fmt.Sprintf("no_changes=%t\n", result.NoChanges))
	sb.WriteString(fmt.Sprintf("resource_count=%d\n", len(result.ResourceDecisions)))
	if result.OverallDescription != "" {
		sb.WriteString(fmt.Sprintf("classification_description=%s\n", result.OverallDescription))
	}

	_, err := f.writer.Write([]byte(sb.String()))
	return err
}
