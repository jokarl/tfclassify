// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// Validate checks that the configuration is valid.
func Validate(cfg *Config) error {
	// Build classification names set once for all validators
	classificationNames := make(map[string]bool, len(cfg.Classifications))
	for _, c := range cfg.Classifications {
		classificationNames[c.Name] = true
	}

	if err := validatePrecedence(cfg, classificationNames); err != nil {
		return err
	}

	if err := validateDefaults(cfg, classificationNames); err != nil {
		return err
	}

	if err := validateClassifications(cfg); err != nil {
		return err
	}

	if err := validateRules(cfg); err != nil {
		return err
	}

	if err := validatePluginReferences(cfg); err != nil {
		return err
	}

	if err := validateBlastRadius(cfg); err != nil {
		return err
	}

	if err := validateSARIFLevels(cfg); err != nil {
		return err
	}

	if err := validateTopology(cfg); err != nil {
		return err
	}

	if err := validateIgnoreAttributes(cfg); err != nil {
		return err
	}

	return nil
}

// validatePrecedence checks that all precedence entries reference defined classifications
// and that no name appears more than once.
func validatePrecedence(cfg *Config, classificationNames map[string]bool) error {
	if len(cfg.Precedence) == 0 {
		return fmt.Errorf("precedence must not be empty")
	}

	seen := make(map[string]bool, len(cfg.Precedence))
	for _, name := range cfg.Precedence {
		if seen[name] {
			return fmt.Errorf("duplicate entry %q in precedence list", name)
		}
		seen[name] = true

		if !classificationNames[name] {
			return fmt.Errorf("precedence references undefined classification %q", name)
		}
	}

	return nil
}

// validateDefaults checks that default values reference valid classifications.
func validateDefaults(cfg *Config, classificationNames map[string]bool) error {
	if cfg.Defaults == nil {
		return fmt.Errorf("defaults block is required")
	}

	if cfg.Defaults.Unclassified != "" && !classificationNames[cfg.Defaults.Unclassified] {
		return fmt.Errorf("defaults.unclassified references undefined classification %q", cfg.Defaults.Unclassified)
	}

	if cfg.Defaults.NoChanges != "" && !classificationNames[cfg.Defaults.NoChanges] {
		return fmt.Errorf("defaults.no_changes references undefined classification %q", cfg.Defaults.NoChanges)
	}

	if cfg.Defaults.DriftClassification != "" && !classificationNames[cfg.Defaults.DriftClassification] {
		return fmt.Errorf("defaults.drift_classification references undefined classification %q", cfg.Defaults.DriftClassification)
	}

	return nil
}

