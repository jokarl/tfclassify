package plugin

import (
	"context"
	"testing"

	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/sdk"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
	"google.golang.org/grpc"
)

func TestRegisterRunnerServiceServer_Integration(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	s := grpc.NewServer()
	defer s.Stop()

	// Should not panic
	RegisterRunnerServiceServer(s, server)
}

func TestRunnerServiceHandlers_NoInterceptor(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type", Actions: []string{"create"}},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	// Test GetResourceChanges handler without interceptor
	result, err := runnerServiceGetResourceChangesHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.GetResourceChangesRequest)
		req.Patterns = []string{"*"}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	resp := result.(*sdkplugin.GetResourceChangesResponse)
	if len(resp.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(resp.Changes))
	}

	// Test GetResourceChange handler without interceptor
	result, err = runnerServiceGetResourceChangeHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.GetResourceChangeRequest)
		req.Address = "test.resource"
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	changeResp := result.(*sdkplugin.GetResourceChangeResponse)
	if changeResp.Change == nil {
		t.Error("expected non-nil change")
	}

	// Test EmitDecision handler without interceptor
	result, err = runnerServiceEmitDecisionHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.EmitDecisionRequest)
		req.AnalyzerName = "test"
		req.Change = &sdkplugin.ResourceChange{Address: "test.resource", Type: "test_type", Actions: []string{"create"}}
		req.Decision = &sdkplugin.Decision{Classification: "standard", Reason: "test"}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil response")
	}
}

func TestRunnerServiceHandlers_WithInterceptor(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type", Actions: []string{"create"}},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	interceptorCalled := false
	interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		interceptorCalled = true
		return handler(ctx, req)
	}

	// Test GetResourceChanges with interceptor
	_, err := runnerServiceGetResourceChangesHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.GetResourceChangesRequest)
		req.Patterns = []string{"*"}
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called")
	}

	// Test GetResourceChange with interceptor
	interceptorCalled = false
	_, err = runnerServiceGetResourceChangeHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.GetResourceChangeRequest)
		req.Address = "test.resource"
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called for GetResourceChange")
	}

	// Test EmitDecision with interceptor
	interceptorCalled = false
	_, err = runnerServiceEmitDecisionHandler(server, context.Background(), func(i interface{}) error {
		req := i.(*sdkplugin.EmitDecisionRequest)
		req.AnalyzerName = "test"
		req.Change = &sdkplugin.ResourceChange{Address: "test.resource", Type: "test_type", Actions: []string{"create"}}
		req.Decision = &sdkplugin.Decision{Classification: "standard", Reason: "test"}
		return nil
	}, interceptor)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !interceptorCalled {
		t.Error("expected interceptor to be called for EmitDecision")
	}
}

func TestProtoConversions_SDKToProto_WithSensitiveFields(t *testing.T) {
	sdkChange := &sdk.ResourceChange{
		Address:         "aws_db_instance.main",
		Type:            "aws_db_instance",
		ProviderName:    "registry.terraform.io/hashicorp/aws",
		Mode:            "managed",
		Actions:         []string{"update"},
		Before:          map[string]interface{}{"username": "admin"},
		After:           map[string]interface{}{"username": "newadmin"},
		BeforeSensitive: map[string]interface{}{"password": true},
		AfterSensitive:  map[string]interface{}{"password": true, "api_key": true},
	}

	proto := sdkToProtoResourceChange(sdkChange)

	if proto.BeforeSensitive == nil {
		t.Fatal("expected BeforeSensitive to be non-nil")
	}
	if proto.AfterSensitive == nil {
		t.Fatal("expected AfterSensitive to be non-nil")
	}

	// Round-trip and verify
	converted := protoToSDKResourceChange(proto)
	if converted.BeforeSensitive == nil {
		t.Fatal("expected BeforeSensitive to survive round-trip")
	}
	beforeSens, ok := converted.BeforeSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map type, got %T", converted.BeforeSensitive)
	}
	if beforeSens["password"] != true {
		t.Errorf("expected password to be true, got %v", beforeSens["password"])
	}

	afterSens, ok := converted.AfterSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map type, got %T", converted.AfterSensitive)
	}
	if afterSens["api_key"] != true {
		t.Errorf("expected api_key to be true, got %v", afterSens["api_key"])
	}
}

func TestProtoConversions_ProtoToSDK_WithAllSensitiveFields(t *testing.T) {
	proto := &sdkplugin.ResourceChange{
		Address:         "test.resource",
		Type:            "test_type",
		Before:          []byte(`{"key":"old"}`),
		After:           []byte(`{"key":"new"}`),
		BeforeSensitive: []byte(`{"secret":true}`),
		AfterSensitive:  []byte(`{"secret":true,"token":true}`),
	}

	converted := protoToSDKResourceChange(proto)

	if converted.Before == nil || converted.Before["key"] != "old" {
		t.Errorf("Before round-trip failed: %v", converted.Before)
	}
	if converted.After == nil || converted.After["key"] != "new" {
		t.Errorf("After round-trip failed: %v", converted.After)
	}
	if converted.BeforeSensitive == nil {
		t.Fatal("BeforeSensitive should not be nil")
	}
	if converted.AfterSensitive == nil {
		t.Fatal("AfterSensitive should not be nil")
	}

	afterSens, ok := converted.AfterSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map type, got %T", converted.AfterSensitive)
	}
	if afterSens["token"] != true {
		t.Errorf("expected token to be true, got %v", afterSens["token"])
	}
}

func TestProtoConversions_Decision_WithAllFields(t *testing.T) {
	proto := &sdkplugin.Decision{
		Classification: "critical",
		Reason:         "test",
		Severity:       85,
		Metadata:       []byte(`{"role":"Owner","scope":"/subscriptions/abc"}`),
	}

	converted := protoToSDKDecision(proto)

	if converted.Classification != "critical" {
		t.Errorf("classification mismatch: %q", converted.Classification)
	}
	if converted.Reason != "test" {
		t.Errorf("reason mismatch: %q", converted.Reason)
	}
	if converted.Severity != 85 {
		t.Errorf("severity mismatch: %d", converted.Severity)
	}
	if converted.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
	if converted.Metadata["role"] != "Owner" {
		t.Errorf("expected role 'Owner', got %v", converted.Metadata["role"])
	}
}

func TestRunnerServiceHandler_GetResourceChanges_Error(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type"},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	_, err := server.GetResourceChanges(context.Background(),
		&sdkplugin.GetResourceChangesRequest{Patterns: []string{"[invalid"}})
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}
