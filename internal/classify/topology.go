package classify

import (
	"fmt"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// DependencyGraph represents the directed dependency graph between resources.
// An edge from A to B means "B depends on A" (a change to A propagates to B).
type DependencyGraph struct {
	// downstream maps resource address → list of downstream dependents
	downstream map[string][]string
}

// BuildDependencyGraph walks the Terraform config to build a resource dependency graph.
// References like "azurerm_network_security_group.main.id" are parsed to extract the
// source resource address, and an edge A → B is created (A's change propagates to B).
func BuildDependencyGraph(cfg *tfjson.Config) *DependencyGraph {
	g := &DependencyGraph{downstream: make(map[string][]string)}
	if cfg == nil || cfg.RootModule == nil {
		return g
	}
	g.walkModule(cfg.RootModule, "")
	return g
}

// walkModule recursively walks a module config and its child module calls.
func (g *DependencyGraph) walkModule(mod *tfjson.ConfigModule, modulePrefix string) {
	if mod == nil {
		return
	}

	for _, res := range mod.Resources {
		targetAddr := qualifyAddress(modulePrefix, res.Address)

		// Ensure the node exists in the graph
		if _, ok := g.downstream[targetAddr]; !ok {
			g.downstream[targetAddr] = nil
		}

		// Extract references from expressions
		refs := extractReferences(res.Expressions)
		for _, ref := range refs {
			sourceAddr := resolveReference(modulePrefix, ref)
			if sourceAddr == "" || sourceAddr == targetAddr {
				continue
			}
			g.downstream[sourceAddr] = append(g.downstream[sourceAddr], targetAddr)
		}
	}

	// Recurse into child module calls
	for callName, call := range mod.ModuleCalls {
		childPrefix := qualifyModulePrefix(modulePrefix, callName)
		if call.Module != nil {
			g.walkModule(call.Module, childPrefix)
		}
	}
}

// qualifyAddress prepends the module prefix to a resource address.
func qualifyAddress(modulePrefix, address string) string {
	if modulePrefix == "" {
		return address
	}
	return modulePrefix + "." + address
}

// qualifyModulePrefix builds a nested module prefix.
func qualifyModulePrefix(parent, childName string) string {
	child := "module." + childName
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// resolveReference parses a Terraform reference string and resolves it to a resource address.
// References like "azurerm_network_security_group.main.id" resolve to
// "azurerm_network_security_group.main". Module references like
// "module.network.azurerm_vnet.main" are handled by qualifying with the module prefix.
func resolveReference(modulePrefix, ref string) string {
	// Skip non-resource references
	if strings.HasPrefix(ref, "var.") ||
		strings.HasPrefix(ref, "local.") ||
		strings.HasPrefix(ref, "data.") ||
		strings.HasPrefix(ref, "each.") ||
		strings.HasPrefix(ref, "count.") ||
		strings.HasPrefix(ref, "terraform.") ||
		strings.HasPrefix(ref, "path.") ||
		strings.HasPrefix(ref, "null_resource.") {
		return ""
	}

	// Handle module references: "module.name.output"
	if strings.HasPrefix(ref, "module.") {
		// Module output references don't map to a single resource
		return ""
	}

	// Standard resource reference: "resource_type.name" or "resource_type.name.attribute"
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		return ""
	}

	// The resource address is the first two parts: type.name
	resourceAddr := parts[0] + "." + parts[1]
	return qualifyAddress(modulePrefix, resourceAddr)
}

// extractReferences collects all reference strings from a resource's expressions.
func extractReferences(expressions map[string]*tfjson.Expression) []string {
	if expressions == nil {
		return nil
	}

	var refs []string
	for _, expr := range expressions {
		refs = append(refs, collectExprRefs(expr)...)
	}
	return refs
}

// collectExprRefs recursively collects references from an expression and its nested expressions.
func collectExprRefs(expr *tfjson.Expression) []string {
	if expr == nil {
		return nil
	}
	var refs []string
	if expr.ExpressionData == nil {
		return nil
	}
	refs = append(refs, expr.References...)
	for _, nested := range expr.NestedBlocks {
		for _, nestedExpr := range nested {
			refs = append(refs, collectExprRefs(nestedExpr)...)
		}
	}
	return refs
}

// Downstream returns the list of downstream dependents for a resource.
func (g *DependencyGraph) Downstream(address string) []string {
	return g.downstream[address]
}

// computeDownstreamFanOut computes the total number of downstream resources and max depth
// reachable from a starting resource via BFS.
func (g *DependencyGraph) computeDownstreamFanOut(address string) (count int, maxDepth int) {
	visited := make(map[string]bool)
	visited[address] = true

	type queueItem struct {
		addr  string
		depth int
	}

	queue := []queueItem{{addr: address, depth: 0}}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		for _, dep := range g.downstream[item.addr] {
			if visited[dep] {
				continue
			}
			visited[dep] = true
			count++
			depth := item.depth + 1
			if depth > maxDepth {
				maxDepth = depth
			}
			queue = append(queue, queueItem{addr: dep, depth: depth})
		}
	}

	return count, maxDepth
}