// validateClassifications checks for duplicate classification names.
func validateClassifications(cfg *Config) error {
	seen := make(map[string]bool)
	for _, c := range cfg.Classifications {
		if seen[c.Name] {
			return fmt.Errorf("duplicate classification name %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// validActions is the set of recognized Terraform plan actions.
var validActions = map[string]bool{
	"create": true,
	"update": true,
	"delete": true,
	"read":   true,
	"no-op":  true,
}

// validateRules checks that each rule has at least resource or not_resource defined,
// and that actions/not_actions are valid and mutually exclusive.
func validateRules(cfg *Config) error {
	for _, classification := range cfg.Classifications {
		for i, rule := range classification.Rules {
			if len(rule.Resource) == 0 && len(rule.NotResource) == 0 {
				return fmt.Errorf("classification %q rule %d: rule must specify resource or not_resource",
					classification.Name, i+1)
			}

			if len(rule.Actions) > 0 && len(rule.NotActions) > 0 {
				return fmt.Errorf("classification %q rule %d: cannot combine actions and not_actions in the same rule",
					classification.Name, i+1)
			}

			if len(rule.Module) > 0 && len(rule.NotModule) > 0 {
				return fmt.Errorf("classification %q rule %d: cannot combine module and not_module in the same rule",
					classification.Name, i+1)
			}

			for _, a := range rule.NotActions {
				if !validActions[a] {
					return fmt.Errorf("classification %q rule %d: invalid not_actions value %q (valid: create, update, delete, read, no-op)",
						classification.Name, i+1, a)
				}
			}
		}
	}
	return nil
}

// validatePluginReferences checks that plugin blocks in classifications reference enabled plugins.
func validatePluginReferences(cfg *Config) error {
	// Build a map of enabled plugins
	enabledPlugins := make(map[string]bool)
	for _, p := range cfg.Plugins {
		if p.Enabled {
			enabledPlugins[p.Name] = true
		}
	}

	// Check each classification's plugin references
	for _, classification := range cfg.Classifications {
		for pluginName := range classification.PluginAnalyzerConfigs {
			if !enabledPlugins[pluginName] {
				return fmt.Errorf("classification %q references plugin %q which is not enabled",
					classification.Name, pluginName)
			}
		}
	}
	return nil
}

// validateBlastRadius checks that blast radius threshold values are positive integers.
func validateBlastRadius(cfg *Config) error {
	for _, c := range cfg.Classifications {
		if c.BlastRadius == nil {
			continue
		}
		br := c.BlastRadius
		if br.MaxDeletions != nil && *br.MaxDeletions <= 0 {
			return fmt.Errorf("classification %q: blast_radius.max_deletions must be a positive integer, got %d",
				c.Name, *br.MaxDeletions)
		}
		if br.MaxReplacements != nil && *br.MaxReplacements <= 0 {
			return fmt.Errorf("classification %q: blast_radius.max_replacements must be a positive integer, got %d",
				c.Name, *br.MaxReplacements)
		}
		if br.MaxChanges != nil && *br.MaxChanges <= 0 {
			return fmt.Errorf("classification %q: blast_radius.max_changes must be a positive integer, got %d",
				c.Name, *br.MaxChanges)
		}
	}
	return nil
}

// validateTopology checks that topology threshold values are positive integers.
func validateTopology(cfg *Config) error {
	for _, c := range cfg.Classifications {
		if c.Topology == nil {
			continue
		}
		topo := c.Topology
		if topo.MaxDownstream != nil && *topo.MaxDownstream <= 0 {
			return fmt.Errorf("classification %q: topology.max_downstream must be a positive integer, got %d",
				c.Name, *topo.MaxDownstream)
		}
		if topo.MaxPropagationDepth != nil && *topo.MaxPropagationDepth <= 0 {
			return fmt.Errorf("classification %q: topology.max_propagation_depth must be a positive integer, got %d",
				c.Name, *topo.MaxPropagationDepth)
		}
	}
	return nil
}

// validSARIFLevels is the set of recognized SARIF severity levels.
var validSARIFLevels = map[string]bool{
	"error":   true,
	"warning": true,
	"note":    true,
	"none":    true,
}

// validateSARIFLevels checks that sarif_level values on classification blocks are valid.
func validateSARIFLevels(cfg *Config) error {
	for _, c := range cfg.Classifications {
		if c.SARIFLevel != "" && !validSARIFLevels[c.SARIFLevel] {
			return fmt.Errorf("classification %q: invalid sarif_level %q (valid: error, warning, note, none)",
				c.Name, c.SARIFLevel)
		}
	}
	return nil
}

// validateIgnoreAttributes checks that ignore_attributes entries are non-empty
// strings and that scoped ignore_attribute blocks are well-formed (required
// description, non-empty attributes, unique names, compilable globs on resource,
// module, and attribute fields).
func validateIgnoreAttributes(cfg *Config) error {
	if cfg.Defaults == nil {
		return nil
	}
	for i, attr := range cfg.Defaults.IgnoreAttributes {
		if strings.TrimSpace(attr) == "" {
			return fmt.Errorf("defaults.ignore_attributes[%d]: entry must not be empty", i)
		}
	}

	seen := make(map[string]struct{}, len(cfg.Defaults.IgnoreAttributeRules))
	for i, rule := range cfg.Defaults.IgnoreAttributeRules {
		if rule.Name == "" {
			return fmt.Errorf("defaults.ignore_attribute[%d]: label is required", i)
		}
		if _, dup := seen[rule.Name]; dup {
			return fmt.Errorf("defaults.ignore_attribute[%d]: duplicate name %q", i, rule.Name)
		}
		seen[rule.Name] = struct{}{}

		if strings.TrimSpace(rule.Description) == "" {
			return fmt.Errorf("defaults.ignore_attribute %q: description must not be empty", rule.Name)
		}
		if len(rule.Attributes) == 0 {
			return fmt.Errorf("defaults.ignore_attribute %q: attributes must not be empty", rule.Name)
		}
		for j, attr := range rule.Attributes {
			if strings.TrimSpace(attr) == "" {
				return fmt.Errorf("defaults.ignore_attribute %q: attributes[%d] must not be empty", rule.Name, j)
			}
			for seg, segment := range strings.Split(attr, ".") {
				if _, err := glob.Compile(segment); err != nil {
					return fmt.Errorf("defaults.ignore_attribute %q: attributes[%d] segment %d %q: invalid glob: %w",
						rule.Name, j, seg, segment, err)
				}
			}
		}
		for j, pattern := range rule.Resource {
			if _, err := glob.Compile(pattern); err != nil {
				return fmt.Errorf("defaults.ignore_attribute %q: invalid resource pattern %q at index %d: %w",
					rule.Name, pattern, j, err)
			}
		}
		for j, pattern := range rule.NotResource {
			if _, err := glob.Compile(pattern); err != nil {
				return fmt.Errorf("defaults.ignore_attribute %q: invalid not_resource pattern %q at index %d: %w",
					rule.Name, pattern, j, err)
			}
		}
		for j, pattern := range rule.Module {
			if _, err := glob.Compile(pattern, '.'); err != nil {
				return fmt.Errorf("defaults.ignore_attribute %q: invalid module pattern %q at index %d: %w",
					rule.Name, pattern, j, err)
			}
		}
		for j, pattern := range rule.NotModule {
			if _, err := glob.Compile(pattern, '.'); err != nil {
				return fmt.Errorf("defaults.ignore_attribute %q: invalid not_module pattern %q at index %d: %w",
					rule.Name, pattern, j, err)
			}
		}
	}
	return nil
}

// WarnRedundantNotResource emits a warning to w when a not_resource rule
// contains only patterns that are already present in higher-precedence
// resource rules. In such cases, using resource = ["*"] is simpler and
// less error-prone because the precedence-ordered evaluation already
// ensures higher-priority classifications match first.
//
// This function is intended to be called with verbose mode enabled.
func WarnRedundantNotResource(cfg *Config, w io.Writer) {
	// Build a map of classifications by name for quick lookup
	classificationByName := make(map[string]*ClassificationConfig)
	for i := range cfg.Classifications {
		classificationByName[cfg.Classifications[i].Name] = &cfg.Classifications[i]
	}

	// Collect resource patterns from each classification in precedence order
	higherPatterns := make(map[string]bool)

	for _, classificationName := range cfg.Precedence {
		classification, ok := classificationByName[classificationName]
		if !ok {
			continue
		}

		for ruleIdx, rule := range classification.Rules {
			// Check if this not_resource list is fully covered by higher-precedence patterns
			if len(rule.NotResource) > 0 && allPatternsKnown(rule.NotResource, higherPatterns) {
				_, _ = fmt.Fprintf(w, "Warning: classification %q rule %d uses not_resource with patterns "+
					"already covered by higher-precedence rules. Consider using resource = [\"*\"] instead.\n",
					classificationName, ruleIdx+1)
			}

			// Accumulate resource patterns for lower-precedence checks
			for _, pattern := range rule.Resource {
				higherPatterns[pattern] = true
			}
		}
	}
}

// allPatternsKnown returns true if every pattern in patterns exists in known.
func allPatternsKnown(patterns []string, known map[string]bool) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if !known[p] {
			return false
		}
	}
	return true
}

// ValidateGlobPatterns checks that all glob patterns in resource and not_resource
// fields compile successfully. Returns an error on the first malformed pattern.
func ValidateGlobPatterns(cfg *Config) error {
	for _, classification := range cfg.Classifications {
		for i, rule := range classification.Rules {
			for _, pattern := range rule.Resource {
				if _, err := glob.Compile(pattern); err != nil {
					return fmt.Errorf("classification %q rule %d: invalid resource pattern %q: %w",
						classification.Name, i+1, pattern, err)
				}
			}
			for _, pattern := range rule.NotResource {
				if _, err := glob.Compile(pattern); err != nil {
					return fmt.Errorf("classification %q rule %d: invalid not_resource pattern %q: %w",
						classification.Name, i+1, pattern, err)
				}
			}
			for _, pattern := range rule.Module {
				if _, err := glob.Compile(pattern, '.'); err != nil {
					return fmt.Errorf("classification %q rule %d: invalid module pattern %q: %w",
						classification.Name, i+1, pattern, err)
				}
			}
			for _, pattern := range rule.NotModule {
				if _, err := glob.Compile(pattern, '.'); err != nil {
					return fmt.Errorf("classification %q rule %d: invalid not_module pattern %q: %w",
						classification.Name, i+1, pattern, err)
				}
			}
		}
	}
	return nil
}

// Warning represents a validation warning with context.
type Warning struct {
	Classification string
	Message        string
}

func (w Warning) String() string {
	if w.Classification != "" {
		return fmt.Sprintf("Warning: classification %q: %s", w.Classification, w.Message)
	}
	return fmt.Sprintf("Warning: %s", w.Message)
}

// ValidateWarnings runs warning-level checks on a valid configuration.
// It returns warnings for: unreachable rules (catch-all shadows), empty
// classifications, and missing plugin binaries.
func ValidateWarnings(cfg *Config) []Warning {
	var warnings []Warning
	warnings = append(warnings, warnUnreachableRules(cfg)...)
	warnings = append(warnings, warnEmptyClassifications(cfg)...)
	warnings = append(warnings, warnMissingPluginBinaries(cfg)...)
	warnings = append(warnings, warnRedundantNotResource(cfg)...)
	warnings = append(warnings, warnModuleGlobOverlap(cfg)...)
	return warnings
}

// warnUnreachableRules detects when a higher-precedence classification has a
// resource=["*"] rule with no action constraint, making all lower-precedence
// classification rules unreachable.
func warnUnreachableRules(cfg *Config) []Warning {
	var warnings []Warning

	// Build classification lookup
	classificationByName := make(map[string]*ClassificationConfig)
	for i := range cfg.Classifications {
		classificationByName[cfg.Classifications[i].Name] = &cfg.Classifications[i]
	}

	// Walk precedence list; if we find a catch-all (resource=["*"], no actions),
	// everything after it is unreachable.
	catchAllFound := ""
	for _, name := range cfg.Precedence {
		c, ok := classificationByName[name]
		if !ok {
			continue
		}

		if catchAllFound != "" {
			// This classification is shadowed
			if len(c.Rules) > 0 {
				warnings = append(warnings, Warning{
					Classification: name,
					Message: fmt.Sprintf("rules are unreachable because classification %q has a catch-all rule (resource=[\"*\"] with no action constraint) at higher precedence",
						catchAllFound),
				})
			}
			continue
		}

		// Check if this classification has a catch-all rule
		for _, rule := range c.Rules {
			if isCatchAllRule(rule) {
				catchAllFound = name
				break
			}
		}
	}

	return warnings
}

// isCatchAllRule returns true if the rule matches all resources with no action constraint.
func isCatchAllRule(rule RuleConfig) bool {
	if len(rule.Actions) > 0 || len(rule.NotActions) > 0 {
		return false
	}
	for _, pattern := range rule.Resource {
		if pattern == "*" {
			return true
		}
	}
	return false
}

// warnEmptyClassifications warns when a classification has no rules and no plugin
// analyzer blocks, meaning it can never match anything. The classification
// referenced by defaults.no_changes is exempt — CR-0036 routes no-op resources
// to it via short-circuit, so it does not need rules.
func warnEmptyClassifications(cfg *Config) []Warning {
	var noChanges string
	if cfg.Defaults != nil {
		noChanges = cfg.Defaults.NoChanges
	}

	var warnings []Warning
	for _, c := range cfg.Classifications {
		if c.Name == noChanges {
			continue
		}
		if len(c.Rules) == 0 && !hasPluginAnalyzers(c) && c.BlastRadius == nil && c.Topology == nil {
			warnings = append(warnings, Warning{
				Classification: c.Name,
				Message:        "has no rules and no plugin analyzer blocks; it will never match any resource",
			})
		}
	}
	return warnings
}

// hasPluginAnalyzers returns true if the classification has any plugin analyzer config.
func hasPluginAnalyzers(c ClassificationConfig) bool {
	for _, pac := range c.PluginAnalyzerConfigs {
		if pac != nil && pac.PrivilegeEscalation != nil {
			return true
		}
	}
	return false
}

// pluginSearchPaths returns the ordered list of directories to search for plugin binaries.
func pluginSearchPaths() []string {
	var paths []string
	if envDir := os.Getenv("TFCLASSIFY_PLUGIN_DIR"); envDir != "" {
		paths = append(paths, envDir)
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".tfclassify", "plugins"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".tfclassify", "plugins"))
	}
	return paths
}

