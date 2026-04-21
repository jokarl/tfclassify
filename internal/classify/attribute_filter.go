package classify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// CompiledIgnoreRules holds pre-compiled global and scoped ignore rules for
// a single plan-filtering pass. Build once per config load with
// CompileIgnoreRules and reuse across every change.
type CompiledIgnoreRules struct {
	global []pathPattern
	scoped []compiledIgnoreRule
}

// Empty reports whether the compiled set has any rules at all.
func (r *CompiledIgnoreRules) Empty() bool {
	return r == nil || (len(r.global) == 0 && len(r.scoped) == 0)
}

type compiledIgnoreRule struct {
	name             string
	description      string
	resourceGlobs    []glob.Glob
	notResourceGlobs []glob.Glob
	moduleGlobs      []glob.Glob
	notModuleGlobs   []glob.Glob
	patterns         []pathPattern
}

// matches reports whether this scoped rule applies to a given resource.
func (r *compiledIgnoreRule) matches(resourceType, moduleAddress string) bool {
	if !matchResource(resourceType, r.resourceGlobs, r.notResourceGlobs) {
		return false
	}
	if !matchModule(moduleAddress, r.moduleGlobs, r.notModuleGlobs) {
		return false
	}
	return true
}

// pathPattern is a dot-segmented glob pattern. A pattern covers a concrete
// dot-separated path if its segment count is less than or equal to the path's
// and every pattern segment matches the corresponding path segment.
type pathPattern struct {
	raw      string
	segments []glob.Glob
}

func compilePathPattern(raw string) (pathPattern, error) {
	parts := strings.Split(raw, ".")
	segments := make([]glob.Glob, 0, len(parts))
	for i, seg := range parts {
		g, err := glob.Compile(seg)
		if err != nil {
			return pathPattern{}, fmt.Errorf("segment %d %q: %w", i, seg, err)
		}
		segments = append(segments, g)
	}
	return pathPattern{raw: raw, segments: segments}, nil
}

func (p pathPattern) covers(path string) bool {
	if len(p.segments) == 0 {
		return false
	}
	parts := strings.Split(path, ".")
	if len(parts) < len(p.segments) {
		return false
	}
	for i, seg := range p.segments {
		if !seg.Match(parts[i]) {
			return false
		}
	}
	return true
}

// CompileIgnoreRules compiles the defaults.ignore_attributes global list and
// every defaults.ignore_attribute block into a reusable matcher. Returns nil
// when the defaults block is absent or has no ignore rules.
func CompileIgnoreRules(d *config.DefaultsConfig) (*CompiledIgnoreRules, error) {
	if d == nil {
		return nil, nil
	}
	if len(d.IgnoreAttributes) == 0 && len(d.IgnoreAttributeRules) == 0 {
		return nil, nil
	}

	rules := &CompiledIgnoreRules{}

	for _, raw := range d.IgnoreAttributes {
		p, err := compilePathPattern(raw)
		if err != nil {
			return nil, fmt.Errorf("defaults.ignore_attributes %q: %w", raw, err)
		}
		rules.global = append(rules.global, p)
	}

	for _, r := range d.IgnoreAttributeRules {
		compiled := compiledIgnoreRule{
			name:        r.Name,
			description: r.Description,
		}
		for _, pattern := range r.Resource {
			g, err := glob.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("defaults.ignore_attribute %q: resource %q: %w", r.Name, pattern, err)
			}
			compiled.resourceGlobs = append(compiled.resourceGlobs, g)
		}
		for _, pattern := range r.NotResource {
			g, err := glob.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("defaults.ignore_attribute %q: not_resource %q: %w", r.Name, pattern, err)
			}
			compiled.notResourceGlobs = append(compiled.notResourceGlobs, g)
		}
		for _, pattern := range r.Module {
			g, err := glob.Compile(pattern, '.')
			if err != nil {
				return nil, fmt.Errorf("defaults.ignore_attribute %q: module %q: %w", r.Name, pattern, err)
			}
			compiled.moduleGlobs = append(compiled.moduleGlobs, g)
		}
		for _, pattern := range r.NotModule {
			g, err := glob.Compile(pattern, '.')
			if err != nil {
				return nil, fmt.Errorf("defaults.ignore_attribute %q: not_module %q: %w", r.Name, pattern, err)
			}
			compiled.notModuleGlobs = append(compiled.notModuleGlobs, g)
		}
		for _, attr := range r.Attributes {
			p, err := compilePathPattern(attr)
			if err != nil {
				return nil, fmt.Errorf("defaults.ignore_attribute %q: attribute %q: %w", r.Name, attr, err)
			}
			compiled.patterns = append(compiled.patterns, p)
		}
		rules.scoped = append(rules.scoped, compiled)
	}

	return rules, nil
}

