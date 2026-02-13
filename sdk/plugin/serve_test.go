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

func TestHandshakeConfig_MagicCookieLength(t *testing.T) {
	// CR-0012 hardened the magic cookie to 60 characters
	if len(HandshakeConfig.MagicCookieValue) < 50 {
		t.Errorf("MagicCookieValue should be at least 50 chars for security, got %d", len(HandshakeConfig.MagicCookieValue))
	}
}

func TestGRPCPluginImpl_Impl(t *testing.T) {
	ps := &mockPluginSet{}
	impl := &GRPCPluginImpl{
		Impl: ps,
	}

	// Verify Impl is accessible
	if impl.Impl == nil {
		t.Error("expected Impl to be set")
	}

	if impl.Impl.PluginSetName() != "test" {
		t.Errorf("expected PluginSetName 'test', got %q", impl.Impl.PluginSetName())
	}
}

func TestMockPluginSet_AllMethods(t *testing.T) {
	ps := &mockPluginSet{}

	// Test all interface methods
	if ps.PluginSetName() != "test" {
		t.Errorf("PluginSetName: expected 'test', got %q", ps.PluginSetName())
	}

	if ps.PluginSetVersion() != "1.0.0" {
		t.Errorf("PluginSetVersion: expected '1.0.0', got %q", ps.PluginSetVersion())
	}

	if len(ps.AnalyzerNames()) != 0 {
		t.Errorf("AnalyzerNames: expected empty, got %v", ps.AnalyzerNames())
	}

	if ps.VersionConstraint() != "" {
		t.Errorf("VersionConstraint: expected empty, got %q", ps.VersionConstraint())
	}

	if ps.ConfigSchema() != nil {
		t.Error("ConfigSchema: expected nil")
	}
}

func TestHandshakeConfig_CR0012Compliance(t *testing.T) {
	// CR-0012 requires a 60+ character magic cookie value for security
	if len(HandshakeConfig.MagicCookieValue) < 60 {
		t.Errorf("MagicCookieValue must be at least 60 chars per CR-0012, got %d chars",
			len(HandshakeConfig.MagicCookieValue))
	}

	// CR-0012 requires ProtocolVersion to remain at 1
	if HandshakeConfig.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion must remain at 1 per CR-0012, got %d",
			HandshakeConfig.ProtocolVersion)
	}
}

func TestServeOpts_NilPluginSet(t *testing.T) {
	// Test that ServeOpts can be created with nil (for validation purposes)
	opts := &ServeOpts{
		PluginSet: nil,
	}

	if opts.PluginSet != nil {
		t.Error("expected nil PluginSet")
	}
}

func TestGRPCPluginImpl_NilImpl(t *testing.T) {
	// Test GRPCPluginImpl with nil Impl doesn't panic on method calls
	impl := &GRPCPluginImpl{
		Impl: nil,
	}

	// Should not panic even with nil Impl
	err := impl.GRPCServer(nil, nil)
	if err != nil {
		t.Errorf("GRPCServer with nil Impl should not error: %v", err)
	}

	result, err := impl.GRPCClient(nil, nil, nil)
	if err != nil {
		t.Errorf("GRPCClient with nil Impl should not error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}
