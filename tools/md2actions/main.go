// md2actions fetches Azure RBAC permission reference markdown files from
// Microsoft Docs on GitHub, parses the structured tables, and outputs a
// provider-keyed JSON file of all control-plane and data-plane actions.
//
// The output schema matches plugins/azurerm/actiondata/actions.json so it
// can be used as a drop-in replacement for the embedded action data.
//
// Primary source: Microsoft Docs GitHub raw markdown files from
// https://github.com/MicrosoftDocs/azure-docs/tree/main/articles/role-based-access-control/permissions
//
// Fallback source: If -from-roles flag is provided, extracts actions from the
// embedded role database (plugins/azurerm/roledata/roles.json) instead.
//
// Usage:
//
//	go run tools/md2actions/main.go > plugins/azurerm/actiondata/actions.json
//	go run tools/md2actions/main.go -from-roles plugins/azurerm/roledata/roles.json > plugins/azurerm/actiondata/actions.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// actionRegistry is the output schema: provider-keyed maps of sorted action names.
type actionRegistry struct {
	Actions     map[string][]string `json:"actions"`
	DataActions map[string][]string `json:"dataActions"`
}

// mdCategoryFiles lists the markdown files in the Microsoft Docs repo that
// contain Azure RBAC permission tables. These are organized by service category.
var mdCategoryFiles = []string{
	"ai-machine-learning",
	"analytics",
	"compute",
	"containers",
	"databases",
	"devops",
	"general",
	"hybrid-multicloud",
	"identity",
	"integration",
	"internet-of-things",
	"management-and-governance",
	"media",
	"migration",
	"mixed-reality",
	"monitor",
	"networking",
	"security",
	"storage",
	"web-and-mobile",
}

const rawBaseURL = "https://raw.githubusercontent.com/MicrosoftDocs/azure-docs/main/articles/role-based-access-control/permissions"

func main() {
	fromRoles := flag.String("from-roles", "", "Extract actions from role database JSON file instead of Microsoft Docs")
	merge := flag.String("merge", "", "Merge actions from role database JSON file into the Microsoft Docs results")
	flag.Parse()

	var registry *actionRegistry
	var err error

	if *fromRoles != "" {
		registry, err = extractFromRoles(*fromRoles)
	} else {
		registry, err = fetchFromMicrosoftDocs()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "md2actions: %v\n", err)
		os.Exit(1)
	}

	// Merge role database actions into the registry if requested
	if *merge != "" {
		roleRegistry, mergeErr := extractFromRoles(*merge)
		if mergeErr != nil {
			fmt.Fprintf(os.Stderr, "md2actions: merge: %v\n", mergeErr)
			os.Exit(1)
		}
		mergeRegistries(registry, roleRegistry)
		fmt.Fprintf(os.Stderr, "After merge: %d control-plane actions, %d data-plane actions across %d providers\n",
			countTotal(registry.Actions), countTotal(registry.DataActions),
			countProviders(registry))
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(registry); err != nil {
		fmt.Fprintf(os.Stderr, "md2actions: failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

// fetchFromMicrosoftDocs fetches markdown files from Microsoft Docs GitHub
// and parses action tables from them.
func fetchFromMicrosoftDocs() (*actionRegistry, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	registry := &actionRegistry{
		Actions:     make(map[string][]string),
		DataActions: make(map[string][]string),
	}

	actionSet := make(map[string]bool)
	dataActionSet := make(map[string]bool)

	for _, category := range mdCategoryFiles {
		url := fmt.Sprintf("%s/%s.md", rawBaseURL, category)
		fmt.Fprintf(os.Stderr, "Fetching %s...\n", url)

		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch %s: %v\n", category, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read %s: %v\n", category, err)
			continue
		}
		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "warning: %s returned status %d\n", category, resp.StatusCode)
			continue
		}

		parseMarkdownActions(string(body), actionSet, dataActionSet)
	}

	if len(actionSet) == 0 && len(dataActionSet) == 0 {
		return nil, fmt.Errorf("no actions found from Microsoft Docs")
	}

	buildRegistryMaps(registry, actionSet, dataActionSet)

	fmt.Fprintf(os.Stderr, "Extracted %d control-plane actions, %d data-plane actions across %d providers\n",
		countTotal(registry.Actions), countTotal(registry.DataActions),
		countProviders(registry))

	return registry, nil
}

// actionPattern matches Azure RBAC action strings like "Microsoft.Storage/storageAccounts/read"
var actionPattern = regexp.MustCompile(`(?i)(Microsoft\.[A-Za-z]+(?:/[A-Za-z0-9*]+)+)`)