// FilterCosmeticChanges preprocesses plan changes, downgrading cosmetic-only
// updates to no-op. A change is cosmetic if ALL its changed attributes are
// covered by the effective ignore rules for that resource — the global list
// plus every scoped rule whose resource/module filter matches. Only
// ["update"] actions are evaluated; creates, deletes, and replacements are
// never affected.
//
// When a resource is downgraded, its OriginalActions, IgnoredAttributes, and
// IgnoreRuleMatches fields are populated for output visibility.
func FilterCosmeticChanges(changes []plan.ResourceChange, rules *CompiledIgnoreRules) {
	if rules.Empty() {
		return
	}

	for i := range changes {
		change := &changes[i]

		if !isUpdateOnly(change.Actions) {
			continue
		}
		if change.Before == nil || change.After == nil {
			continue
		}

		effective := effectivePatterns(rules, change.Type, change.ModuleAddress)
		if len(effective) == 0 {
			continue
		}

		if !hasOnlyIgnoredChanges(change.Before, change.After, "", effective) {
			continue
		}

		changedPaths := collectChangedPaths(change.Before, change.After, "")
		if len(changedPaths) == 0 {
			continue
		}

		change.OriginalActions = change.Actions
		change.IgnoredAttributes = changedPaths
		change.IgnoreRuleMatches = attributeMatches(rules, change.Type, change.ModuleAddress, changedPaths)
		change.Actions = []string{"no-op"}
	}
}

// effectivePatterns returns the union of the global patterns and the patterns
// of every scoped rule whose filter matches the given resource.
func effectivePatterns(rules *CompiledIgnoreRules, resourceType, moduleAddress string) []pathPattern {
	if rules == nil {
		return nil
	}
	effective := make([]pathPattern, 0, len(rules.global))
	effective = append(effective, rules.global...)
	for idx := range rules.scoped {
		r := &rules.scoped[idx]
		if r.matches(resourceType, moduleAddress) {
			effective = append(effective, r.patterns...)
		}
	}
	return effective
}

// attributeMatches returns the scoped rules that covered at least one of the
// given changed paths. Global (unnamed) patterns are not reported here.
func attributeMatches(rules *CompiledIgnoreRules, resourceType, moduleAddress string, paths []string) []plan.IgnoreRuleMatch {
	if rules == nil || len(rules.scoped) == 0 {
		return nil
	}
	var out []plan.IgnoreRuleMatch
	for idx := range rules.scoped {
		r := &rules.scoped[idx]
		if !r.matches(resourceType, moduleAddress) {
			continue
		}
		var covered []string
		for _, path := range paths {
			for _, p := range r.patterns {
				if p.covers(path) {
					covered = append(covered, path)
					break
				}
			}
		}
		if len(covered) == 0 {
			continue
		}
		out = append(out, plan.IgnoreRuleMatch{
			Name:        r.name,
			Description: r.description,
			Paths:       covered,
		})
	}
	return out
}

