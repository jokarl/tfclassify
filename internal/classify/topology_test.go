package classify

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// exprWithRefs creates a tfjson.Expression with the given references.
func exprWithRefs(refs ...string) *tfjson.Expression {
	return &tfjson.Expression{
		ExpressionData: &tfjson.ExpressionData{
			References: refs,
		},
	}
}

func TestBuildDependencyGraph_NilConfig(t *testing.T) {
	g := BuildDependencyGraph(nil)
	if len(g.downstream) != 0 {
		t.Errorf("expected empty graph for nil config, got %d entries", len(g.downstream))
	}
}

func TestBuildDependencyGraph_EmptyModule(t *testing.T) {
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{},
	}
	g := BuildDependencyGraph(cfg)
	if len(g.downstream) != 0 {
		t.Errorf("expected empty graph for empty module, got %d entries", len(g.downstream))
	}
}

func TestBuildDependencyGraph_SimpleReference(t *testing.T) {
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{
					Address: "azurerm_resource_group.main",
					Type:    "azurerm_resource_group",
				},
				{
					Address: "azurerm_virtual_network.main",
					Type:    "azurerm_virtual_network",
					Expressions: map[string]*tfjson.Expression{
						"resource_group_name": exprWithRefs("azurerm_resource_group.main.name"),
					},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	// azurerm_resource_group.main → azurerm_virtual_network.main
	downstream := g.Downstream("azurerm_resource_group.main")
	if len(downstream) != 1 {
		t.Fatalf("expected 1 downstream for resource_group, got %d", len(downstream))
	}
	if downstream[0] != "azurerm_virtual_network.main" {
		t.Errorf("expected downstream azurerm_virtual_network.main, got %s", downstream[0])
	}
}

func TestBuildDependencyGraph_ChainedDependencies(t *testing.T) {
	// A → B → C
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{Address: "a.one", Type: "a"},
				{
					Address: "b.one",
					Type:    "b",
					Expressions: map[string]*tfjson.Expression{
						"ref": exprWithRefs("a.one.id"),
					},
				},
				{
					Address: "c.one",
					Type:    "c",
					Expressions: map[string]*tfjson.Expression{
						"ref": exprWithRefs("b.one.id"),
					},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	count, depth := g.computeDownstreamFanOut("a.one")
	if count != 2 {
		t.Errorf("expected 2 downstream from a.one, got %d", count)
	}
	if depth != 2 {
		t.Errorf("expected depth 2 from a.one, got %d", depth)
	}
}

func TestBuildDependencyGraph_FanOut(t *testing.T) {
	// A → B, A → C, A → D
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{Address: "a.one", Type: "a"},
				{
					Address:     "b.one",
					Type:        "b",
					Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")},
				},
				{
					Address:     "c.one",
					Type:        "c",
					Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")},
				},
				{
					Address:     "d.one",
					Type:        "d",
					Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	count, depth := g.computeDownstreamFanOut("a.one")
	if count != 3 {
		t.Errorf("expected 3 downstream from a.one, got %d", count)
	}
	if depth != 1 {
		t.Errorf("expected depth 1 from a.one, got %d", depth)
	}
}

func TestBuildDependencyGraph_CircularGuard(t *testing.T) {
	// A → B → A (shouldn't happen in valid TF, but guard against infinite loop)
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{
					Address:     "a.one",
					Type:        "a",
					Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("b.one.id")},
				},
				{
					Address:     "b.one",
					Type:        "b",
					Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	// Should not hang; visited set prevents infinite loop
	count, depth := g.computeDownstreamFanOut("a.one")
	if count != 1 {
		t.Errorf("expected 1 downstream (b.one) from a.one in circular, got %d", count)
	}
	if depth != 1 {
		t.Errorf("expected depth 1 from a.one in circular, got %d", depth)
	}
}

func TestBuildDependencyGraph_SkipsNonResourceRefs(t *testing.T) {
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{
					Address: "a.one",
					Type:    "a",
					Expressions: map[string]*tfjson.Expression{
						"ref": exprWithRefs(
							"var.name",
							"local.value",
							"data.something.id",
							"each.key",
							"count.index",
						),
					},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	downstream := g.Downstream("a.one")
	if len(downstream) != 0 {
		t.Errorf("expected 0 downstream (non-resource refs filtered), got %d: %v", len(downstream), downstream)
	}
}

func TestBuildDependencyGraph_ModuleCalls(t *testing.T) {
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{Address: "a.root", Type: "a"},
			},
			ModuleCalls: map[string]*tfjson.ModuleCall{
				"network": {
					Module: &tfjson.ConfigModule{
						Resources: []*tfjson.ConfigResource{
							{Address: "b.child", Type: "b"},
							{
								Address: "c.child",
								Type:    "c",
								Expressions: map[string]*tfjson.Expression{
									"ref": exprWithRefs("b.child.id"),
								},
							},
						},
					},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	// b.child within module.network should have qualified address
	downstream := g.Downstream("module.network.b.child")
	if len(downstream) != 1 {
		t.Fatalf("expected 1 downstream for module.network.b.child, got %d", len(downstream))
	}
	if downstream[0] != "module.network.c.child" {
		t.Errorf("expected downstream module.network.c.child, got %s", downstream[0])
	}
}

func TestTopologyAnalyzer_NilConfig(t *testing.T) {
	maxDown := 5
	a := NewTopologyAnalyzer([]config.ClassificationConfig{
		{Name: "critical", Topology: &config.TopologyConfig{MaxDownstream: &maxDown}},
	})

	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"update"}},
		},
		Config: nil,
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions when Config is nil, got %d", len(decisions))
	}
}

func TestTopologyAnalyzer_NoThresholds(t *testing.T) {
	a := NewTopologyAnalyzer(nil)
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"update"}},
		},
	}
	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions with no thresholds, got %d", len(decisions))
	}
}

