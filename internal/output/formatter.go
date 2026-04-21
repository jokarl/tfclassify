// Package output provides output formatters for classification results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jokarl/tfclassify/internal/classify"
)

// Format represents the output format type.
type Format string

const (
	FormatJSON   Format = "json"
	FormatText   Format = "text"
	FormatGitHub Format = "github"
	FormatSARIF  Format = "sarif"
)

// Formatter outputs classification results.
type Formatter struct {
	writer      io.Writer
	format      Format
	verbose     bool
	version     string
	sarifLevels map[string]string // classification name → SARIF level
}

// Option configures a Formatter.
type Option func(*Formatter)

// WithVersion sets the tool version for formats that report it (e.g., SARIF).
func WithVersion(version string) Option {
	return func(f *Formatter) { f.version = version }
}

// WithSARIFLevels sets the classification-to-SARIF-level mapping.
func WithSARIFLevels(levels map[string]string) Option {
	return func(f *Formatter) { f.sarifLevels = levels }
}

// NewFormatter creates a new Formatter.
func NewFormatter(w io.Writer, format Format, verbose bool, opts ...Option) *Formatter {
	f := &Formatter{
		writer:  w,
		format:  format,
		verbose: verbose,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format outputs the classification result in the configured format.
func (f *Formatter) Format(result *classify.Result) error {
	switch f.format {
	case FormatJSON:
		return f.formatJSON(result)
	case FormatGitHub:
		return f.formatGitHub(result)
	case FormatSARIF:
		return f.formatSARIF(result)
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
	MatchedRules              []string `json:"matched_rules"`
	OriginalActions           []string `json:"original_actions,omitempty"`
	IgnoredAttributes         []string `json:"ignored_attributes,omitempty"`
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
			MatchedRules:              decision.MatchedRules,
			OriginalActions:           decision.OriginalActions,
			IgnoredAttributes:         decision.IgnoredAttributes,
		})
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (f *Formatter) formatText(result *classify.Result) error {
	var sb strings.Builder

	// Overall classification
	fmt.Fprintf(&sb, "Classification: %s\n", result.Overall)
	if result.NoChanges {
		sb.WriteString("  No resource changes in plan.\n")
	} else if f.verbose && result.OverallDescription != "" {
		fmt.Fprintf(&sb, "  %s\n", result.OverallDescription)
	}
	fmt.Fprintf(&sb, "Exit code: %d\n", result.OverallExitCode)

	if result.NoChanges {
		// Show resources that were downgraded to no-op by ignore_attributes
		if f.verbose {
			var downgraded []classify.ResourceDecision
			for _, d := range result.ResourceDecisions {
				if len(d.OriginalActions) > 0 {
					downgraded = append(downgraded, d)
				}
			}
			if len(downgraded) > 0 {
				fmt.Fprintf(&sb, "Downgraded to no-op by ignore_attributes:\n")
				for _, d := range downgraded {
					fmt.Fprintf(&sb, "  - %s (%s)\n", d.Address, d.ResourceType)
					fmt.Fprintf(&sb, "    Originally: %v (downgraded by ignore_attributes: %s)\n",
						d.OriginalActions, strings.Join(d.IgnoredAttributes, ", "))
				}
			}
		}
	} else {
		activeCount := 0
		for _, d := range result.ResourceDecisions {
			if !isNoOpDecision(d) {
				activeCount++
			}
		}
		fmt.Fprintf(&sb, "Resources: %d\n", activeCount)
		sb.WriteByte('\n')

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

			noopByClassification := make(map[string]int)
			for _, classification := range classificationOrder {
				decisions := byClassification[classification]

				// Separate active changes from no-ops
				var active, noop []classify.ResourceDecision
				for _, d := range decisions {
					if isNoOpDecision(d) {
						noop = append(noop, d)
					} else {
						active = append(active, d)
					}
				}
				noopByClassification[classification] = len(noop)

				// Skip classifications that contain only no-op resources
				if len(active) == 0 {
					continue
				}

				fmt.Fprintf(&sb, "[%s] (%d resources)\n", classification, len(active))
				// Show classification description if available (from first decision)
				if active[0].ClassificationDescription != "" {
					fmt.Fprintf(&sb, "  %s\n", active[0].ClassificationDescription)
				}
				for _, decision := range active {
					fmt.Fprintf(&sb, "  - %s (%s) %v\n",
						decision.Address, decision.ResourceType, decision.Actions)
					if len(decision.OriginalActions) > 0 {
						fmt.Fprintf(&sb, "    Originally: %v (downgraded by ignore_attributes: %s)\n",
							decision.OriginalActions, strings.Join(decision.IgnoredAttributes, ", "))
					}
					for _, rule := range decision.MatchedRules {
						fmt.Fprintf(&sb, "    Rule: %s\n", rule)
					}
				}
				sb.WriteByte('\n')
			}
			writeNoOpSummary(&sb, "", classificationOrder, noopByClassification)
		} else {
			// Compact output — skip no-op resources
			noopByClassification := make(map[string]int)
			classificationOrder := make([]string, 0)
			for _, decision := range result.ResourceDecisions {
				if isNoOpDecision(decision) {
					if _, seen := noopByClassification[decision.Classification]; !seen {
						classificationOrder = append(classificationOrder, decision.Classification)
					}
					noopByClassification[decision.Classification]++
					continue
				}
				fmt.Fprintf(&sb, "  [%s] %s\n",
					decision.Classification, decision.Address)
			}
			writeNoOpSummary(&sb, "  ", classificationOrder, noopByClassification)
		}
	}

	_, err := io.WriteString(f.writer, sb.String())
	return err
}

// isNoOpDecision returns true if the decision's actions consist only of "no-op".
func isNoOpDecision(d classify.ResourceDecision) bool {
	for _, a := range d.Actions {
		if a != "no-op" {
			return false
		}
	}
	return true
}

// writeNoOpSummary prints the "N no-op resources hidden" line with a per-classification
// breakdown, so a reader can see what a config's ignore_attributes filtered out instead
// of having to trust an opaque flat count.
func writeNoOpSummary(sb *strings.Builder, indent string, order []string, counts map[string]int) {
	total := 0
	for _, c := range order {
		total += counts[c]
	}
	if total == 0 {
		return
	}

	var parts []string
	for _, c := range order {
		if n := counts[c]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", c, n))
		}
	}
	if len(parts) > 1 {
		fmt.Fprintf(sb, "%s(%d no-op resources hidden — %s)\n", indent, total, strings.Join(parts, ", "))
	} else {
		fmt.Fprintf(sb, "%s(%d no-op resources hidden)\n", indent, total)
	}
}

func (f *Formatter) formatGitHub(result *classify.Result) error {
	var sb strings.Builder

	// Set output variables using GITHUB_OUTPUT file format
	fmt.Fprintf(&sb, "classification=%s\n", result.Overall)
	fmt.Fprintf(&sb, "exit_code=%d\n", result.OverallExitCode)
	fmt.Fprintf(&sb, "no_changes=%t\n", result.NoChanges)
	fmt.Fprintf(&sb, "resource_count=%d\n", len(result.ResourceDecisions))
	if result.OverallDescription != "" {
		fmt.Fprintf(&sb, "classification_description=%s\n", result.OverallDescription)
	}

	_, err := io.WriteString(f.writer, sb.String())
	return err
}
