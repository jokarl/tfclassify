package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// --- helpers -----------------------------------------------------------------

func compilePatterns(t *testing.T, raws ...string) []pathPattern {
	t.Helper()
	patterns := make([]pathPattern, 0, len(raws))
	for _, raw := range raws {
		p, err := compilePathPattern(raw)
		if err != nil {
			t.Fatalf("compilePathPattern(%q): %v", raw, err)
		}
		patterns = append(patterns, p)
	}
	return patterns
}

func isPathCovered(t *testing.T, path string, patterns ...string) bool {
	t.Helper()
	return anyCovers(compilePatterns(t, patterns...), path)
}

func rulesWithGlobal(t *testing.T, entries ...string) *CompiledIgnoreRules {
	t.Helper()
	rules, err := CompileIgnoreRules(&config.DefaultsConfig{IgnoreAttributes: entries})
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}
	return rules
}

// --- pathPattern (bare-prefix backward compat) -------------------------------

func TestPathPattern_ExactMatch(t *testing.T) {
	if !isPathCovered(t, "tags", "tags") {
		t.Error("expected 'tags' to be covered by pattern 'tags'")
	}
}

func TestPathPattern_PrefixMatch(t *testing.T) {
	if !isPathCovered(t, "tags.env", "tags") {
		t.Error("expected 'tags.env' to be covered by pattern 'tags'")
	}
}

func TestPathPattern_DeepPrefixMatch(t *testing.T) {
	if !isPathCovered(t, "tags.env.team", "tags") {
		t.Error("expected 'tags.env.team' to be covered by bare prefix 'tags'")
	}
}

func TestPathPattern_NoFalsePrefix(t *testing.T) {
	if isPathCovered(t, "tags_all", "tags") {
		t.Error("'tags_all' must NOT be covered by pattern 'tags'")
	}
}

func TestPathPattern_NestedPrefix(t *testing.T) {
	if !isPathCovered(t, "meta.tags.env", "meta.tags") {
		t.Error("expected 'meta.tags.env' to be covered by pattern 'meta.tags'")
	}
	if isPathCovered(t, "meta.name", "meta.tags") {
		t.Error("'meta.name' must NOT be covered by pattern 'meta.tags'")
	}
}

func TestPathPattern_MultiplePatterns(t *testing.T) {
	if !isPathCovered(t, "tags.env", "tags", "tags_all") {
		t.Error("expected 'tags.env' to be covered")
	}
	if !isPathCovered(t, "tags_all.env", "tags", "tags_all") {
		t.Error("expected 'tags_all.env' to be covered")
	}
	if isPathCovered(t, "name", "tags", "tags_all") {
		t.Error("'name' must NOT be covered")
	}
}

// --- pathPattern (glob semantics) --------------------------------------------

func TestPathPattern_TailWildcard(t *testing.T) {
	if !isPathCovered(t, "tags.temp_foo", "tags.temp_*") {
		t.Error("expected 'tags.temp_foo' to be covered by 'tags.temp_*'")
	}
	if isPathCovered(t, "tags.keep", "tags.temp_*") {
		t.Error("'tags.keep' must NOT be covered by 'tags.temp_*'")
	}
}

func TestPathPattern_MidWildcard(t *testing.T) {
	if !isPathCovered(t, "properties.rule1.tags", "properties.*.tags") {
		t.Error("expected 'properties.rule1.tags' to be covered by 'properties.*.tags'")
	}
	if !isPathCovered(t, "properties.rule2.tags", "properties.*.tags") {
		t.Error("expected 'properties.rule2.tags' to be covered by 'properties.*.tags'")
	}
	if isPathCovered(t, "properties.tags", "properties.*.tags") {
		t.Error("'properties.tags' must NOT be covered by 'properties.*.tags' (segment counts differ)")
	}
}

func TestPathPattern_LeadingWildcard(t *testing.T) {
	if !isPathCovered(t, "meta.tags", "*.tags") {
		t.Error("expected 'meta.tags' to be covered by '*.tags'")
	}
	if !isPathCovered(t, "spec.tags", "*.tags") {
		t.Error("expected 'spec.tags' to be covered by '*.tags'")
	}
	if isPathCovered(t, "tags", "*.tags") {
		t.Error("'tags' (single segment) must NOT be covered by '*.tags' (two segments)")
	}
}

// --- collectChangedPaths ------------------------------------------------------

