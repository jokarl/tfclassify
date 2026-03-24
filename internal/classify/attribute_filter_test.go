package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/plan"
)

func TestIsPathCovered_ExactMatch(t *testing.T) {
	if !isPathCovered("tags", []string{"tags"}) {
		t.Error("expected 'tags' to be covered by prefix 'tags'")
	}
}

func TestIsPathCovered_PrefixMatch(t *testing.T) {
	if !isPathCovered("tags.env", []string{"tags"}) {
		t.Error("expected 'tags.env' to be covered by prefix 'tags'")
	}
}

func TestIsPathCovered_NoFalsePrefix(t *testing.T) {
	if isPathCovered("tags_all", []string{"tags"}) {
		t.Error("'tags_all' should NOT be covered by prefix 'tags'")
	}
}

func TestIsPathCovered_NestedPrefix(t *testing.T) {
	if !isPathCovered("meta.tags.env", []string{"meta.tags"}) {
		t.Error("expected 'meta.tags.env' to be covered by prefix 'meta.tags'")
	}
	if isPathCovered("meta.name", []string{"meta.tags"}) {
		t.Error("'meta.name' should NOT be covered by prefix 'meta.tags'")
	}
}

func TestIsPathCovered_MultiplePrefixes(t *testing.T) {
	prefixes := []string{"tags", "tags_all"}
	if !isPathCovered("tags.env", prefixes) {
		t.Error("expected 'tags.env' to be covered")
	}
	if !isPathCovered("tags_all.env", prefixes) {
		t.Error("expected 'tags_all.env' to be covered")
	}
	if isPathCovered("name", prefixes) {
		t.Error("'name' should NOT be covered")
	}
}

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

func TestHasOnlyIgnoredChanges_AllCovered(t *testing.T) {
	before := map[string]interface{}{
		"name": "foo",
		"tags": map[string]interface{}{"v": "1.0"},
	}
	after := map[string]interface{}{
		"name": "foo",
		"tags": map[string]interface{}{"v": "1.1"},
	}

	if !hasOnlyIgnoredChanges(before, after, "", []string{"tags"}) {
		t.Error("expected all changes to be covered by 'tags' prefix")
	}
}

func TestHasOnlyIgnoredChanges_Uncovered(t *testing.T) {
	before := map[string]interface{}{
		"name": "foo",
		"tags": map[string]interface{}{"v": "1.0"},
	}
	after := map[string]interface{}{
		"name": "bar",
		"tags": map[string]interface{}{"v": "1.1"},
	}

	if hasOnlyIgnoredChanges(before, after, "", []string{"tags"}) {
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

	if !hasOnlyIgnoredChanges(before, after, "", []string{"meta.tags"}) {
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

	if hasOnlyIgnoredChanges(before, after, "", []string{"meta.tags"}) {
		t.Error("expected uncovered change on 'meta.name'")
	}
}

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

	FilterCosmeticChanges(changes, []string{"tags"})

	if len(changes[0].Actions) != 1 || changes[0].Actions[0] != "no-op" {
		t.Errorf("expected actions [no-op], got %v", changes[0].Actions)
	}
	if len(changes[0].OriginalActions) != 1 || changes[0].OriginalActions[0] != "update" {
		t.Errorf("expected original_actions [update], got %v", changes[0].OriginalActions)
	}
	if len(changes[0].IgnoredAttributes) != 1 || changes[0].IgnoredAttributes[0] != "tags.v" {
		t.Errorf("expected ignored_attributes [tags.v], got %v", changes[0].IgnoredAttributes)
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

	FilterCosmeticChanges(changes, []string{"tags"})

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

	FilterCosmeticChanges(changes, []string{"tags"})

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

	FilterCosmeticChanges(changes, []string{"tags"})

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

	FilterCosmeticChanges(changes, []string{"tags"})

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

	FilterCosmeticChanges(changes, []string{"tags", "tags_all"})

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

	FilterCosmeticChanges(changes, []string{"tags"})

	if changes[0].Actions[0] != "no-op" {
		t.Errorf("res.cosmetic: expected [no-op], got %v", changes[0].Actions)
	}
	if changes[1].Actions[0] != "update" {
		t.Errorf("res.real: expected [update], got %v", changes[1].Actions)
	}
	if changes[2].Actions[0] != "no-op" {
		t.Errorf("res.noop: expected [no-op], got %v", changes[2].Actions)
	}
	// Only the cosmetic resource should have annotations
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

	// Should not panic
	FilterCosmeticChanges(changes, []string{"tags"})

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

	FilterCosmeticChanges(changes, []string{"tags"})

	if changes[0].OriginalActions != nil {
		t.Error("no-op resources should not get original_actions annotation")
	}
}
