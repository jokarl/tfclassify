package main

import (
	"testing"
)

func TestActionMatchesPattern_Wildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "*", true},
		{"anything", "*", true},
		{"", "*", true},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_NamespaceWildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "Microsoft.Compute/*", true},
		{"Microsoft.Compute/disks/write", "Microsoft.Compute/*", true},
		{"Microsoft.Storage/storageAccounts/read", "Microsoft.Compute/*", false},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_NamespaceNoMatch(t *testing.T) {
	got := actionMatchesPattern("Microsoft.Storage/storageAccounts/read", "Microsoft.Compute/*")
	if got {
		t.Error("expected no match for different namespace")
	}
}

func TestActionMatchesPattern_SuffixWildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "*/read", true},
		{"Microsoft.Storage/storageAccounts/read", "*/read", true},
		{"Microsoft.Compute/virtualMachines/write", "*/read", false},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_ExactMatch(t *testing.T) {
	action := "Microsoft.Compute/virtualMachines/read"
	got := actionMatchesPattern(action, action)
	if !got {
		t.Errorf("expected exact match for %q", action)
	}
}

func TestActionMatchesPattern_CaseInsensitive(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
	}{
		{"MICROSOFT.COMPUTE/virtualMachines/read", "microsoft.compute/*"},
		{"microsoft.compute/virtualmachines/READ", "Microsoft.Compute/*"},
		{"MICROSOFT.AUTHORIZATION/ROLEASSIGNMENTS/WRITE", "microsoft.authorization/*"},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if !got {
			t.Errorf("actionMatchesPattern(%q, %q) = false, want true (case insensitive)", tc.action, tc.pattern)
		}
	}
}

func TestComputeEffective_NoExclusions(t *testing.T) {
	actions := []string{"*"}
	result := computeEffectiveActions(actions, nil)

	if len(result) != 1 || result[0] != "*" {
		t.Errorf("computeEffectiveActions with no exclusions = %v, want [*]", result)
	}
}

func TestComputeEffective_SubtractsMatching(t *testing.T) {
	actions := []string{
		"Microsoft.Authorization/roleAssignments/write",
		"Microsoft.Compute/virtualMachines/read",
	}
	notActions := []string{"Microsoft.Authorization/*"}

	result := computeEffectiveActions(actions, notActions)

	if len(result) != 1 {
		t.Fatalf("expected 1 effective action, got %d: %v", len(result), result)
	}
	if result[0] != "Microsoft.Compute/virtualMachines/read" {
		t.Errorf("expected Compute action, got %q", result[0])
	}
}