// warnMissingPluginBinaries checks that enabled plugins have their binary
// available in the plugin search path.
func warnMissingPluginBinaries(cfg *Config) []Warning {
	var warnings []Warning
	paths := pluginSearchPaths()

	for _, p := range cfg.Plugins {
		if !p.Enabled || p.Source == "" {
			continue
		}

		binaryName := "tfclassify-plugin-" + p.Name
		found := false
		for _, dir := range paths {
			candidate := filepath.Join(dir, binaryName)
			if _, err := os.Stat(candidate); err == nil {
				found = true
				break
			}
		}

		if !found {
			warnings = append(warnings, Warning{
				Message: fmt.Sprintf("plugin %q is enabled but binary %q was not found in any search path; run \"tfclassify init\" to install it",
					p.Name, binaryName),
			})
		}
	}

	return warnings
}

// warnRedundantNotResource checks for not_resource rules that are fully covered
// by higher-precedence resource patterns.
func warnRedundantNotResource(cfg *Config) []Warning {
	var warnings []Warning

	classificationByName := make(map[string]*ClassificationConfig)
	for i := range cfg.Classifications {
		classificationByName[cfg.Classifications[i].Name] = &cfg.Classifications[i]
	}

	higherPatterns := make(map[string]bool)
	for _, classificationName := range cfg.Precedence {
		classification, ok := classificationByName[classificationName]
		if !ok {
			continue
		}
		for ruleIdx, rule := range classification.Rules {
			if len(rule.NotResource) > 0 && allPatternsKnown(rule.NotResource, higherPatterns) {
				warnings = append(warnings, Warning{
					Classification: classificationName,
					Message: fmt.Sprintf("rule %d uses not_resource with patterns already covered by higher-precedence rules; consider using resource = [\"*\"] instead",
						ruleIdx+1),
				})
			}
			for _, pattern := range rule.Resource {
				higherPatterns[pattern] = true
			}
		}
	}

	return warnings
}

