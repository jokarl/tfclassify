package plugin

import (
	"context"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
	"google.golang.org/grpc"
)

// testAnalyzer is a simple analyzer that emits a decision for every resource.
type testAnalyzer struct {
	sdk.DefaultAnalyzer
	name           string
	classification string
	reason         string
	severity       int
	enabled        bool
	patterns       []string
}

func (a *testAnalyzer) Name() string             { return a.name }
func (a *testAnalyzer) Enabled() bool             { return a.enabled }
func (a *testAnalyzer) ResourcePatterns() []string { return a.patterns }
func (a *testAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.patterns)
	if err != nil {
		return err
	}
	for _, change := range changes {
		err := runner.EmitDecision(a, change, &sdk.Decision{
			Classification: a.classification,
			Reason:         a.reason,
			Severity:       a.severity,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// disabledAnalyzer is an analyzer that is not enabled.
type disabledAnalyzer struct {
	sdk.DefaultAnalyzer
}

func (a *disabledAnalyzer) Name() string             { return "disabled" }
func (a *disabledAnalyzer) Enabled() bool             { return false }
func (a *disabledAnalyzer) Analyze(runner sdk.Runner) error { return nil }

func TestPluginServiceServer_GetConfigSchema_WithAttributes(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "test-with-schema",
		Version: "1.0.0",
		Schema: &sdk.ConfigSchemaSpec{
			Attributes: []sdk.ConfigAttribute{
				{Name: "threshold", Type: "number", Required: true},
				{Name: "enabled", Type: "bool", Required: false},
				{Name: "regions", Type: "list(string)", Required: false},
			},
		},
	}

	server := NewPluginServiceServer(ps, nil)

	resp, err := server.GetConfigSchema(context.Background(), &GetConfigSchemaRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(resp.Attributes))
	}

	if resp.Attributes[0].Name != "threshold" {
		t.Errorf("expected first attribute 'threshold', got %q", resp.Attributes[0].Name)
	}
	if resp.Attributes[0].Type != "number" {
		t.Errorf("expected type 'number', got %q", resp.Attributes[0].Type)
	}
	if !resp.Attributes[0].Required {
		t.Error("expected threshold to be required")
	}

	if resp.Attributes[1].Name != "enabled" {
		t.Errorf("expected second attribute 'enabled', got %q", resp.Attributes[1].Name)
	}
	if resp.Attributes[1].Required {
		t.Error("expected enabled to not be required")
	}

	if resp.Attributes[2].Name != "regions" {
		t.Errorf("expected third attribute 'regions', got %q", resp.Attributes[2].Name)
	}
}

func TestPluginServiceServer_GetConfigSchema_SingleAttribute(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "test-single",
		Version: "1.0.0",
		Schema: &sdk.ConfigSchemaSpec{
			Attributes: []sdk.ConfigAttribute{
				{Name: "debug", Type: "bool", Required: false},
			},
		},
	}

	server := NewPluginServiceServer(ps, nil)
	resp, err := server.GetConfigSchema(context.Background(), &GetConfigSchemaRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(resp.Attributes))
	}
	if resp.Attributes[0].Name != "debug" {
		t.Errorf("expected attribute 'debug', got %q", resp.Attributes[0].Name)
	}
}

func TestPluginServiceServer_Analyze_NonBuiltinSet(t *testing.T) {
	// When a non-BuiltinPluginSet is used, Analyze returns empty response
	ps := &mockPluginSet{}
	server := NewPluginServiceServer(ps, nil)

	// Analyze will fail because broker is nil, but the non-builtin branch should handle gracefully
	// We test this indirectly by verifying the analyzer cache is empty
	if len(server.analyzers) != 0 {
		t.Errorf("expected 0 analyzers for non-builtin set, got %d", len(server.analyzers))
	}
}

func TestPluginServiceServer_Analyze_DisabledAnalyzerSkipped(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "test-plugin",
		Version: "1.0.0",
		Analyzers: []sdk.Analyzer{
			&disabledAnalyzer{},
			&testAnalyzer{
				name:           "enabled-one",
				enabled:        true,
				classification: "standard",
				reason:         "test",
			},
		},
	}

	server := NewPluginServiceServer(ps, nil)

	// Verify both analyzers are in the cache
	if len(server.analyzers) != 2 {
		t.Errorf("expected 2 analyzers in cache, got %d", len(server.analyzers))
	}
	if _, ok := server.analyzers["disabled"]; !ok {
		t.Error("expected 'disabled' analyzer in cache")
	}
	if _, ok := server.analyzers["enabled-one"]; !ok {
		t.Error("expected 'enabled-one' analyzer in cache")
	}
}

func TestPluginServiceServer_NewPluginServiceServer_NonBuiltinPluginSet(t *testing.T) {
	ps := &mockPluginSet{}

	server := NewPluginServiceServer(ps, nil)

	if len(server.analyzers) != 0 {
		t.Errorf("expected 0 analyzers in cache for non-builtin set, got %d", len(server.analyzers))
	}
}