func TestCollectChangedPaths_TopLevel(t *testing.T) {
	before := map[string]interface{}{
		"name": "foo",
		"tags": map[string]interface{}{"env": "prod", "v": "1.0"},
	}
	after := map[string]interface{}{
		"name": "foo",
		"tags": map[string]interface{}{"env": "prod", "v": "1.1"},
	}

	paths := collectChangedPaths(before, after, "")
	if len(paths) != 1 || paths[0] != "tags.v" {
		t.Errorf("expected [tags.v], got %v", paths)
	}
}

func TestCollectChangedPaths_MultipleTags(t *testing.T) {
	before := map[string]interface{}{
		"tags": map[string]interface{}{"v": "1.0", "env": "prod"},
	}
	after := map[string]interface{}{
		"tags": map[string]interface{}{"v": "1.1", "env": "staging"},
	}

	paths := collectChangedPaths(before, after, "")
	if len(paths) != 2 {
		t.Errorf("expected 2 changed paths, got %d: %v", len(paths), paths)
	}
}

func TestCollectChangedPaths_NoChanges(t *testing.T) {
	data := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"env": "prod"}}
	paths := collectChangedPaths(data, data, "")
	if len(paths) != 0 {
		t.Errorf("expected no changed paths, got %v", paths)
	}
}

func TestCollectChangedPaths_Nested(t *testing.T) {
	before := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"env": "prod"},
			"name": "foo",
		},
	}
	after := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"env": "staging"},
			"name": "foo",
		},
	}

	paths := collectChangedPaths(before, after, "")
	if len(paths) != 1 || paths[0] != "meta.tags.env" {
		t.Errorf("expected [meta.tags.env], got %v", paths)
	}
}

func TestCollectChangedPaths_AddedAttribute(t *testing.T) {
	before := map[string]interface{}{"name": "foo"}
	after := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"env": "prod"}}

	paths := collectChangedPaths(before, after, "")
	if len(paths) != 1 || paths[0] != "tags" {
		t.Errorf("expected [tags], got %v", paths)
	}
}

func TestCollectChangedPaths_RemovedAttribute(t *testing.T) {
	before := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"env": "prod"}}
	after := map[string]interface{}{"name": "foo"}

	paths := collectChangedPaths(before, after, "")
	if len(paths) != 1 || paths[0] != "tags" {
		t.Errorf("expected [tags], got %v", paths)
	}
}

// --- hasOnlyIgnoredChanges ---------------------------------------------------

func TestHasOnlyIgnoredChanges_AllCovered(t *testing.T) {
	before := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"v": "1.0"}}
	after := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"v": "1.1"}}

	if !hasOnlyIgnoredChanges(before, after, "", compilePatterns(t, "tags")) {
		t.Error("expected all changes to be covered by 'tags' prefix")
	}
}

func TestHasOnlyIgnoredChanges_Uncovered(t *testing.T) {
	before := map[string]interface{}{"name": "foo", "tags": map[string]interface{}{"v": "1.0"}}
	after := map[string]interface{}{"name": "bar", "tags": map[string]interface{}{"v": "1.1"}}

	if hasOnlyIgnoredChanges(before, after, "", compilePatterns(t, "tags")) {
		t.Error("expected uncovered change on 'name'")
	}
}

func TestHasOnlyIgnoredChanges_NestedPrefix(t *testing.T) {
	before := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"v": "1.0"},
			"name": "foo",
		},
	}
	after := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"v": "1.1"},
			"name": "foo",
		},
	}

	if !hasOnlyIgnoredChanges(before, after, "", compilePatterns(t, "meta.tags")) {
		t.Error("expected all changes to be covered by 'meta.tags' prefix")
	}
}

func TestHasOnlyIgnoredChanges_NestedPrefixPartial(t *testing.T) {
	before := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"v": "1.0"},
			"name": "foo",
		},
	}
	after := map[string]interface{}{
		"meta": map[string]interface{}{
			"tags": map[string]interface{}{"v": "1.1"},
			"name": "bar",
		},
	}

	if hasOnlyIgnoredChanges(before, after, "", compilePatterns(t, "meta.tags")) {
		t.Error("expected uncovered change on 'meta.name'")
	}
}

// --- FilterCosmeticChanges (global list — CR-0034 semantics) -----------------