// warnModuleGlobOverlap warns when two rules in different classifications have
// overlapping module globs. The author likely meant the patterns to be
// disjoint; without that, the rule whose classification appears earlier in
// `precedence` silently wins for the overlapping addresses.
//
// The classic case is a prefix-greedy `*` that consumes more than the author
// intended — e.g. `**.foo*` matches both `module.foo["k"]` and
// `module.foo_bar["k"]`. If `**.foo_bar*` is a separate rule in another
// classification, both rules match `module.foo_bar["k"]` and the higher-
// precedence one wins.
func warnModuleGlobOverlap(cfg *Config) []Warning {
	type rulePos struct {
		classification string
		ruleIdx        int
		ruleDesc       string
		pattern        string
	}

	// Collect every (classification, rule, pattern) triple, skipping rules
	// without module patterns. Skip any pattern that is just "*" or "**"
	// since those are already covered by warnUnreachableRules.
	var triples []rulePos
	for _, c := range cfg.Classifications {
		for i, rule := range c.Rules {
			for _, p := range rule.Module {
				if isTrivialModulePattern(p) {
					continue
				}
				triples = append(triples, rulePos{
					classification: c.Name,
					ruleIdx:        i + 1,
					ruleDesc:       rule.Description,
					pattern:        p,
				})
			}
		}
	}

	// Walk every cross-classification pattern pair and emit a warning when
	// they overlap. Use a sorted (a,b) key per emitted pair so the same
	// overlap is not reported twice.
	emitted := make(map[string]bool)
	var warnings []Warning
	for i := 0; i < len(triples); i++ {
		for j := i + 1; j < len(triples); j++ {
			a, b := triples[i], triples[j]
			if a.classification == b.classification {
				continue
			}
			sample, ok := patternsOverlap(a.pattern, b.pattern)
			if !ok {
				continue
			}
			key := overlapKey(a.classification, a.ruleIdx, a.pattern, b.classification, b.ruleIdx, b.pattern)
			if emitted[key] {
				continue
			}
			emitted[key] = true
			warnings = append(warnings, Warning{
				Message: fmt.Sprintf(
					"module glob overlap: classification %q rule %d (%s) and classification %q rule %d (%s) both match %q; the classification listed first in precedence will win",
					a.classification, a.ruleIdx, describePattern(a),
					b.classification, b.ruleIdx, describePattern(b),
					sample,
				),
			})
		}
	}
	return warnings
}