func TestTopologyAnalyzer_MaxDownstreamExceeded(t *testing.T) {
	maxDown := 2
	a := NewTopologyAnalyzer([]config.ClassificationConfig{
		{Name: "critical", Topology: &config.TopologyConfig{MaxDownstream: &maxDown}},
	})

	// a.one → b.one, c.one, d.one (fan-out of 3 > threshold of 2)
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"update"}},
			{Address: "b.one", Type: "b", Actions: []string{"no-op"}},
		},
		Config: &tfjson.Config{
			RootModule: &tfjson.ConfigModule{
				Resources: []*tfjson.ConfigResource{
					{Address: "a.one", Type: "a"},
					{Address: "b.one", Type: "b", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
					{Address: "c.one", Type: "c", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
					{Address: "d.one", Type: "d", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
				},
			},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision for a.one exceeding threshold, got %d", len(decisions))
	}
	if decisions[0].Address != "a.one" {
		t.Errorf("expected decision for a.one, got %s", decisions[0].Address)
	}
	if decisions[0].Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decisions[0].Classification)
	}
}

func TestTopologyAnalyzer_MaxDepthExceeded(t *testing.T) {
	maxDepth := 1
	a := NewTopologyAnalyzer([]config.ClassificationConfig{
		{Name: "critical", Topology: &config.TopologyConfig{MaxPropagationDepth: &maxDepth}},
	})

	// a.one → b.one → c.one (depth 2 > threshold of 1)
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"update"}},
		},
		Config: &tfjson.Config{
			RootModule: &tfjson.ConfigModule{
				Resources: []*tfjson.ConfigResource{
					{Address: "a.one", Type: "a"},
					{Address: "b.one", Type: "b", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
					{Address: "c.one", Type: "c", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("b.one.id")}},
				},
			},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision for a.one exceeding depth, got %d", len(decisions))
	}
}

func TestTopologyAnalyzer_BelowThreshold(t *testing.T) {
	maxDown := 10
	a := NewTopologyAnalyzer([]config.ClassificationConfig{
		{Name: "critical", Topology: &config.TopologyConfig{MaxDownstream: &maxDown}},
	})

	// a.one → b.one (fan-out of 1, well below 10)
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"update"}},
		},
		Config: &tfjson.Config{
			RootModule: &tfjson.ConfigModule{
				Resources: []*tfjson.ConfigResource{
					{Address: "a.one", Type: "a"},
					{Address: "b.one", Type: "b", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
				},
			},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions when below threshold, got %d", len(decisions))
	}
}

func TestTopologyAnalyzer_SkipsNoOp(t *testing.T) {
	maxDown := 0
	a := NewTopologyAnalyzer([]config.ClassificationConfig{
		{Name: "critical", Topology: &config.TopologyConfig{MaxDownstream: &maxDown}},
	})

	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "a.one", Type: "a", Actions: []string{"no-op"}},
		},
		Config: &tfjson.Config{
			RootModule: &tfjson.ConfigModule{
				Resources: []*tfjson.ConfigResource{
					{Address: "a.one", Type: "a"},
					{Address: "b.one", Type: "b", Expressions: map[string]*tfjson.Expression{"ref": exprWithRefs("a.one.id")}},
				},
			},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions for no-op resources, got %d", len(decisions))
	}
}

func TestTopologyAnalyzer_Name(t *testing.T) {
	a := &TopologyAnalyzer{}
	if a.Name() != "topology" {
		t.Errorf("expected name 'topology', got %q", a.Name())
	}
}

func TestResolveReference(t *testing.T) {
	tests := []struct {
		prefix string
		ref    string
		want   string
	}{
		{"", "azurerm_resource_group.main.name", "azurerm_resource_group.main"},
		{"", "azurerm_resource_group.main", "azurerm_resource_group.main"},
		{"module.net", "azurerm_vnet.main.id", "module.net.azurerm_vnet.main"},
		{"", "var.name", ""},
		{"", "local.value", ""},
		{"", "data.something.id", ""},
		{"", "module.net.output", ""},
		{"", "each.key", ""},
		{"", "count.index", ""},
		{"", "x", ""},
	}

	for _, tt := range tests {
		got := resolveReference(tt.prefix, tt.ref)
		if got != tt.want {
			t.Errorf("resolveReference(%q, %q) = %q, want %q", tt.prefix, tt.ref, got, tt.want)
		}
	}
}

func TestBuildDependencyGraph_NestedExpressions(t *testing.T) {
	cfg := &tfjson.Config{
		RootModule: &tfjson.ConfigModule{
			Resources: []*tfjson.ConfigResource{
				{Address: "a.one", Type: "a"},
				{
					Address: "b.one",
					Type:    "b",
					Expressions: map[string]*tfjson.Expression{
						"block": {
							ExpressionData: &tfjson.ExpressionData{
								NestedBlocks: []map[string]*tfjson.Expression{
									{
										"inner": exprWithRefs("a.one.id"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	g := BuildDependencyGraph(cfg)

	downstream := g.Downstream("a.one")
	if len(downstream) != 1 || downstream[0] != "b.one" {
		t.Errorf("expected a.one → b.one from nested expression, got %v", downstream)
	}
}