func TestFilterCosmeticChanges_TagOnly(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_search_service.this",
			Type:    "azurerm_search_service",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"name": "svc", "tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"name": "svc", "tags": map[string]interface{}{"v": "1.1"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if len(changes[0].Actions) != 1 || changes[0].Actions[0] != "no-op" {
		t.Errorf("expected actions [no-op], got %v", changes[0].Actions)
	}
	if len(changes[0].OriginalActions) != 1 || changes[0].OriginalActions[0] != "update" {
		t.Errorf("expected original_actions [update], got %v", changes[0].OriginalActions)
	}
	if len(changes[0].IgnoredAttributes) != 1 || changes[0].IgnoredAttributes[0] != "tags.v" {
		t.Errorf("expected ignored_attributes [tags.v], got %v", changes[0].IgnoredAttributes)
	}
	if len(changes[0].IgnoreRuleMatches) != 0 {
		t.Errorf("global list must not populate IgnoreRuleMatches, got %v", changes[0].IgnoreRuleMatches)
	}
}

func TestFilterCosmeticChanges_MixedChange(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_storage_account.this",
			Type:    "azurerm_storage_account",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"sku": "Standard", "tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"sku": "Premium", "tags": map[string]interface{}{"v": "1.1"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].Actions[0] != "update" {
		t.Errorf("expected actions unchanged [update], got %v", changes[0].Actions)
	}
	if changes[0].OriginalActions != nil {
		t.Errorf("expected nil original_actions, got %v", changes[0].OriginalActions)
	}
}

func TestFilterCosmeticChanges_CreateNotAffected(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.new",
			Type:    "azurerm_resource_group",
			Actions: []string{"create"},
			After:   map[string]interface{}{"name": "rg", "tags": map[string]interface{}{"v": "1.0"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].Actions[0] != "create" {
		t.Errorf("expected create unchanged, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_DeleteNotAffected(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.old",
			Type:    "azurerm_resource_group",
			Actions: []string{"delete"},
			Before:  map[string]interface{}{"name": "rg", "tags": map[string]interface{}{"v": "1.0"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].Actions[0] != "delete" {
		t.Errorf("expected delete unchanged, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_ReplaceNotAffected(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.replaced",
			Type:    "azurerm_resource_group",
			Actions: []string{"delete", "create"},
			Before:  map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"tags": map[string]interface{}{"v": "1.1"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if len(changes[0].Actions) != 2 {
		t.Errorf("expected replace unchanged, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_EmptyIgnoreList(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_search_service.this",
			Type:    "azurerm_search_service",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"tags": map[string]interface{}{"v": "1.1"}},
		},
	}

	FilterCosmeticChanges(changes, nil)

	if changes[0].Actions[0] != "update" {
		t.Errorf("expected no filtering with empty ignore list, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_MultiplePrefixes(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_vnet.this",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"name":     "vnet",
				"tags":     map[string]interface{}{"v": "1.0"},
				"tags_all": map[string]interface{}{"v": "1.0", "managed": "true"},
			},
			After: map[string]interface{}{
				"name":     "vnet",
				"tags":     map[string]interface{}{"v": "1.1"},
				"tags_all": map[string]interface{}{"v": "1.1", "managed": "true"},
			},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags", "tags_all"))

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("expected [no-op] with multiple prefixes, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_MultipleResources(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "res.cosmetic",
			Type:    "azurerm_search_service",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"name": "svc", "tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"name": "svc", "tags": map[string]interface{}{"v": "1.1"}},
		},
		{
			Address: "res.real",
			Type:    "azurerm_storage_account",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"name": "old", "tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"name": "new", "tags": map[string]interface{}{"v": "1.1"}},
		},
		{
			Address: "res.noop",
			Type:    "azurerm_resource_group",
			Actions: []string{"no-op"},
			Before:  map[string]interface{}{"name": "rg"},
			After:   map[string]interface{}{"name": "rg"},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("res.cosmetic: expected [no-op], got %v", changes[0].Actions)
	}
	if changes[1].Actions[0] != "update" {
		t.Errorf("res.real: expected [update], got %v", changes[1].Actions)
	}
	if changes[2].Actions[0] != "no-op" {
		t.Errorf("res.noop: expected [no-op], got %v", changes[2].Actions)
	}
	if changes[0].OriginalActions == nil {
		t.Error("res.cosmetic: expected original_actions to be set")
	}
	if changes[1].OriginalActions != nil {
		t.Error("res.real: expected original_actions to be nil")
	}
	if changes[2].OriginalActions != nil {
		t.Error("res.noop: expected original_actions to be nil")
	}
}

func TestFilterCosmeticChanges_NilBeforeAfter(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "res.update_nil",
			Type:    "some_type",
			Actions: []string{"update"},
			Before:  nil,
			After:   nil,
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].Actions[0] != "update" {
		t.Errorf("expected update unchanged when before/after nil, got %v", changes[0].Actions)
	}
}

func TestFilterCosmeticChanges_NoOpNotReprocessed(t *testing.T) {
	changes := []plan.ResourceChange{
		{
			Address: "res.already_noop",
			Type:    "some_type",
			Actions: []string{"no-op"},
			Before:  map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
			After:   map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
		},
	}

	FilterCosmeticChanges(changes, rulesWithGlobal(t, "tags"))

	if changes[0].OriginalActions != nil {
		t.Error("no-op resources should not get original_actions annotation")
	}
}

// --- FilterCosmeticChanges (scoped blocks — CR-0035) -------------------------

func TestFilter_ScopedRule_MatchesAzapiOnly(t *testing.T) {
	defaults := &config.DefaultsConfig{
		IgnoreAttributes: []string{"tags"},
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "azapi_output",
			Description: "azapi computed refresh",
			Resource:    []string{"azapi_resource"},
			Attributes:  []string{"output"},
		}},
	}
	rules, err := CompileIgnoreRules(defaults)
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azapi_resource.this",
			Type:    "azapi_resource",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"tags":   map[string]interface{}{"v": "1.0"},
				"output": map[string]interface{}{"id": "before"},
			},
			After: map[string]interface{}{
				"tags":   map[string]interface{}{"v": "1.1"},
				"output": map[string]interface{}{"id": "after"},
			},
		},
		{
			Address: "azurerm_storage_account.this",
			Type:    "azurerm_storage_account",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"tags":   map[string]interface{}{"v": "1.0"},
				"output": map[string]interface{}{"id": "before"},
			},
			After: map[string]interface{}{
				"tags":   map[string]interface{}{"v": "1.1"},
				"output": map[string]interface{}{"id": "after"},
			},
		},
	}

	FilterCosmeticChanges(changes, rules)

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("azapi: expected no-op, got %v", changes[0].Actions)
	}
	if len(changes[0].IgnoreRuleMatches) != 1 || changes[0].IgnoreRuleMatches[0].Name != "azapi_output" {
		t.Errorf("azapi: expected IgnoreRuleMatches=[azapi_output], got %v", changes[0].IgnoreRuleMatches)
	}
	if changes[0].IgnoreRuleMatches[0].Description != "azapi computed refresh" {
		t.Errorf("azapi: expected description propagated, got %q", changes[0].IgnoreRuleMatches[0].Description)
	}

	if changes[1].Actions[0] != "update" {
		t.Errorf("storage: expected update (scoped rule must not apply), got %v", changes[1].Actions)
	}
	if len(changes[1].IgnoreRuleMatches) != 0 {
		t.Errorf("storage: expected no IgnoreRuleMatches, got %v", changes[1].IgnoreRuleMatches)
	}
}

