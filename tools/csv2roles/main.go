// csv2roles reads an AzAdvertizer CSV from stdin and writes a JSON array
// of Azure built-in role definitions to stdout.  The output schema matches
// plugins/azurerm/roledata/roles.json so it can be used as a drop-in
// replacement for the embedded role data.
//
// Usage:
//
//	curl -sL https://www.azadvertizer.net/azrolesadvertizer-comma.csv | go run tools/csv2roles/main.go > plugins/azurerm/roledata/roles.json
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// requiredColumns lists the CSV headers that must be present.
var requiredColumns = []string{
	"RoleId",
	"RoleName",
	"RoleDescription",
	"RoleActions",
	"RoleNotActions",
	"RoleDataActions",
	"RoleNotDataActions",
}

// roleDefinition mirrors the JSON schema used by plugins/azurerm/roledata/roles.json.
type roleDefinition struct {
	ID          string       `json:"id"`
	RoleName    string       `json:"roleName"`
	Description string       `json:"description"`
	RoleType    string       `json:"roleType"`
	Permissions []permission `json:"permissions"`
}

// permission represents a single permission block in a role definition.
type permission struct {
	Actions        []string `json:"actions"`
	NotActions     []string `json:"notActions"`
	DataActions    []string `json:"dataActions"`
	NotDataActions []string `json:"notDataActions"`
}

func main() {
	roles, err := parseCSV(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csv2roles: %v\n", err)
		os.Exit(1)
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].RoleName < roles[j].RoleName
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(roles); err != nil {
		fmt.Fprintf(os.Stderr, "csv2roles: failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

// parseCSV reads a CSV from r and returns a slice of roleDefinition values.
// It maps header names to column indices so that column order does not matter.
// Returns an error if required columns are missing or there are zero data rows.
func parseCSV(r io.Reader) ([]roleDefinition, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true

	// Read header row.
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column index map.
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		colIdx[strings.TrimSpace(name)] = i
	}

	// Verify all required columns are present.
	for _, col := range requiredColumns {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column %q in CSV header", col)
		}
	}

	idIdx := colIdx["RoleId"]
	nameIdx := colIdx["RoleName"]
	descIdx := colIdx["RoleDescription"]
	actionsIdx := colIdx["RoleActions"]
	notActionsIdx := colIdx["RoleNotActions"]
	dataActionsIdx := colIdx["RoleDataActions"]
	notDataActionsIdx := colIdx["RoleNotDataActions"]

	var roles []roleDefinition

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		role := roleDefinition{
			ID:          strings.TrimSpace(record[idIdx]),
			RoleName:    strings.TrimSpace(record[nameIdx]),
			Description: strings.TrimSpace(record[descIdx]),
			RoleType:    "BuiltInRole",
			Permissions: []permission{
				{
					Actions:        splitPermissions(record[actionsIdx]),
					NotActions:     splitPermissions(record[notActionsIdx]),
					DataActions:    splitPermissions(record[dataActionsIdx]),
					NotDataActions: splitPermissions(record[notDataActionsIdx]),
				},
			},
		}

		roles = append(roles, role)
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("CSV contains zero data rows")
	}

	return roles, nil
}

// splitPermissions splits a CSV permission field into a string slice.
// The AzAdvertizer CSV uses ", " (comma-space) as the delimiter between
// permissions within a field, and the literal string "empty" for fields
// with no permissions.
func splitPermissions(field string) []string {
	field = strings.TrimSpace(field)
	if field == "" || strings.EqualFold(field, "empty") {
		return []string{}
	}

	parts := strings.Split(field, ", ")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}
