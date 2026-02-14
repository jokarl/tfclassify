package plugin

import (
	"context"
	"testing"

	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/sdk"
	"github.com/jokarl/tfclassify/sdk/pb"
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
	pb.RegisterRunnerServiceServer(s, server)
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

	proto := sdkplugin.SDKToProtoResourceChange(sdkChange)

	if proto.BeforeSensitive == nil {
		t.Fatal("expected BeforeSensitive to be non-nil")
	}
	if proto.AfterSensitive == nil {
		t.Fatal("expected AfterSensitive to be non-nil")
	}

	// Round-trip and verify
	converted := sdkplugin.ProtoToSDKResourceChange(proto)
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
	proto := &pb.ResourceChange{
		Address:         "test.resource",
		Type:            "test_type",
		Before:          []byte(`{"key":"old"}`),
		After:           []byte(`{"key":"new"}`),
		BeforeSensitive: []byte(`{"secret":true}`),
		AfterSensitive:  []byte(`{"secret":true,"token":true}`),
	}

	converted := sdkplugin.ProtoToSDKResourceChange(proto)

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
	proto := &pb.Decision{
		Classification: "critical",
		Reason:         "test",
		Severity:       85,
		Metadata:       []byte(`{"role":"Owner","scope":"/subscriptions/abc"}`),
	}

	converted := sdkplugin.ProtoToSDKDecision(proto)

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

func TestRunnerServiceServer_GetResourceChanges_Error(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type"},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	_, err := server.GetResourceChanges(context.Background(),
		&pb.GetResourceChangesRequest{Patterns: []string{"[invalid"}})
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}
