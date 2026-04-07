// Package scaffold generates starter .tfclassify.hcl configurations
// from terraform state list output.
package scaffold

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ParseResult contains the extracted resource types and module paths
// from terraform state list output.
type ParseResult struct {
	// ResourceTypes is a deduplicated, sorted list of resource types
	// (e.g., "azurerm_resource_group", "azurerm_storage_account").
	ResourceTypes []string

	// Modules is a deduplicated, sorted list of module paths found
	// (e.g., "module.network", "module.production.module.app").
	Modules []string
}

// ParseStateList reads terraform state list output and extracts unique
// resource types and module paths. It skips data sources, strips instance
// indices, and deduplicates entries.
func ParseStateList(r io.Reader) (*ParseResult, error) {
	types := make(map[string]struct{})
	modules := make(map[string]struct{})

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip data sources
		if strings.HasPrefix(line, "data.") {
			continue
		}

		// Strip instance indices: resource[0], resource["key"]
		if idx := strings.IndexByte(line, '['); idx != -1 {
			line = line[:idx]
		}

		// Separate module prefix from resource address
		addr := line
		var modulePath string
		if i := lastModuleDot(addr); i != -1 {
			modulePath = addr[:i]
			addr = addr[i+1:]
		}

		if modulePath != "" {
			modules[modulePath] = struct{}{}
		}

		// Extract resource type: "azurerm_resource_group.example" -> "azurerm_resource_group"
		resType := extractResourceType(addr)
		if resType == "" {
			continue
		}
		types[resType] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading state list: %w", err)
	}

	if len(types) == 0 {
		return nil, fmt.Errorf("no resource types found in input")
	}

	return &ParseResult{
		ResourceTypes: sortedKeys(types),
		Modules:       sortedKeys(modules),
	}, nil
}

// lastModuleDot finds the position of the dot that separates the last
// "module.X" prefix from the resource address. Returns -1 if there is
// no module prefix.
//
// Examples:
//
//	"azurerm_resource_group.example"                       -> -1
//	"module.network.azurerm_virtual_network.main"          -> 14
//	"module.prod.module.network.azurerm_subnet.default"    -> 27
func lastModuleDot(addr string) int {
	// Walk forward through "module.X." segments
	pos := 0
	lastEnd := -1

	for strings.HasPrefix(addr[pos:], "module.") {
		// Skip "module."
		pos += len("module.")

		// Find the end of the module name (next dot)
		dot := strings.IndexByte(addr[pos:], '.')
		if dot == -1 {
			break
		}
		pos += dot
		lastEnd = pos
		pos++ // skip the dot
	}

	return lastEnd
}

// extractResourceType extracts the resource type from a bare resource address
// (no module prefix). Returns empty string if the address is invalid.
//
// "azurerm_resource_group.example" -> "azurerm_resource_group"
// "azurerm_resource_group"         -> ""  (no name part)
func extractResourceType(addr string) string {
	dot := strings.LastIndexByte(addr, '.')
	if dot <= 0 {
		return ""
	}
	return addr[:dot]
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