// describePattern returns either the rule's user-provided description (if any)
// or the raw module pattern, used to help the author find the overlap source.
func describePattern(r struct {
	classification string
	ruleIdx        int
	ruleDesc       string
	pattern        string
}) string {
	if r.ruleDesc != "" {
		return fmt.Sprintf("%q, module=%q", r.ruleDesc, r.pattern)
	}
	return fmt.Sprintf("module=%q", r.pattern)
}

// overlapKey returns a stable key for an unordered pair of (classification,
// rule, pattern) tuples so the same overlap warning is not emitted twice.
func overlapKey(ac string, ai int, ap, bc string, bi int, bp string) string {
	left := fmt.Sprintf("%s|%d|%s", ac, ai, ap)
	right := fmt.Sprintf("%s|%d|%s", bc, bi, bp)
	if left < right {
		return left + "<>" + right
	}
	return right + "<>" + left
}

// isTrivialModulePattern reports whether a module pattern is a catch-all
// (matches every address). Trivial patterns are excluded from overlap
// detection since they overlap with everything by definition; the
// unreachable-rule warning covers that case.
func isTrivialModulePattern(p string) bool {
	switch p {
	case "*", "**", "**.*", "*.**":
		return true
	}
	return false
}

// patternsOverlap reports whether two module globs share at least one
// matching address. When they do, it also returns a sample address that
// matches both, suitable for surfacing in a warning. Both patterns are
// compiled with '.' as the path separator (matching how the matcher
// compiles module patterns).
//
// The check is approximate: it concretizes each pattern by substituting
// wildcards with placeholder characters and tests whether the other
// pattern matches that concrete form. If either direction matches,
// overlap is confirmed. Misses are possible for exotic patterns
// (e.g. character classes that exclude the placeholder) but the common
// shapes used in real configs (`**.foo`, `**.foo*`, `**.foo.*`) are
// caught reliably.
func patternsOverlap(patternA, patternB string) (string, bool) {
	gA, err := glob.Compile(patternA, '.')
	if err != nil {
		return "", false
	}
	gB, err := glob.Compile(patternB, '.')
	if err != nil {
		return "", false
	}

	candA := concretizeModulePattern(patternA)
	if candA != "" && gB.Match(candA) {
		return candA, true
	}
	candB := concretizeModulePattern(patternB)
	if candB != "" && gA.Match(candB) {
		return candB, true
	}
	return "", false
}

