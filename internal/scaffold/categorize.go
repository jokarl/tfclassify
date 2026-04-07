package scaffold

import (
	"sort"
	"strings"
)

// Category represents a semantic grouping of resource types.
type Category struct {
	Name        string   // e.g., "IAM", "Networking", "Security"
	Description string   // Human-readable explanation for HCL comments
	Types       []string // Actual matched resource types
	Patterns    []string // Generated glob patterns for rules
}

// CategorizeResult holds grouped resource types organized by risk level.
type CategorizeResult struct {
	Critical      []Category // High-risk categories
	Standard      []Category // Moderate-risk categories
	Uncategorized []string   // Types not matching any known category
	Modules       []string   // Module paths found in state
}

// categoryDef defines a resource type category in the knowledge base.
type categoryDef struct {
	name        string
	description string
	risk        string // "critical" or "standard"
	keywords    []string
	exact       bool // if true, match whole suffix (not substring)
}

// providerKnowledge maps provider prefixes to their category definitions.
var providerKnowledge = map[string][]categoryDef{
	"azurerm": {
		{
			name:        "IAM",
			description: "Identity and access management changes affect who can access your infrastructure",
			risk:        "critical",
			keywords:    []string{"role_assignment", "role_definition", "user_assigned_identity"},
		},
		{
			name:        "Security",
			description: "Security resource changes affect data protection and access controls",
			risk:        "critical",
			keywords:    []string{"key_vault", "firewall", "application_security_group"},
			exact:       true,
		},
		{
			name:        "Network ACLs",
			description: "Network security rules control traffic flow and can expose services",
			risk:        "critical",
			keywords:    []string{"network_security_group", "network_security_rule"},
		},
		{
			name:        "Networking",
			description: "Network infrastructure changes",
			risk:        "standard",
			keywords:    []string{"virtual_network", "subnet", "public_ip", "route_table", "nat_gateway", "private_endpoint", "private_dns", "lb", "application_gateway"},
		},
		{
			name:        "Compute",
			description: "Compute resource changes",
			risk:        "standard",
			keywords:    []string{"virtual_machine", "container_group", "kubernetes_cluster", "function_app", "app_service", "linux_", "windows_"},
		},
		{
			name:        "Storage",
			description: "Storage resource changes",
			risk:        "standard",
			keywords:    []string{"storage_account", "storage_container", "storage_blob", "storage_share"},
		},
		{
			name:        "Database",
			description: "Database resource changes",
			risk:        "standard",
			keywords:    []string{"cosmosdb", "sql_server", "sql_database", "postgresql", "mysql", "mssql"},
		},
		{
			name:        "Monitoring",
			description: "Observability and monitoring changes",
			risk:        "standard",
			keywords:    []string{"monitor_", "log_analytics", "diagnostic"},
		},
	},
	"aws": {
		{
			name:        "IAM",
			description: "Identity and access management changes affect who can access your infrastructure",
			risk:        "critical",
			keywords:    []string{"iam_role", "iam_policy", "iam_user", "iam_group", "iam_access_key"},
		},
		{
			name:        "Security",
			description: "Security resource changes affect data protection and encryption",
			risk:        "critical",
			keywords:    []string{"kms_key", "kms_alias", "secretsmanager"},
		},
		{
			name:        "Network ACLs",
			description: "Network access controls affect traffic flow and can expose services",
			risk:        "critical",
			keywords:    []string{"security_group", "network_acl"},
		},
		{
			name:        "Networking",
			description: "Network infrastructure changes",
			risk:        "standard",
			keywords:    []string{"vpc", "subnet", "route_table", "nat_gateway", "internet_gateway", "eip", "lb"},
		},
		{
			name:        "Compute",
			description: "Compute resource changes",
			risk:        "standard",
			keywords:    []string{"instance", "launch_template", "autoscaling", "ecs_", "eks_", "lambda_"},
		},
		{
			name:        "Storage",
			description: "Storage resource changes",
			risk:        "standard",
			keywords:    []string{"s3_bucket", "ebs_", "efs_"},
		},
		{
			name:        "Database",
			description: "Database resource changes",
			risk:        "standard",
			keywords:    []string{"rds_", "db_instance", "dynamodb_", "elasticache_"},
		},
		{
			name:        "Monitoring",
			description: "Observability and monitoring changes",
			risk:        "standard",
			keywords:    []string{"cloudwatch_", "sns_", "cloudtrail"},
		},
	},
}

// Categorize groups resource types into semantic categories based on
// provider-specific knowledge. Types that don't match any known category
// are collected in Uncategorized.
func Categorize(parsed *ParseResult) *CategorizeResult {
	result := &CategorizeResult{
		Modules: parsed.Modules,
	}

	provider := detectProvider(parsed.ResourceTypes)
	defs, known := providerKnowledge[provider]
	if !known {
		// Unknown provider: everything is uncategorized
		result.Uncategorized = parsed.ResourceTypes
		return result
	}

	// Track which types have been categorized
	categorized := make(map[string]bool)

	for _, def := range defs {
		var matched []string
		for _, rt := range parsed.ResourceTypes {
			if categorized[rt] {
				continue
			}
			suffix := stripProviderPrefix(rt, provider)
			if matchesCategory(suffix, def) {
				matched = append(matched, rt)
			}
		}

		if len(matched) == 0 {
			continue
		}

		for _, rt := range matched {
			categorized[rt] = true
		}

		cat := Category{
			Name:        def.name,
			Description: def.description,
			Types:       matched,
			Patterns:    generatePatterns(matched, provider),
		}

		if def.risk == "critical" {
			result.Critical = append(result.Critical, cat)
		} else {
			result.Standard = append(result.Standard, cat)
		}
	}

	// Collect uncategorized
	for _, rt := range parsed.ResourceTypes {
		if !categorized[rt] {
			result.Uncategorized = append(result.Uncategorized, rt)
		}
	}

	return result
}

// detectProvider returns the most common provider prefix among the resource types.
func detectProvider(types []string) string {
	counts := make(map[string]int)
	for _, rt := range types {
		if idx := strings.IndexByte(rt, '_'); idx > 0 {
			counts[rt[:idx]]++
		}
	}

	var best string
	var bestCount int
	for prefix, count := range counts {
		if count > bestCount {
			best = prefix
			bestCount = count
		}
	}
	return best
}

// stripProviderPrefix removes the provider prefix (e.g., "azurerm_") from
// a resource type, returning the remainder.
func stripProviderPrefix(rt, provider string) string {
	prefix := provider + "_"
	if strings.HasPrefix(rt, prefix) {
		return rt[len(prefix):]
	}
	return rt
}

// matchesCategory checks whether a resource type suffix matches a category definition.
func matchesCategory(suffix string, def categoryDef) bool {
	for _, kw := range def.keywords {
		if def.exact {
			if suffix == kw {
				return true
			}
		} else {
			if strings.Contains(suffix, kw) || strings.HasPrefix(suffix, kw) {
				return true
			}
		}
	}
	return false
}

// generatePatterns creates glob patterns for a set of resource types.
// Uses provider-agnostic "*_" prefix patterns to match the convention
// used throughout existing tfclassify configs.
func generatePatterns(types []string, provider string) []string {
	patterns := make([]string, 0, len(types))
	seen := make(map[string]bool)

	for _, rt := range types {
		suffix := stripProviderPrefix(rt, provider)
		pattern := "*_" + suffix
		if !seen[pattern] {
			seen[pattern] = true
			patterns = append(patterns, pattern)
		}
	}

	sort.Strings(patterns)
	return patterns
}
