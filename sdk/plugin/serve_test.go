package plugin

import (
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestHandshakeConfig(t *testing.T) {
	// Verify handshake config has expected values
	if HandshakeConfig.ProtocolVersion != 1 {
		t.Errorf("expected ProtocolVersion 1, got %d", HandshakeConfig.ProtocolVersion)
	}

	if HandshakeConfig.MagicCookieKey != "TFCLASSIFY_PLUGIN" {
		t.Errorf("expected MagicCookieKey 'TFCLASSIFY_PLUGIN', got %q", HandshakeConfig.MagicCookieKey)
	}

	if HandshakeConfig.MagicCookieValue == "" {
		t.Error("MagicCookieValue should not be empty")
	}
}

func TestPluginName(t *testing.T) {
	if PluginName != "tfclassify" {
		t.Errorf("expected PluginName 'tfclassify', got %q", PluginName)
	}
}

// mockPluginSet implements sdk.PluginSet for testing
type mockPluginSet struct{}

func (m *mockPluginSet) PluginSetName() string {
	return "test"
}

func (m *mockPluginSet) PluginSetVersion() string {
	return "1.0.0"
}

func (m *mockPluginSet) AnalyzerNames() []string {
	return []string{}
}

func (m *mockPluginSet) VersionConstraint() string {
	return ""
}

func (m *mockPluginSet) ConfigSchema() *sdk.ConfigSchemaSpec {
	return nil
}

func TestGRPCPluginImpl_GRPCServer(t *testing.T) {
	impl := &GRPCPluginImpl{
		Impl: &mockPluginSet{},
	}

	// GRPCServer should not error (TODO implementation)
	err := impl.GRPCServer(nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGRPCPluginImpl_GRPCClient(t *testing.T) {
	impl := &GRPCPluginImpl{
		Impl: &mockPluginSet{},
	}

	// GRPCClient should not error (TODO implementation)
	result, err := impl.GRPCClient(nil, nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result from TODO implementation")
	}
}

func TestServeOpts(t *testing.T) {
	opts := &ServeOpts{
		PluginSet: &mockPluginSet{},
	}

	if opts.PluginSet == nil {
		t.Error("expected PluginSet to be set")
	}

	if opts.PluginSet.PluginSetName() != "test" {
		t.Errorf("expected name 'test', got %q", opts.PluginSet.PluginSetName())
	}
}