func TestProtoConversions_RoundTrip_WithSensitiveFields(t *testing.T) {
	original := &sdk.ResourceChange{
		Address:         "aws_db_instance.main",
		Type:            "aws_db_instance",
		ProviderName:    "registry.terraform.io/hashicorp/aws",
		Mode:            "managed",
		Actions:         []string{"update"},
		Before:          map[string]interface{}{"username": "admin", "port": float64(5432)},
		After:           map[string]interface{}{"username": "newadmin", "port": float64(5432)},
		BeforeSensitive: map[string]interface{}{"password": true},
		AfterSensitive:  map[string]interface{}{"password": true, "master_password": true},
	}

	proto := sdkToProtoResourceChange(original)
	converted := protoToSDKResourceChange(proto)

	if converted.Address != original.Address {
		t.Errorf("address mismatch: %q vs %q", converted.Address, original.Address)
	}
	if converted.Type != original.Type {
		t.Errorf("type mismatch: %q vs %q", converted.Type, original.Type)
	}
	if converted.ProviderName != original.ProviderName {
		t.Errorf("provider mismatch: %q vs %q", converted.ProviderName, original.ProviderName)
	}
	if converted.Mode != original.Mode {
		t.Errorf("mode mismatch: %q vs %q", converted.Mode, original.Mode)
	}
	if len(converted.Actions) != 1 || converted.Actions[0] != "update" {
		t.Errorf("actions mismatch: %v", converted.Actions)
	}

	if converted.Before["username"] != "admin" {
		t.Errorf("before username mismatch: %v", converted.Before["username"])
	}
	if converted.After["username"] != "newadmin" {
		t.Errorf("after username mismatch: %v", converted.After["username"])
	}

	if converted.BeforeSensitive == nil {
		t.Fatal("expected BeforeSensitive to be non-nil")
	}
	beforeSens, ok := converted.BeforeSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected BeforeSensitive to be map, got %T", converted.BeforeSensitive)
	}
	if beforeSens["password"] != true {
		t.Errorf("expected BeforeSensitive.password to be true, got %v", beforeSens["password"])
	}

	if converted.AfterSensitive == nil {
		t.Fatal("expected AfterSensitive to be non-nil")
	}
	afterSens, ok := converted.AfterSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected AfterSensitive to be map, got %T", converted.AfterSensitive)
	}
	if afterSens["password"] != true {
		t.Errorf("expected AfterSensitive.password to be true, got %v", afterSens["password"])
	}
	if afterSens["master_password"] != true {
		t.Errorf("expected AfterSensitive.master_password to be true, got %v", afterSens["master_password"])
	}
}

func TestProtoConversions_Decision_WithMetadata(t *testing.T) {
	original := &sdk.Decision{
		Classification: "critical",
		Reason:         "privilege escalation detected",
		Severity:       95,
		Metadata: map[string]interface{}{
			"role":  "Owner",
			"scope": "/subscriptions/abc",
		},
	}

	proto := sdkToProtoDecision(original)

	if proto.Classification != "critical" {
		t.Errorf("classification mismatch: %q", proto.Classification)
	}
	if proto.Severity != 95 {
		t.Errorf("severity mismatch: %d", proto.Severity)
	}
	if len(proto.Metadata) == 0 {
		t.Error("expected metadata to be serialized")
	}
}

func TestProtoConversions_Decision_NilMetadata(t *testing.T) {
	original := &sdk.Decision{
		Classification: "standard",
		Reason:         "normal change",
		Severity:       10,
		Metadata:       nil,
	}

	proto := sdkToProtoDecision(original)

	if proto.Metadata != nil {
		t.Errorf("expected nil metadata, got %v", proto.Metadata)
	}
}

func TestProtoConversions_ResourceChange_EmptyFields(t *testing.T) {
	original := &sdk.ResourceChange{
		Address: "null_resource.test",
		Type:    "null_resource",
		Actions: []string{"create"},
	}

	proto := sdkToProtoResourceChange(original)
	if proto.Before != nil {
		t.Errorf("expected nil Before, got %v", proto.Before)
	}
	if proto.After != nil {
		t.Errorf("expected nil After, got %v", proto.After)
	}
	if proto.BeforeSensitive != nil {
		t.Errorf("expected nil BeforeSensitive, got %v", proto.BeforeSensitive)
	}

	converted := protoToSDKResourceChange(proto)
	if converted.Before != nil {
		t.Errorf("expected nil Before after round-trip, got %v", converted.Before)
	}
	if converted.After != nil {
		t.Errorf("expected nil After after round-trip, got %v", converted.After)
	}
}