func TestFilter_ScopedRule_UnionWithGlobal(t *testing.T) {
	defaults := &config.DefaultsConfig{
		IgnoreAttributes: []string{"tags"},
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "azapi_output",
			Description: "azapi computed refresh",
			Resource:    []string{"azapi_*"},
			Attributes:  []string{"output"},
		}},
	}
	rules, err := CompileIgnoreRules(defaults)
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}

	changes := []plan.ResourceChange{{
		Address: "azapi_resource.vnet",
		Type:    "azapi_resource",
		Actions: []string{"update"},
		Before: map[string]interface{}{
			"tags":   map[string]interface{}{"env": "prod"},
			"output": map[string]interface{}{"id": "x"},
		},
		After: map[string]interface{}{
			"tags":   map[string]interface{}{"env": "staging"},
			"output": map[string]interface{}{"id": "y"},
		},
	}}

	FilterCosmeticChanges(changes, rules)

	if changes[0].Actions[0] != "no-op" {
		t.Fatalf("expected no-op, got %v", changes[0].Actions)
	}
	if len(changes[0].IgnoredAttributes) != 2 {
		t.Errorf("expected 2 ignored paths, got %v", changes[0].IgnoredAttributes)
	}
	if len(changes[0].IgnoreRuleMatches) != 1 {
		t.Fatalf("expected 1 scoped rule attributed, got %v", changes[0].IgnoreRuleMatches)
	}
	match := changes[0].IgnoreRuleMatches[0]
	if match.Name != "azapi_output" {
		t.Errorf("expected rule name azapi_output, got %q", match.Name)
	}
	if len(match.Paths) != 1 || match.Paths[0] != "output.id" {
		t.Errorf("expected attributed path [output.id], got %v", match.Paths)
	}
}

