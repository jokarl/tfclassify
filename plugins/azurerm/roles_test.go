package main

import (
	"strings"
	"sync"
	"testing"
)

func TestNewRoleDatabaseFromJSON_ValidData(t *testing.T) {
	jsonData := `[
		{
			"id": "abc-123",
			"roleName": "Role A",
			"description": "Test role A",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["action1"], "notActions": [], "dataActions": [], "notDataActions": []}]
		},
		{
			"id": "def-456",
			"roleName": "Role B",
			"description": "Test role B",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["action2"], "notActions": [], "dataActions": [], "notDataActions": []}]
		},
		{
			"id": "ghi-789",
			"roleName": "Role C",
			"description": "Test role C",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["action3"], "notActions": [], "dataActions": [], "notDataActions": []}]
		}
	]`

	db, err := NewRoleDatabaseFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("NewRoleDatabaseFromJSON() error = %v, want nil", err)
	}

	if db.RoleCount() != 3 {
		t.Errorf("RoleCount() = %d, want 3", db.RoleCount())
	}
}

func TestNewRoleDatabaseFromJSON_EmptyArray(t *testing.T) {
	db, err := NewRoleDatabaseFromJSON([]byte(`[]`))
	if err != nil {
		t.Fatalf("NewRoleDatabaseFromJSON([]) error = %v, want nil", err)
	}

	if db.RoleCount() != 0 {
		t.Errorf("RoleCount() = %d, want 0", db.RoleCount())
	}
}

func TestNewRoleDatabaseFromJSON_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"invalid syntax", `{bad}`},
		{"not an array", `{"id": "123"}`},
		{"truncated", `[{"id": "123"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, err := NewRoleDatabaseFromJSON([]byte(tc.json))
			if err == nil {
				t.Error("NewRoleDatabaseFromJSON() error = nil, want non-nil")
			}
			if db != nil {
				t.Error("NewRoleDatabaseFromJSON() db = non-nil, want nil")
			}
		})
	}
}

func TestLookupByName_ExactMatch(t *testing.T) {
	jsonData := `[
		{
			"id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["*"], "notActions": [], "dataActions": [], "notDataActions": []}]
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))
	role, found := db.LookupByName("Owner")
	if !found {
		t.Fatal("LookupByName(\"Owner\") found = false, want true")
	}
	if role.Name != "Owner" {
		t.Errorf("role.Name = %q, want %q", role.Name, "Owner")
	}
}

func TestLookupByName_CaseInsensitive(t *testing.T) {
	jsonData := `[
		{
			"id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["*"], "notActions": [], "dataActions": [], "notDataActions": []}]
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))

	tests := []string{"owner", "OWNER", "Owner", "oWnEr"}
	for _, name := range tests {
		role, found := db.LookupByName(name)
		if !found {
			t.Errorf("LookupByName(%q) found = false, want true", name)
			continue
		}
		if role.Name != "Owner" {
			t.Errorf("LookupByName(%q) role.Name = %q, want %q", name, role.Name, "Owner")
		}
	}
}

func TestLookupByName_NotFound(t *testing.T) {
	jsonData := `[
		{
			"id": "abc-123",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": []
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))
	role, found := db.LookupByName("NonExistent")
	if found {
		t.Error("LookupByName(\"NonExistent\") found = true, want false")
	}
	if role != nil {
		t.Error("LookupByName(\"NonExistent\") role = non-nil, want nil")
	}
}

func TestLookupByID_BareGUID(t *testing.T) {
	jsonData := `[
		{
			"id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["*"], "notActions": [], "dataActions": [], "notDataActions": []}]
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))
	role, found := db.LookupByID("8e3af657-a8ff-443c-a75c-2fe8c4bcb635")
	if !found {
		t.Fatal("LookupByID() found = false, want true")
	}
	if role.Name != "Owner" {
		t.Errorf("role.Name = %q, want %q", role.Name, "Owner")
	}
}

func TestLookupByID_FullARMPath(t *testing.T) {
	jsonData := `[
		{
			"id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": [{"actions": ["*"], "notActions": [], "dataActions": [], "notDataActions": []}]
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))

	armPaths := []string{
		"/providers/Microsoft.Authorization/roleDefinitions/8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
		"/subscriptions/12345/providers/Microsoft.Authorization/roleDefinitions/8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
	}

	for _, path := range armPaths {
		role, found := db.LookupByID(path)
		if !found {
			t.Errorf("LookupByID(%q) found = false, want true", path)
			continue
		}
		if role.Name != "Owner" {
			t.Errorf("LookupByID(%q) role.Name = %q, want %q", path, role.Name, "Owner")
		}
	}
}