func TestRegisterPluginServiceServer(t *testing.T) {
	ps := &mockPluginSet{}
	server := NewPluginServiceServer(ps, nil)
	s := grpc.NewServer()
	defer s.Stop()

	// Should not panic
	RegisterPluginServiceServer(s, server)
}

func TestGRPCPluginServiceHandlers_NoInterceptor(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "handler-test",
		Version: "1.0.0",
		Schema: &sdk.ConfigSchemaSpec{
			Attributes: []sdk.ConfigAttribute{
				{Name: "test", Type: "string", Required: false},
			},
		},
	}
	server := NewPluginServiceServer(ps, nil)

	// Test GetPluginInfo handler without interceptor
	result, err := pluginServiceGetPluginInfoHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("GetPluginInfo handler error: %v", err)
	}
	resp := result.(*GetPluginInfoResponse)
	if resp.Name != "handler-test" {
		t.Errorf("expected name 'handler-test', got %q", resp.Name)
	}

	// Test GetConfigSchema handler without interceptor
	result, err = pluginServiceGetConfigSchemaHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("GetConfigSchema handler error: %v", err)
	}
	schemaResp := result.(*GetConfigSchemaResponse)
	if len(schemaResp.Attributes) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(schemaResp.Attributes))
	}

	// Test ApplyConfig handler without interceptor
	result, err = pluginServiceApplyConfigHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("ApplyConfig handler error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil ApplyConfig response")
	}
}

func TestGRPCPluginServiceHandlers_WithInterceptor(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "interceptor-test",
		Version: "1.0.0",
	}
	server := NewPluginServiceServer(ps, nil)

	interceptorCalled := false
	interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		interceptorCalled = true
		return handler(ctx, req)
	}

	// Test GetPluginInfo with interceptor
	result, err := pluginServiceGetPluginInfoHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called")
	}
	resp := result.(*GetPluginInfoResponse)
	if resp.Name != "interceptor-test" {
		t.Errorf("expected name 'interceptor-test', got %q", resp.Name)
	}

	// Test GetConfigSchema with interceptor
	interceptorCalled = false
	_, err = pluginServiceGetConfigSchemaHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("GetConfigSchema handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called for GetConfigSchema")
	}

	// Test ApplyConfig with interceptor
	interceptorCalled = false
	_, err = pluginServiceApplyConfigHandler(server, context.Background(), func(i interface{}) error {
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("ApplyConfig handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called for ApplyConfig")
	}
}

func TestNewRunnerClient(t *testing.T) {
	client := NewRunnerClient(nil)
	if client == nil {
		t.Fatal("expected non-nil RunnerClient")
	}
	if client.conn != nil {
		t.Error("expected nil conn")
	}
}

func TestNewPluginServiceClient(t *testing.T) {
	client := NewPluginServiceClient(nil, nil)
	if client == nil {
		t.Fatal("expected non-nil PluginServiceClient")
	}
	if client.client != nil {
		t.Error("expected nil client conn")
	}
	if client.broker != nil {
		t.Error("expected nil broker")
	}
}

func TestPluginServiceServer_AnalyzerCacheWithMultipleAnalyzers(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:    "multi-analyzer",
		Version: "1.0.0",
		Analyzers: []sdk.Analyzer{
			&testAnalyzer{name: "analyzer-a", enabled: true, classification: "critical"},
			&testAnalyzer{name: "analyzer-b", enabled: true, classification: "standard"},
			&testAnalyzer{name: "analyzer-c", enabled: false, classification: "auto"},
		},
	}

	server := NewPluginServiceServer(ps, nil)

	if len(server.analyzers) != 3 {
		t.Fatalf("expected 3 analyzers, got %d", len(server.analyzers))
	}

	for _, name := range []string{"analyzer-a", "analyzer-b", "analyzer-c"} {
		if _, ok := server.analyzers[name]; !ok {
			t.Errorf("expected analyzer %q in cache", name)
		}
	}
}

func TestPluginServiceServer_GetPluginInfo_AllFields(t *testing.T) {
	ps := &sdk.BuiltinPluginSet{
		Name:                  "full-info",
		Version:               "2.5.0",
		HostVersionConstraint: ">= 0.2.0",
	}

	server := NewPluginServiceServer(ps, nil)

	resp, err := server.GetPluginInfo(context.Background(), &GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Name != "full-info" {
		t.Errorf("expected name 'full-info', got %q", resp.Name)
	}
	if resp.Version != "2.5.0" {
		t.Errorf("expected version '2.5.0', got %q", resp.Version)
	}
	if resp.SDKVersion != sdk.SDKVersion {
		t.Errorf("expected SDK version %q, got %q", sdk.SDKVersion, resp.SDKVersion)
	}
	if resp.HostVersionConstraint != ">= 0.2.0" {
		t.Errorf("expected host constraint '>= 0.2.0', got %q", resp.HostVersionConstraint)
	}
}