func TestFilter_ScopedRule_DoesNotApplyIfResourceUnmatched(t *testing.T) {
	defaults := &config.DefaultsConfig{
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "azapi_only",
			Description: "only azapi",
			Resource:    []string{"azapi_resource"},
			Attributes:  []string{"output"},
		}},
	}
	rules, err := CompileIgnoreRules(defaults)
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}

	changes := []plan.ResourceChange{{
		Address: "azurerm_key_vault.this",
		Type:    "azurerm_key_vault",
		Actions: []string{"update"},
		Before:  map[string]interface{}{"output": map[string]interface{}{"id": "x"}},
		After:   map[string]interface{}{"output": map[string]interface{}{"id": "y"}},
	}}

	FilterCosmeticChanges(changes, rules)

	if changes[0].Actions[0] != "update" {
		t.Errorf("expected update on unmatched resource, got %v", changes[0].Actions)
	}
	if len(changes[0].IgnoreRuleMatches) != 0 {
		t.Errorf("expected no rule matches, got %v", changes[0].IgnoreRuleMatches)
	}
}

func TestFilter_ScopedRule_ModuleFilter(t *testing.T) {
	defaults := &config.DefaultsConfig{
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "platform_only",
			Description: "only for platform module tree",
			Module:      []string{"module.platform.**"},
			Attributes:  []string{"tags"},
		}},
	}
	rules, err := CompileIgnoreRules(defaults)
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address:       "module.platform.app.azurerm_rg.r",
			Type:          "azurerm_resource_group",
			Actions:       []string{"update"},
			ModuleAddress: "module.platform.app",
			Before:        map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
			After:         map[string]interface{}{"tags": map[string]interface{}{"v": "1.1"}},
		},
		{
			Address:       "module.other.azurerm_rg.r",
			Type:          "azurerm_resource_group",
			Actions:       []string{"update"},
			ModuleAddress: "module.other",
			Before:        map[string]interface{}{"tags": map[string]interface{}{"v": "1.0"}},
			After:         map[string]interface{}{"tags": map[string]interface{}{"v": "1.1"}},
		},
	}

	FilterCosmeticChanges(changes, rules)

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("platform: expected no-op, got %v", changes[0].Actions)
	}
	if changes[1].Actions[0] != "update" {
		t.Errorf("other module: expected update, got %v", changes[1].Actions)
	}
}

func TestFilter_PathGlob_MidWildcard(t *testing.T) {
	defaults := &config.DefaultsConfig{
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "timestamps",
			Description: "transient timestamps",
			Resource:    []string{"*"},
			Attributes:  []string{"properties.*.createdAt"},
		}},
	}
	rules, err := CompileIgnoreRules(defaults)
	if err != nil {
		t.Fatalf("CompileIgnoreRules: %v", err)
	}

	changes := []plan.ResourceChange{{
		Address: "r.this",
		Type:    "r",
		Actions: []string{"update"},
		Before: map[string]interface{}{
			"properties": map[string]interface{}{
				"rule1": map[string]interface{}{"createdAt": "t0"},
				"rule2": map[string]interface{}{"createdAt": "t0"},
			},
		},
		After: map[string]interface{}{
			"properties": map[string]interface{}{
				"rule1": map[string]interface{}{"createdAt": "t1"},
				"rule2": map[string]interface{}{"createdAt": "t1"},
			},
		},
	}}

	FilterCosmeticChanges(changes, rules)

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("expected no-op, got %v", changes[0].Actions)
	}
}

// --- CompileIgnoreRules (validation) -----------------------------------------

func TestCompileIgnoreRules_NilOrEmpty(t *testing.T) {
	if r, err := CompileIgnoreRules(nil); err != nil || r != nil {
		t.Errorf("expected (nil, nil) for nil defaults, got (%v, %v)", r, err)
	}
	if r, err := CompileIgnoreRules(&config.DefaultsConfig{}); err != nil || r != nil {
		t.Errorf("expected (nil, nil) for empty defaults, got (%v, %v)", r, err)
	}
}

func TestCompileIgnoreRules_InvalidGlob(t *testing.T) {
	_, err := CompileIgnoreRules(&config.DefaultsConfig{
		IgnoreAttributeRules: []config.IgnoreAttributeRule{{
			Name:        "bad",
			Description: "bad glob",
			Attributes:  []string{"["},
		}},
	})
	if err == nil {
		t.Fatal("expected compile error for invalid glob, got nil")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("expected error to mention rule name, got %v", err)
	}
}