// hasOnlyIgnoredChanges returns true if every changed attribute between before
// and after (at the given path prefix) is covered by at least one pattern.
// Short-circuits on the first uncovered change.
func hasOnlyIgnoredChanges(before, after map[string]interface{}, pathPrefix string, patterns []pathPattern) bool {
	for _, key := range mergedKeys(before, after) {
		path := key
		if pathPrefix != "" {
			path = pathPrefix + "." + key
		}

		bv, av := before[key], after[key]
		if !valueChanged(bv, av) {
			continue
		}

		if anyCovers(patterns, path) {
			continue
		}

		// Not directly covered. If both values are maps and a nested pattern
		// extends this path, recurse to check sub-attributes.
		bMap, bOk := bv.(map[string]interface{})
		aMap, aOk := av.(map[string]interface{})
		if bOk && aOk && hasNestedPattern(path, patterns) {
			if !hasOnlyIgnoredChanges(bMap, aMap, path, patterns) {
				return false
			}
			continue
		}

		return false
	}
	return true
}

// anyCovers returns true if any pattern covers path.
func anyCovers(patterns []pathPattern, path string) bool {
	for _, p := range patterns {
		if p.covers(path) {
			return true
		}
	}
	return false
}

// hasNestedPattern returns true if any pattern has more segments than path
// and the pattern's leading segments match path segment-for-segment. This
// mirrors the old "a more specific prefix lives under this path" check.
func hasNestedPattern(path string, patterns []pathPattern) bool {
	pathParts := strings.Split(path, ".")
	for _, p := range patterns {
		if len(p.segments) <= len(pathParts) {
			continue
		}
		match := true
		for i, part := range pathParts {
			if !p.segments[i].Match(part) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// collectChangedPaths returns dot-delimited paths of all attributes that
// differ between before and after. Recurses into nested maps to report
// leaf-level paths (e.g., "tags.tf-module-l2" rather than just "tags").
func collectChangedPaths(before, after map[string]interface{}, pathPrefix string) []string {
	var paths []string

	for _, key := range mergedKeys(before, after) {
		path := key
		if pathPrefix != "" {
			path = pathPrefix + "." + key
		}

		bv, av := before[key], after[key]
		if !valueChanged(bv, av) {
			continue
		}

		bMap, bOk := bv.(map[string]interface{})
		aMap, aOk := av.(map[string]interface{})
		if bOk && aOk {
			paths = append(paths, collectChangedPaths(bMap, aMap, path)...)
			continue
		}

		paths = append(paths, path)
	}

	return paths
}

// isUpdateOnly returns true if the actions slice is exactly ["update"].
func isUpdateOnly(actions []string) bool {
	return len(actions) == 1 && actions[0] == "update"
}

// mergedKeys returns the sorted union of keys from two maps.
func mergedKeys(m1, m2 map[string]interface{}) []string {
	seen := make(map[string]struct{}, len(m1)+len(m2))
	for k := range m1 {
		seen[k] = struct{}{}
	}
	for k := range m2 {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// matchResource mirrors classifier rule semantics: allow-list takes priority
// over deny-list; absence of both means "match everything".
func matchResource(resourceType string, allow, deny []glob.Glob) bool {
	if len(allow) > 0 {
		for _, g := range allow {
			if g.Match(resourceType) {
				return true
			}
		}
		return false
	}
	if len(deny) > 0 {
		for _, g := range deny {
			if g.Match(resourceType) {
				return false
			}
		}
		return true
	}
	return true
}

// matchModule mirrors classifier rule semantics for module filters.
func matchModule(moduleAddress string, allow, deny []glob.Glob) bool {
	if len(allow) > 0 {
		for _, g := range allow {
			if g.Match(moduleAddress) {
				return true
			}
		}
		return false
	}
	if len(deny) > 0 {
		for _, g := range deny {
			if g.Match(moduleAddress) {
				return false
			}
		}
		return true
	}
	return true
}