func TestLookupByID_CaseInsensitive(t *testing.T) {
	jsonData := `[
		{
			"id": "8E3AF657-A8FF-443C-A75C-2FE8C4BCB635",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": []
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))

	// Should find with lowercase
	role, found := db.LookupByID("8e3af657-a8ff-443c-a75c-2fe8c4bcb635")
	if !found {
		t.Fatal("LookupByID() found = false, want true")
	}
	if role.Name != "Owner" {
		t.Errorf("role.Name = %q, want %q", role.Name, "Owner")
	}
}

func TestLookupByID_NotFound(t *testing.T) {
	jsonData := `[
		{
			"id": "abc-123",
			"roleName": "Owner",
			"description": "Full access",
			"roleType": "BuiltInRole",
			"permissions": []
		}
	]`

	db, _ := NewRoleDatabaseFromJSON([]byte(jsonData))
	role, found := db.LookupByID("00000000-0000-0000-0000-000000000000")
	if found {
		t.Error("LookupByID() found = true, want false")
	}
	if role != nil {
		t.Error("LookupByID() role = non-nil, want nil")
	}
}

func TestDefaultRoleDatabase_ContainsOwner(t *testing.T) {
	db := DefaultRoleDatabase()

	role, found := db.LookupByName("Owner")
	if !found {
		t.Fatal("DefaultRoleDatabase does not contain Owner role")
	}

	// Verify Owner has * action
	if len(role.Permissions) == 0 {
		t.Fatal("Owner role has no permissions")
	}

	hasWildcard := false
	for _, perm := range role.Permissions {
		for _, action := range perm.Actions {
			if action == "*" {
				hasWildcard = true
				break
			}
		}
	}

	if !hasWildcard {
		t.Error("Owner role does not have '*' action")
	}
}

func TestDefaultRoleDatabase_ContainsContributor(t *testing.T) {
	db := DefaultRoleDatabase()

	role, found := db.LookupByName("Contributor")
	if !found {
		t.Fatal("DefaultRoleDatabase does not contain Contributor role")
	}

	// Verify Contributor has notActions including Microsoft.Authorization/*
	if len(role.Permissions) == 0 {
		t.Fatal("Contributor role has no permissions")
	}

	hasAuthNotAction := false
	for _, perm := range role.Permissions {
		for _, notAction := range perm.NotActions {
			if strings.Contains(strings.ToLower(notAction), "microsoft.authorization") {
				hasAuthNotAction = true
				break
			}
		}
	}

	if !hasAuthNotAction {
		t.Error("Contributor role does not have Microsoft.Authorization in notActions")
	}
}

func TestDefaultRoleDatabase_ContainsReader(t *testing.T) {
	db := DefaultRoleDatabase()

	role, found := db.LookupByName("Reader")
	if !found {
		t.Fatal("DefaultRoleDatabase does not contain Reader role")
	}

	// Verify Reader has */read action
	if len(role.Permissions) == 0 {
		t.Fatal("Reader role has no permissions")
	}

	hasReadAction := false
	for _, perm := range role.Permissions {
		for _, action := range perm.Actions {
			if action == "*/read" {
				hasReadAction = true
				break
			}
		}
	}

	if !hasReadAction {
		t.Error("Reader role does not have '*/read' action")
	}
}

func TestDefaultRoleDatabase_ContainsUserAccessAdministrator(t *testing.T) {
	db := DefaultRoleDatabase()

	role, found := db.LookupByName("User Access Administrator")
	if !found {
		t.Fatal("DefaultRoleDatabase does not contain User Access Administrator role")
	}

	// Verify UAA has Microsoft.Authorization/* action
	if len(role.Permissions) == 0 {
		t.Fatal("User Access Administrator role has no permissions")
	}

	hasAuthAction := false
	for _, perm := range role.Permissions {
		for _, action := range perm.Actions {
			if action == "Microsoft.Authorization/*" {
				hasAuthAction = true
				break
			}
		}
	}

	if !hasAuthAction {
		t.Error("User Access Administrator role does not have 'Microsoft.Authorization/*' action")
	}
}

func TestDefaultRoleDatabase_ThreadSafe(t *testing.T) {
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]*RoleDatabase, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = DefaultRoleDatabase()
		}(i)
	}

	wg.Wait()

	// All results should be the same instance
	first := results[0]
	for i, db := range results {
		if db != first {
			t.Errorf("goroutine %d got different instance", i)
		}
	}
}

func TestPermissionsPopulated(t *testing.T) {
	db := DefaultRoleDatabase()

	role, found := db.LookupByName("Owner")
	if !found {
		t.Fatal("Owner role not found")
	}

	if len(role.Permissions) == 0 {
		t.Fatal("Owner role has no permissions")
	}

	perm := role.Permissions[0]
	if len(perm.Actions) == 0 {
		t.Error("Owner role has no actions")
	}

	foundWildcard := false
	for _, action := range perm.Actions {
		if action == "*" {
			foundWildcard = true
			break
		}
	}

	if !foundWildcard {
		t.Error("Owner role Actions does not contain '*'")
	}
}

func TestLookupByName_NilDatabase(t *testing.T) {
	var db *RoleDatabase
	role, found := db.LookupByName("Owner")
	if found {
		t.Error("LookupByName on nil database found = true, want false")
	}
	if role != nil {
		t.Error("LookupByName on nil database role = non-nil, want nil")
	}
}

func TestLookupByID_NilDatabase(t *testing.T) {
	var db *RoleDatabase
	role, found := db.LookupByID("8e3af657-a8ff-443c-a75c-2fe8c4bcb635")
	if found {
		t.Error("LookupByID on nil database found = true, want false")
	}
	if role != nil {
		t.Error("LookupByID on nil database role = non-nil, want nil")
	}
}

func TestRoleCount_NilDatabase(t *testing.T) {
	var db *RoleDatabase
	if db.RoleCount() != 0 {
		t.Errorf("RoleCount on nil database = %d, want 0", db.RoleCount())
	}
}

func TestDefaultRoleDatabase_HasSufficientRoles(t *testing.T) {
	db := DefaultRoleDatabase()
	// We expect at least the well-known roles we embedded
	if db.RoleCount() < 10 {
		t.Errorf("RoleCount() = %d, want at least 10", db.RoleCount())
	}
}