func TestScorePermissions_Owner(t *testing.T) {
	role := &RoleDefinition{
		Name: "Owner",
		Permissions: []Permission{{
			Actions:    []string{"*"},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 95 {
		t.Errorf("Owner score = %d, want 95", score.Total)
	}
	if !score.HasWildcard {
		t.Error("Owner should have HasWildcard = true")
	}
	if !score.HasAuthWrite {
		t.Error("Owner should have HasAuthWrite = true")
	}
}

func TestScorePermissions_Contributor(t *testing.T) {
	role := &RoleDefinition{
		Name: "Contributor",
		Permissions: []Permission{{
			Actions: []string{"*"},
			NotActions: []string{
				"Microsoft.Authorization/*/Delete",
				"Microsoft.Authorization/*/Write",
				"Microsoft.Authorization/elevateAccess/Action",
			},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 70 {
		t.Errorf("Contributor score = %d, want 70", score.Total)
	}
	if score.HasWildcard {
		t.Error("Contributor should have HasWildcard = false (auth excluded)")
	}
	if score.HasAuthWrite {
		t.Error("Contributor should have HasAuthWrite = false")
	}
}

func TestScorePermissions_Reader(t *testing.T) {
	role := &RoleDefinition{
		Name: "Reader",
		Permissions: []Permission{{
			Actions:    []string{"*/read"},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 15 {
		t.Errorf("Reader score = %d, want 15", score.Total)
	}
	if score.HasWildcard {
		t.Error("Reader should have HasWildcard = false")
	}
	if score.HasAuthWrite {
		t.Error("Reader should have HasAuthWrite = false")
	}
}

func TestScorePermissions_UAA(t *testing.T) {
	role := &RoleDefinition{
		Name: "User Access Administrator",
		Permissions: []Permission{{
			Actions: []string{
				"*/read",
				"Microsoft.Authorization/*",
				"Microsoft.Support/*",
			},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 85 {
		t.Errorf("UAA score = %d, want 85", score.Total)
	}
	if score.HasWildcard {
		t.Error("UAA should have HasWildcard = false")
	}
	if !score.HasAuthWrite {
		t.Error("UAA should have HasAuthWrite = true")
	}
}

func TestScorePermissions_TargetedAuthWrite(t *testing.T) {
	role := &RoleDefinition{
		Name: "Custom Deployer",
		Permissions: []Permission{{
			Actions: []string{
				"Microsoft.Authorization/roleAssignments/write",
			},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 75 {
		t.Errorf("Targeted auth write score = %d, want 75", score.Total)
	}
	if !score.HasAuthWrite {
		t.Error("should have HasAuthWrite = true")
	}
}

func TestScorePermissions_EmptyPermissions(t *testing.T) {
	role := &RoleDefinition{
		Name:        "Empty",
		Permissions: []Permission{},
	}

	score := ScorePermissions(role)

	if score.Total != 0 {
		t.Errorf("Empty role score = %d, want 0", score.Total)
	}
}

func TestScorePermissions_NilRole(t *testing.T) {
	score := ScorePermissions(nil)

	if score.Total != 0 {
		t.Errorf("Nil role score = %d, want 0", score.Total)
	}
}

func TestScorePermissions_EmptyActions(t *testing.T) {
	role := &RoleDefinition{
		Name: "NoActions",
		Permissions: []Permission{{
			Actions:    []string{},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 0 {
		t.Errorf("Empty actions score = %d, want 0", score.Total)
	}
}

func TestScorePermissions_MultipleBlocks(t *testing.T) {
	role := &RoleDefinition{
		Name: "Multiple",
		Permissions: []Permission{
			{
				Actions:    []string{"*/read"},
				NotActions: []string{},
			},
			{
				Actions:    []string{"Microsoft.Authorization/*"},
				NotActions: []string{},
			},
		},
	}

	score := ScorePermissions(role)

	// Should use highest scoring block (auth block = 85)
	if score.Total != 85 {
		t.Errorf("Multiple blocks score = %d, want 85 (highest)", score.Total)
	}
}

func TestScorePermissions_Deterministic(t *testing.T) {
	role := &RoleDefinition{
		Name: "Owner",
		Permissions: []Permission{{
			Actions:    []string{"*"},
			NotActions: []string{},
		}},
	}

	score1 := ScorePermissions(role)
	score2 := ScorePermissions(role)

	if score1.Total != score2.Total {
		t.Errorf("Non-deterministic: first = %d, second = %d", score1.Total, score2.Total)
	}
	if score1.HasWildcard != score2.HasWildcard {
		t.Error("Non-deterministic HasWildcard")
	}
	if score1.HasAuthWrite != score2.HasAuthWrite {
		t.Error("Non-deterministic HasAuthWrite")
	}
}

func TestScorePermissions_ProviderWildcards(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		wantMin int
		wantMax int
	}{
		{
			name:    "single provider",
			actions: []string{"Microsoft.Compute/*"},
			wantMin: 50,
			wantMax: 55,
		},
		{
			name:    "two providers",
			actions: []string{"Microsoft.Compute/*", "Microsoft.Storage/*"},
			wantMin: 55,
			wantMax: 60,
		},
		{
			name:    "many providers",
			actions: []string{"Microsoft.Compute/*", "Microsoft.Storage/*", "Microsoft.Network/*", "Microsoft.Web/*"},
			wantMin: 60,
			wantMax: 65,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			role := &RoleDefinition{
				Permissions: []Permission{{
					Actions:    tc.actions,
					NotActions: []string{},
				}},
			}
			score := ScorePermissions(role)

			if score.Total < tc.wantMin || score.Total > tc.wantMax {
				t.Errorf("score = %d, want [%d, %d]", score.Total, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func TestScorePermissions_LimitedWrite(t *testing.T) {
	role := &RoleDefinition{
		Name: "Custom Write",
		Permissions: []Permission{{
			Actions: []string{
				"Microsoft.Compute/virtualMachines/write",
				"Microsoft.Storage/storageAccounts/listKeys/action",
			},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if score.Total != 30 {
		t.Errorf("Limited write score = %d, want 30", score.Total)
	}
}

func TestScorePermissions_Factors(t *testing.T) {
	role := &RoleDefinition{
		Name: "Owner",
		Permissions: []Permission{{
			Actions:    []string{"*"},
			NotActions: []string{},
		}},
	}

	score := ScorePermissions(role)

	if len(score.Factors) == 0 {
		t.Error("expected at least one factor")
	}
}

func TestMatchesAny(t *testing.T) {
	patterns := []string{"Microsoft.Compute/*", "Microsoft.Storage/*"}

	tests := []struct {
		action string
		want   bool
	}{
		{"Microsoft.Compute/virtualMachines/read", true},
		{"Microsoft.Storage/storageAccounts/read", true},
		{"Microsoft.Network/virtualNetworks/read", false},
	}

	for _, tc := range tests {
		got := matchesAny(tc.action, patterns)
		if got != tc.want {
			t.Errorf("matchesAny(%q, patterns) = %v, want %v", tc.action, got, tc.want)
		}
	}
}

func TestContainsPattern(t *testing.T) {
	patterns := []string{"Microsoft.Compute/*", "*", "*/read"}

	tests := []struct {
		pattern string
		want    bool
	}{
		{"*", true},
		{"*/read", true},
		{"Microsoft.Compute/*", true},
		{"Microsoft.Storage/*", false},
	}

	for _, tc := range tests {
		got := containsPattern(patterns, tc.pattern)
		if got != tc.want {
			t.Errorf("containsPattern(patterns, %q) = %v, want %v", tc.pattern, got, tc.want)
		}
	}
}

func TestCoversAuthorizationWrite(t *testing.T) {
	tests := []struct {
		name       string
		notActions []string
		want       bool
	}{
		{
			name:       "explicit wildcard",
			notActions: []string{"Microsoft.Authorization/*"},
			want:       true,
		},
		{
			name:       "write exclusion",
			notActions: []string{"Microsoft.Authorization/*/Write"},
			want:       true,
		},
		{
			name:       "delete exclusion",
			notActions: []string{"Microsoft.Authorization/*/Delete"},
			want:       true,
		},
		{
			name:       "no exclusion",
			notActions: []string{"Microsoft.Compute/*"},
			want:       false,
		},
		{
			name:       "empty",
			notActions: []string{},
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := coversAuthorizationWrite(tc.notActions)
			if got != tc.want {
				t.Errorf("coversAuthorizationWrite(%v) = %v, want %v", tc.notActions, got, tc.want)
			}
		})
	}
}

func TestIsReadOnlyPermissions(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    bool
	}{
		{
			name:    "single read",
			actions: []string{"*/read"},
			want:    true,
		},
		{
			name:    "specific read",
			actions: []string{"Microsoft.Compute/virtualMachines/read"},
			want:    true,
		},
		{
			name:    "multiple reads",
			actions: []string{"Microsoft.Compute/virtualMachines/read", "Microsoft.Storage/storageAccounts/read"},
			want:    true,
		},
		{
			name:    "includes write",
			actions: []string{"*/read", "Microsoft.Compute/virtualMachines/write"},
			want:    false,
		},
		{
			name:    "provider wildcard",
			actions: []string{"Microsoft.Compute/*"},
			want:    false,
		},
		{
			name:    "empty",
			actions: []string{},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isReadOnlyPermissions(tc.actions, nil)
			if got != tc.want {
				t.Errorf("isReadOnlyPermissions(%v) = %v, want %v", tc.actions, got, tc.want)
			}
		})
	}
}

func TestCountProviderWildcards(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    int
	}{
		{
			name:    "none",
			actions: []string{"Microsoft.Compute/virtualMachines/read"},
			want:    0,
		},
		{
			name:    "one",
			actions: []string{"Microsoft.Compute/*"},
			want:    1,
		},
		{
			name:    "multiple",
			actions: []string{"Microsoft.Compute/*", "Microsoft.Storage/*", "Microsoft.Network/*"},
			want:    3,
		},
		{
			name:    "global wildcard not counted",
			actions: []string{"*"},
			want:    0,
		},
		{
			name:    "mixed",
			actions: []string{"Microsoft.Compute/*", "*/read", "*"},
			want:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countProviderWildcards(tc.actions)
			if got != tc.want {
				t.Errorf("countProviderWildcards(%v) = %d, want %d", tc.actions, got, tc.want)
			}
		})
	}
}

// Test scoring with actual embedded roles
func TestScorePermissions_EmbeddedOwner(t *testing.T) {
	db := DefaultRoleDatabase()
	role, found := db.LookupByName("Owner")
	if !found {
		t.Skip("Owner role not found in embedded database")
	}

	score := ScorePermissions(role)

	if score.Total != 95 {
		t.Errorf("Embedded Owner score = %d, want 95", score.Total)
	}
}

func TestScorePermissions_EmbeddedContributor(t *testing.T) {
	db := DefaultRoleDatabase()
	role, found := db.LookupByName("Contributor")
	if !found {
		t.Skip("Contributor role not found in embedded database")
	}

	score := ScorePermissions(role)

	if score.Total != 70 {
		t.Errorf("Embedded Contributor score = %d, want 70", score.Total)
	}
}

func TestScorePermissions_EmbeddedReader(t *testing.T) {
	db := DefaultRoleDatabase()
	role, found := db.LookupByName("Reader")
	if !found {
		t.Skip("Reader role not found in embedded database")
	}

	score := ScorePermissions(role)

	if score.Total != 15 {
		t.Errorf("Embedded Reader score = %d, want 15", score.Total)
	}
}

func TestScorePermissions_EmbeddedUAA(t *testing.T) {
	db := DefaultRoleDatabase()
	role, found := db.LookupByName("User Access Administrator")
	if !found {
		t.Skip("User Access Administrator role not found in embedded database")
	}

	score := ScorePermissions(role)

	if score.Total != 85 {
		t.Errorf("Embedded UAA score = %d, want 85", score.Total)
	}
}

func TestActionMatchesPattern_NoMatch(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
	}{
		{"Microsoft.Compute/virtualMachines/read", "Microsoft.Storage/*"},
		{"Microsoft.Compute/virtualMachines/write", "*/read"},
		{"Microsoft.Compute/virtualMachines", "Microsoft.Network/virtualNetworks"},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got {
			t.Errorf("actionMatchesPattern(%q, %q) = true, want false", tc.action, tc.pattern)
		}
	}
}

func TestCoversAuthorizationWrite_RoleAssignmentsWildcard(t *testing.T) {
	// Test for Microsoft.Authorization/roleAssignments/* pattern
	notActions := []string{"Microsoft.Authorization/roleAssignments/*"}
	got := coversAuthorizationWrite(notActions)
	if got {
		t.Error("expected false for roleAssignments/* (doesn't cover all Authorization)")
	}
}

func TestCoversAuthorizationWrite_PartialPath(t *testing.T) {
	// Test partial paths ending in /write - these DO match per the function logic
	// as they contain "write" and end with "/write"
	notActions := []string{
		"Microsoft.Authorization/locks/write",
		"Microsoft.Authorization/policyAssignments/write",
	}
	got := coversAuthorizationWrite(notActions)
	if !got {
		t.Error("expected true - paths ending in /write do match")
	}
}

func TestCoversAuthorizationWrite_ReadOnly(t *testing.T) {
	// Test paths that shouldn't match (no write/delete)
	notActions := []string{
		"Microsoft.Authorization/locks/read",
		"Microsoft.Authorization/policyAssignments/read",
	}
	got := coversAuthorizationWrite(notActions)
	if got {
		t.Error("expected false for read-only paths")
	}
}

func TestHasAuthorizationAccess_NotExcluded(t *testing.T) {
	// Test when auth action is present but excluded by notActions
	actions := []string{"Microsoft.Authorization/*"}
	notActions := []string{"Microsoft.Authorization/*"}

	got := hasAuthorizationAccess(actions, notActions)
	if got {
		t.Error("expected false when auth action is excluded")
	}
}

func TestHasTargetedRoleAssignmentWrite_Excluded(t *testing.T) {
	// Test when roleAssignments/write is excluded
	actions := []string{"Microsoft.Authorization/roleAssignments/write"}
	notActions := []string{"Microsoft.Authorization/*"}

	got := hasTargetedRoleAssignmentWrite(actions, notActions)
	if got {
		t.Error("expected false when roleAssignments write is excluded")
	}
}

func TestHasTargetedRoleAssignmentWrite_Wildcard(t *testing.T) {
	// Test with roleAssignments/* pattern
	actions := []string{"Microsoft.Authorization/roleAssignments/*"}
	notActions := []string{}

	got := hasTargetedRoleAssignmentWrite(actions, notActions)
	if !got {
		t.Error("expected true for roleAssignments/*")
	}
}
