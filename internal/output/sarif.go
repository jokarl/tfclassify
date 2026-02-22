package output

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/internal/classify"
)

// SARIF 2.1.0 types — minimal subset needed for tfclassify output.

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string                   `json:"name"`
	Version        string                   `json:"version,omitempty"`
	InformationURI string                   `json:"informationUri"`
	Rules          []sarifReportingDescriptor `json:"rules"`
}

type sarifReportingDescriptor struct {
	ID                   string                `json:"id"`
	ShortDescription     sarifMessage          `json:"shortDescription"`
	DefaultConfiguration sarifConfiguration    `json:"defaultConfiguration"`
}

type sarifConfiguration struct {
	Level string `json:"level"`
}

type sarifResult struct {
	RuleID              string                     `json:"ruleId"`
	RuleIndex           int                        `json:"ruleIndex"`
	Level               string                     `json:"level"`
	Message             sarifMessage               `json:"message"`
	Locations           []sarifLocation             `json:"locations,omitempty"`
	PartialFingerprints map[string]string           `json:"partialFingerprints"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	LogicalLocations []sarifLogicalLocation `json:"logicalLocations"`
}

type sarifLogicalLocation struct {
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	FullyQualifiedName string `json:"fullyQualifiedName"`
}

func (f *Formatter) formatSARIF(result *classify.Result) error {
	// Build rule index: classification name → index in rules array
	ruleIndex := make(map[string]int)
	var rules []sarifReportingDescriptor

	// Collect unique classifications from decisions (preserves encounter order)
	seen := make(map[string]bool)
	for _, d := range result.ResourceDecisions {
		if seen[d.Classification] {
			continue
		}
		seen[d.Classification] = true
		level := f.resolveLevel(d.Classification)
		ruleIndex[d.Classification] = len(rules)
		desc := d.ClassificationDescription
		if desc == "" {
			desc = d.Classification
		}
		rules = append(rules, sarifReportingDescriptor{
			ID:               d.Classification,
			ShortDescription: sarifMessage{Text: desc},
			DefaultConfiguration: sarifConfiguration{Level: level},
		})
	}

	// Build results
	results := make([]sarifResult, 0, len(result.ResourceDecisions))
	for _, d := range result.ResourceDecisions {
		level := f.resolveLevel(d.Classification)
		msgText := strings.Join(d.MatchedRules, "; ")
		if msgText == "" {
			msgText = fmt.Sprintf("Resource classified as %s", d.Classification)
		}

		results = append(results, sarifResult{
			RuleID:    d.Classification,
			RuleIndex: ruleIndex[d.Classification],
			Level:     level,
			Message:   sarifMessage{Text: msgText},
			Locations: []sarifLocation{{
				LogicalLocations: []sarifLogicalLocation{{
					Name:               d.Address,
					Kind:               "resource",
					FullyQualifiedName: d.Address,
				}},
			}},
			PartialFingerprints: map[string]string{
				"primaryLocationFingerprint": fingerprint(d.Address, d.Classification),
			},
		})
	}

	doc := sarifDocument{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "tfclassify",
					Version:        f.version,
					InformationURI: "https://github.com/jokarl/tfclassify",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// resolveLevel returns the SARIF level for a classification name.
// It checks the user-configured sarifLevels map first, falling back to "warning".
func (f *Formatter) resolveLevel(classification string) string {
	if f.sarifLevels != nil {
		if level, ok := f.sarifLevels[classification]; ok {
			return level
		}
	}
	return "warning"
}

// fingerprint computes a stable SHA-256 fingerprint for deduplication.
func fingerprint(address, classification string) string {
	h := sha256.Sum256([]byte(address + "/" + classification))
	return fmt.Sprintf("%x", h)
}