// concretizeModulePattern replaces glob wildcards in a module pattern with
// literal placeholder characters, producing a sample address that the
// pattern would match. Used to probe whether another pattern also matches
// that address (overlap detection). Honors backslash escapes and skips
// over character classes / brace alternations.
func concretizeModulePattern(pattern string) string {
	const placeholder = "x"
	var b strings.Builder
	for i := 0; i < len(pattern); {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			// Escaped char — emit the next byte as a literal.
			b.WriteByte(pattern[i+1])
			i += 2
		case c == '*':
			// `*` and `**` both collapse to a single placeholder char so the
			// resulting string contains no separator characters from the wildcard
			// itself. The rest of the pattern provides the structure.
			b.WriteString(placeholder)
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
			} else {
				i++
			}
		case c == '?':
			b.WriteString(placeholder)
			i++
		case c == '[':
			// Character class: emit a placeholder and skip past the matching `]`.
			j := i + 1
			for j < len(pattern) && pattern[j] != ']' {
				if pattern[j] == '\\' && j+1 < len(pattern) {
					j += 2
				} else {
					j++
				}
			}
			b.WriteString(placeholder)
			if j < len(pattern) {
				i = j + 1
			} else {
				i = len(pattern)
			}
		case c == '{':
			// Brace alternation: emit a placeholder and skip past the matching `}`.
			j := i + 1
			for j < len(pattern) && pattern[j] != '}' {
				if pattern[j] == '\\' && j+1 < len(pattern) {
					j += 2
				} else {
					j++
				}
			}
			b.WriteString(placeholder)
			if j < len(pattern) {
				i = j + 1
			} else {
				i = len(pattern)
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
