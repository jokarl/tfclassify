package config

import (
	"testing"
)

// FuzzParseHCL fuzzes the HCL config parser with malformed input.
// Seed corpus includes valid config snippets from testdata.
func FuzzParseHCL(f *testing.F) {
	// Seed with a minimal valid config
	f.Add([]byte(`
classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`))

	// Seed with a config containing a plugin
	f.Add([]byte(`
plugin "azurerm" {
  enabled = true
  source  = "github.com/example/plugin"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical"
  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions = ["*"]
    }
  }
}

classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`))

	// Seed with empty input
	f.Add([]byte(``))

	// Seed with truncated HCL
	f.Add([]byte(`classification "broken" {`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse should not panic on any input
		_, _ = Parse(data, "fuzz.hcl")
	})
}