// TopologyAnalyzer inspects the Terraform dependency graph and flags resources
// whose change propagation exceeds configured thresholds.
type TopologyAnalyzer struct {
	thresholds map[string]*config.TopologyConfig
}

// NewTopologyAnalyzer creates a TopologyAnalyzer from the classification configs.
func NewTopologyAnalyzer(classifications []config.ClassificationConfig) *TopologyAnalyzer {
	thresholds := make(map[string]*config.TopologyConfig)
	for _, c := range classifications {
		if c.Topology != nil {
			thresholds[c.Name] = c.Topology
		}
	}
	return &TopologyAnalyzer{thresholds: thresholds}
}

// Thresholds returns the thresholds map for checking if the analyzer has any config.
func (a *TopologyAnalyzer) Thresholds() map[string]*config.TopologyConfig {
	return a.thresholds
}

// Name returns the analyzer name.
func (a *TopologyAnalyzer) Name() string {
	return "topology"
}

// AnalyzePlan builds the dependency graph from the plan config and checks
// each changed resource's downstream fan-out against configured thresholds.
func (a *TopologyAnalyzer) AnalyzePlan(result *plan.ParseResult) []ResourceDecision {
	if len(a.thresholds) == 0 {
		return nil
	}
	if result.Config == nil {
		return nil
	}

	graph := BuildDependencyGraph(result.Config)

	// Build set of changed resource addresses
	changedAddrs := make(map[string]bool, len(result.Changes))
	for _, change := range result.Changes {
		if !isNoOp(change.Actions) {
			changedAddrs[change.Address] = true
		}
	}

	var decisions []ResourceDecision
	for classificationName, topo := range a.thresholds {
		for _, change := range result.Changes {
			if isNoOp(change.Actions) {
				continue
			}

			count, depth := graph.computeDownstreamFanOut(change.Address)

			var reasons []string
			if topo.MaxDownstream != nil && count > *topo.MaxDownstream {
				reasons = append(reasons, fmt.Sprintf(
					"builtin: topology - Resource %s change propagates to %d downstream resources (threshold: %d)",
					change.Address, count, *topo.MaxDownstream))
			}
			if topo.MaxPropagationDepth != nil && depth > *topo.MaxPropagationDepth {
				reasons = append(reasons, fmt.Sprintf(
					"builtin: topology - Resource %s change cascades %d levels deep (threshold: %d)",
					change.Address, depth, *topo.MaxPropagationDepth))
			}

			if len(reasons) > 0 {
				decisions = append(decisions, ResourceDecision{
					Address:        change.Address,
					ResourceType:   change.Type,
					Actions:        change.Actions,
					Classification: classificationName,
					MatchedRules:   reasons,
				})
			}
		}
	}

	return decisions
}