// parseMarkdownActions extracts action names from markdown content.
// It recognizes two table formats:
//   - Regular actions: `| action_name | Description |`
//   - Data actions: `> | **DataAction** | **Description** |` or lines after a "DataActions" header
func parseMarkdownActions(content string, actionSet, dataActionSet map[string]bool) {
	lines := strings.Split(content, "\n")
	inDataActions := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect data action sections
		if strings.Contains(strings.ToLower(trimmed), "dataaction") {
			inDataActions = true
		}
		// Detect regular action sections (reset data action flag)
		if strings.Contains(trimmed, "| Action |") || strings.Contains(trimmed, "| **Action**") {
			inDataActions = false
		}
		if strings.Contains(trimmed, "| **DataAction**") || strings.Contains(trimmed, "| DataAction |") {
			inDataActions = true
		}

		// Skip non-table lines and separator lines
		if !strings.Contains(trimmed, "|") || strings.Contains(trimmed, "---") {
			continue
		}

		// Extract action names from table cells
		matches := actionPattern.FindAllString(trimmed, -1)
		for _, match := range matches {
			// Skip if it looks like a description/URL rather than an action
			if strings.Contains(match, "Microsoft.com") || strings.Contains(strings.ToLower(match), "microsoft.com") {
				continue
			}
			// Only include concrete actions (not wildcards) for the registry
			if strings.HasSuffix(match, "/*") || match == "*" {
				continue
			}

			if inDataActions {
				dataActionSet[match] = true
			} else {
				actionSet[match] = true
			}
		}
	}
}

// extractFromRoles extracts actions from the role database JSON file.
// This is a fallback when Microsoft Docs are unavailable.
func extractFromRoles(path string) (*actionRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read role database: %w", err)
	}

	type permission struct {
		Actions        []string `json:"actions"`
		NotActions     []string `json:"notActions"`
		DataActions    []string `json:"dataActions"`
		NotDataActions []string `json:"notDataActions"`
	}
	type role struct {
		Permissions []permission `json:"permissions"`
	}

	var roles []role
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, fmt.Errorf("failed to parse role database: %w", err)
	}

	actionSet := make(map[string]bool)
	dataActionSet := make(map[string]bool)

	for _, r := range roles {
		for _, p := range r.Permissions {
			for _, a := range p.Actions {
				// Skip wildcards - we only want concrete actions
				if a != "*" && !strings.HasSuffix(a, "/*") {
					actionSet[a] = true
				}
			}
			for _, a := range p.NotActions {
				if a != "*" && !strings.HasSuffix(a, "/*") {
					actionSet[a] = true
				}
			}
			for _, a := range p.DataActions {
				if a != "*" && !strings.HasSuffix(a, "/*") {
					dataActionSet[a] = true
				}
			}
			for _, a := range p.NotDataActions {
				if a != "*" && !strings.HasSuffix(a, "/*") {
					dataActionSet[a] = true
				}
			}
		}
	}

	registry := &actionRegistry{
		Actions:     make(map[string][]string),
		DataActions: make(map[string][]string),
	}

	buildRegistryMaps(registry, actionSet, dataActionSet)

	fmt.Fprintf(os.Stderr, "Extracted %d control-plane actions, %d data-plane actions across %d providers from role database\n",
		countTotal(registry.Actions), countTotal(registry.DataActions),
		countProviders(registry))

	return registry, nil
}

// buildRegistryMaps groups actions by lowercase provider namespace.
func buildRegistryMaps(registry *actionRegistry, actionSet, dataActionSet map[string]bool) {
	for action := range actionSet {
		provider := extractProvider(action)
		if provider == "" {
			continue
		}
		key := strings.ToLower(provider)
		registry.Actions[key] = append(registry.Actions[key], action)
	}

	for action := range dataActionSet {
		provider := extractProvider(action)
		if provider == "" {
			continue
		}
		key := strings.ToLower(provider)
		registry.DataActions[key] = append(registry.DataActions[key], action)
	}

	// Sort actions within each provider for deterministic output
	for key := range registry.Actions {
		sort.Strings(registry.Actions[key])
	}
	for key := range registry.DataActions {
		sort.Strings(registry.DataActions[key])
	}
}

// extractProvider returns the provider namespace from an action string.
// e.g., "Microsoft.Storage/storageAccounts/read" → "Microsoft.Storage"
// Returns "" for wildcards like "*/read" or "*".
func extractProvider(action string) string {
	parts := strings.SplitN(action, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	// Skip wildcards - they're patterns, not real providers
	if parts[0] == "*" {
		return ""
	}
	return parts[0]
}

// mergeRegistries adds all actions from src into dst, deduplicating within each provider.
func mergeRegistries(dst, src *actionRegistry) {
	mergeMap := func(dstMap, srcMap map[string][]string) {
		for provider, actions := range srcMap {
			existing := make(map[string]bool)
			for _, a := range dstMap[provider] {
				existing[a] = true
			}
			for _, a := range actions {
				if !existing[a] {
					dstMap[provider] = append(dstMap[provider], a)
					existing[a] = true
				}
			}
			sort.Strings(dstMap[provider])
		}
	}
	mergeMap(dst.Actions, src.Actions)
	mergeMap(dst.DataActions, src.DataActions)
}

func countTotal(m map[string][]string) int {
	total := 0
	for _, v := range m {
		total += len(v)
	}
	return total
}

func countProviders(registry *actionRegistry) int {
	providers := make(map[string]bool)
	for k := range registry.Actions {
		providers[k] = true
	}
	for k := range registry.DataActions {
		providers[k] = true
	}
	return len(providers)
}
