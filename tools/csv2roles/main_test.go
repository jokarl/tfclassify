package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// makeCSV constructs a CSV string from a header row and data rows.
func makeCSV(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(strings.Join(headers, ","))
	b.WriteString("\n")
	for _, row := range rows {
		// Quote each field to mimic AzAdvertizer format.
		quoted := make([]string, len(row))
		for i, f := range row {
			quoted[i] = `"` + f + `"`
		}
		b.WriteString(strings.Join(quoted, ","))
		b.WriteString("\n")
	}
	return b.String()
}

var standardHeaders = []string{
	"RoleId", "RoleName", "RoleDescription",
	"RoleActions", "RoleNotActions", "RoleDataActions", "RoleNotDataActions",
}

func TestParseCSV_ValidData(t *testing.T) {
	csv := makeCSV(standardHeaders, [][]string{
		{"id-1", "Alpha Role", "Description A", "Microsoft.Compute/*/read", "empty", "empty", "empty"},
		{"id-2", "Beta Role", "Description B", "*/read", "Microsoft.Authorization/*/Delete", "empty", "empty"},
		{"id-3", "Gamma Role", "Description C", "*", "empty", "Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read", "empty"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	if len(roles) != 3 {
		t.Fatalf("parseCSV() returned %d roles, want 3", len(roles))
	}

	// Verify fields of the first role.
	r := roles[0]
	if r.ID != "id-1" {
		t.Errorf("roles[0].ID = %q, want %q", r.ID, "id-1")
	}
	if r.RoleName != "Alpha Role" {
		t.Errorf("roles[0].RoleName = %q, want %q", r.RoleName, "Alpha Role")
	}
	if r.Description != "Description A" {
		t.Errorf("roles[0].Description = %q, want %q", r.Description, "Description A")
	}
	if r.RoleType != "BuiltInRole" {
		t.Errorf("roles[0].RoleType = %q, want %q", r.RoleType, "BuiltInRole")
	}
	if len(r.Permissions) != 1 {
		t.Fatalf("roles[0].Permissions length = %d, want 1", len(r.Permissions))
	}
	if len(r.Permissions[0].Actions) != 1 || r.Permissions[0].Actions[0] != "Microsoft.Compute/*/read" {
		t.Errorf("roles[0].Permissions[0].Actions = %v, want [Microsoft.Compute/*/read]", r.Permissions[0].Actions)
	}
}

func TestParseCSV_EmptyPermissions(t *testing.T) {
	csv := makeCSV(standardHeaders, [][]string{
		{"id-1", "Empty Role", "No perms", "empty", "empty", "empty", "empty"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	if len(roles) != 1 {
		t.Fatalf("parseCSV() returned %d roles, want 1", len(roles))
	}

	perm := roles[0].Permissions[0]
	if len(perm.Actions) != 0 {
		t.Errorf("Actions = %v, want empty", perm.Actions)
	}
	if len(perm.NotActions) != 0 {
		t.Errorf("NotActions = %v, want empty", perm.NotActions)
	}
	if len(perm.DataActions) != 0 {
		t.Errorf("DataActions = %v, want empty", perm.DataActions)
	}
	if len(perm.NotDataActions) != 0 {
		t.Errorf("NotDataActions = %v, want empty", perm.NotDataActions)
	}
}

func TestParseCSV_MultiplePermissions(t *testing.T) {
	csv := makeCSV(standardHeaders, [][]string{
		{
			"id-1", "Multi Role", "Desc",
			"Microsoft.Compute/*/read, Microsoft.Network/*/read, Microsoft.Storage/*/read",
			"Microsoft.Authorization/*/Delete, Microsoft.Authorization/*/Write",
			"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read, Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write",
			"empty",
		},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	perm := roles[0].Permissions[0]

	if len(perm.Actions) != 3 {
		t.Errorf("Actions length = %d, want 3; got %v", len(perm.Actions), perm.Actions)
	}
	if len(perm.NotActions) != 2 {
		t.Errorf("NotActions length = %d, want 2; got %v", len(perm.NotActions), perm.NotActions)
	}
	if len(perm.DataActions) != 2 {
		t.Errorf("DataActions length = %d, want 2; got %v", len(perm.DataActions), perm.DataActions)
	}
	if len(perm.NotDataActions) != 0 {
		t.Errorf("NotDataActions length = %d, want 0; got %v", len(perm.NotDataActions), perm.NotDataActions)
	}

	// Verify specific values.
	expected := []string{
		"Microsoft.Compute/*/read",
		"Microsoft.Network/*/read",
		"Microsoft.Storage/*/read",
	}
	for i, want := range expected {
		if i >= len(perm.Actions) {
			break
		}
		if perm.Actions[i] != want {
			t.Errorf("Actions[%d] = %q, want %q", i, perm.Actions[i], want)
		}
	}
}

func TestParseCSV_EmptyInput(t *testing.T) {
	// Headers only, no data rows.
	csv := strings.Join(standardHeaders, ",") + "\n"

	_, err := parseCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("parseCSV() error = nil, want non-nil for empty input")
	}

	if !strings.Contains(err.Error(), "zero data rows") {
		t.Errorf("error = %q, want it to mention zero data rows", err.Error())
	}
}

func TestParseCSV_MissingColumns(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
	}{
		{
			name:    "missing RoleId",
			headers: []string{"RoleName", "RoleDescription", "RoleActions", "RoleNotActions", "RoleDataActions", "RoleNotDataActions"},
		},
		{
			name:    "missing RoleName",
			headers: []string{"RoleId", "RoleDescription", "RoleActions", "RoleNotActions", "RoleDataActions", "RoleNotDataActions"},
		},
		{
			name:    "missing RoleActions",
			headers: []string{"RoleId", "RoleName", "RoleDescription", "RoleNotActions", "RoleDataActions", "RoleNotDataActions"},
		},
		{
			name:    "missing RoleNotDataActions",
			headers: []string{"RoleId", "RoleName", "RoleDescription", "RoleActions", "RoleNotActions", "RoleDataActions"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			csv := strings.Join(tc.headers, ",") + "\n"
			_, err := parseCSV(strings.NewReader(csv))
			if err == nil {
				t.Fatal("parseCSV() error = nil, want non-nil for missing columns")
			}
			if !strings.Contains(err.Error(), "missing required column") {
				t.Errorf("error = %q, want it to mention missing required column", err.Error())
			}
		})
	}
}

func TestOutputSortedByRoleName(t *testing.T) {
	// Provide roles in reverse alphabetical order.
	csv := makeCSV(standardHeaders, [][]string{
		{"id-3", "Zebra Role", "Z desc", "*/read", "empty", "empty", "empty"},
		{"id-1", "Alpha Role", "A desc", "*/read", "empty", "empty", "empty"},
		{"id-2", "Middle Role", "M desc", "*/read", "empty", "empty", "empty"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	// Sort as main() does.
	sortRoles(roles)

	expected := []string{"Alpha Role", "Middle Role", "Zebra Role"}
	for i, want := range expected {
		if roles[i].RoleName != want {
			t.Errorf("roles[%d].RoleName = %q, want %q", i, roles[i].RoleName, want)
		}
	}
}

// sortRoles is a test helper that mirrors the sort logic in main().
func sortRoles(roles []roleDefinition) {
	for i := 0; i < len(roles); i++ {
		for j := i + 1; j < len(roles); j++ {
			if roles[j].RoleName < roles[i].RoleName {
				roles[i], roles[j] = roles[j], roles[i]
			}
		}
	}
}

func TestOutputSchemaMatchesEmbed(t *testing.T) {
	// This test verifies the JSON output can be deserialized into the same
	// structure used by plugins/azurerm/roles.go (RoleDefinition / Permission).
	csv := makeCSV(standardHeaders, [][]string{
		{"acdd72a7-3385-48ef-bd42-f606fba81ae7", "Reader", "View everything", "*/read", "empty", "empty", "empty"},
		{"8e3af657-a8ff-443c-a75c-2fe8c4bcb635", "Owner", "Full access", "*", "empty", "empty", "empty"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	// Marshal to JSON and back to verify round-trip.
	data, err := json.Marshal(roles)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Deserialize into the identical schema used by the embed consumer.
	type embedPermission struct {
		Actions        []string `json:"actions"`
		NotActions     []string `json:"notActions"`
		DataActions    []string `json:"dataActions"`
		NotDataActions []string `json:"notDataActions"`
	}
	type embedRole struct {
		ID          string            `json:"id"`
		RoleName    string            `json:"roleName"`
		Description string            `json:"description"`
		RoleType    string            `json:"roleType"`
		Permissions []embedPermission `json:"permissions"`
	}

	var parsed []embedRole
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output does not match embed schema", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("parsed %d roles, want 2", len(parsed))
	}

	// Verify key fields.
	for _, r := range parsed {
		if r.ID == "" {
			t.Error("role has empty id")
		}
		if r.RoleName == "" {
			t.Error("role has empty roleName")
		}
		if r.RoleType != "BuiltInRole" {
			t.Errorf("role %q has roleType = %q, want BuiltInRole", r.RoleName, r.RoleType)
		}
		if len(r.Permissions) != 1 {
			t.Errorf("role %q has %d permission blocks, want 1", r.RoleName, len(r.Permissions))
		}
	}
}

func TestIgnoreNonBuiltInColumns(t *testing.T) {
	// Add extra columns that should be ignored (UsedInPolicyCount, UsedInPolicy).
	headers := []string{
		"RoleId", "RoleName", "RoleDescription",
		"RoleActions", "RoleNotActions", "RoleDataActions", "RoleNotDataActions",
		"UsedInPolicyCount", "UsedInPolicy",
	}

	csv := makeCSV(headers, [][]string{
		{"id-1", "Test Role", "A test role", "*/read", "empty", "empty", "empty", "5", "some-policy"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	if len(roles) != 1 {
		t.Fatalf("parseCSV() returned %d roles, want 1", len(roles))
	}

	r := roles[0]
	if r.ID != "id-1" {
		t.Errorf("ID = %q, want %q", r.ID, "id-1")
	}
	if r.RoleName != "Test Role" {
		t.Errorf("RoleName = %q, want %q", r.RoleName, "Test Role")
	}

	// Verify output JSON has no extra fields.
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(data), "UsedInPolicy") {
		t.Error("output JSON contains UsedInPolicy field; extra columns should be ignored")
	}
	if strings.Contains(string(data), "some-policy") {
		t.Error("output JSON contains policy value; extra columns should be ignored")
	}
}

func TestSplitPermissions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty literal", "empty", []string{}},
		{"empty string", "", []string{}},
		{"single action", "Microsoft.Compute/*/read", []string{"Microsoft.Compute/*/read"}},
		{"two actions", "Microsoft.Compute/*/read, Microsoft.Network/*/read", []string{"Microsoft.Compute/*/read", "Microsoft.Network/*/read"}},
		{"three actions", "a/b/read, c/d/write, e/f/delete", []string{"a/b/read", "c/d/write", "e/f/delete"}},
		{"wildcard", "*", []string{"*"}},
		{"wildcard read", "*/read", []string{"*/read"}},
		{"whitespace around empty", "  empty  ", []string{}},
		{"EMPTY uppercase", "EMPTY", []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitPermissions(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitPermissions(%q) = %v (len %d), want %v (len %d)", tc.input, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitPermissions(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseCSV_CompletelyEmptyInput(t *testing.T) {
	_, err := parseCSV(strings.NewReader(""))
	if err == nil {
		t.Fatal("parseCSV() error = nil, want non-nil for completely empty input")
	}
}

func TestParseCSV_ReorderedColumns(t *testing.T) {
	// Reorder columns to verify header-based mapping works.
	headers := []string{
		"RoleNotDataActions", "RoleName", "RoleDataActions",
		"RoleId", "RoleNotActions", "RoleDescription", "RoleActions",
	}
	csv := makeCSV(headers, [][]string{
		{"empty", "Reordered Role", "empty", "id-reorder", "empty", "A reordered role", "*/read"},
	})

	roles, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV() error = %v, want nil", err)
	}

	if len(roles) != 1 {
		t.Fatalf("parseCSV() returned %d roles, want 1", len(roles))
	}

	r := roles[0]
	if r.ID != "id-reorder" {
		t.Errorf("ID = %q, want %q", r.ID, "id-reorder")
	}
	if r.RoleName != "Reordered Role" {
		t.Errorf("RoleName = %q, want %q", r.RoleName, "Reordered Role")
	}
	if r.Description != "A reordered role" {
		t.Errorf("Description = %q, want %q", r.Description, "A reordered role")
	}
	if len(r.Permissions[0].Actions) != 1 || r.Permissions[0].Actions[0] != "*/read" {
		t.Errorf("Actions = %v, want [*/read]", r.Permissions[0].Actions)
	}
}
