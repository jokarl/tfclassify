// Package plugin provides the entry point for tfclassify plugins.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jokarl/tfclassify/sdk"
	"github.com/jokarl/tfclassify/sdk/pb"
	"google.golang.org/grpc"
)

// PluginServiceServer implements the gRPC server for PluginService.
// This is the server side that runs inside the plugin process.
type PluginServiceServer struct {
	pb.UnimplementedPluginServiceServer
	impl   sdk.PluginSet
	broker *goplugin.GRPCBroker
}

// NewPluginServiceServer creates a new plugin service server.
func NewPluginServiceServer(impl sdk.PluginSet, broker *goplugin.GRPCBroker) *PluginServiceServer {
	return &PluginServiceServer{
		impl:   impl,
		broker: broker,
	}
}

// GetPluginInfo returns plugin metadata for version negotiation.
func (s *PluginServiceServer) GetPluginInfo(ctx context.Context, req *pb.GetPluginInfoRequest) (*pb.GetPluginInfoResponse, error) {
	return &pb.GetPluginInfoResponse{
		Name:                  s.impl.PluginSetName(),
		Version:               s.impl.PluginSetVersion(),
		SdkVersion:            sdk.SDKVersion,
		HostVersionConstraint: s.impl.VersionConstraint(),
	}, nil
}

// GetConfigSchema returns the configuration schema for validation.
func (s *PluginServiceServer) GetConfigSchema(ctx context.Context, req *pb.GetConfigSchemaRequest) (*pb.GetConfigSchemaResponse, error) {
	schema := s.impl.ConfigSchema()
	if schema == nil {
		return &pb.GetConfigSchemaResponse{Attributes: nil}, nil
	}

	attrs := make([]*pb.ConfigAttribute, len(schema.Attributes))
	for i, a := range schema.Attributes {
		attrs[i] = &pb.ConfigAttribute{
			Name:        a.Name,
			Type:        a.Type,
			Required:    a.Required,
			Description: a.Description,
		}
	}

	return &pb.GetConfigSchemaResponse{Attributes: attrs}, nil
}

// ApplyConfig applies the plugin-specific configuration.
func (s *PluginServiceServer) ApplyConfig(ctx context.Context, req *pb.ApplyConfigRequest) (*pb.ApplyConfigResponse, error) {
	return &pb.ApplyConfigResponse{}, nil
}

// Analyze runs all enabled analyzers in the plugin.
// If classification and analyzerConfig are provided, only analyzers that implement
// ClassificationAwareAnalyzer are called with the classification context.
func (s *PluginServiceServer) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalyzeResponse, error) {
	conn, err := s.broker.Dial(req.BrokerId)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runner: %w", err)
	}
	defer conn.Close()

	// Create a classification-aware runner that sets the classification on emitted decisions
	runner := NewClassificationAwareRunnerClient(conn, req.Classification)

	builtinSet, ok := s.impl.(*sdk.BuiltinPluginSet)
	if !ok {
		return &pb.AnalyzeResponse{}, nil
	}

	classification := req.Classification
	analyzerConfig := req.AnalyzerConfig

	for _, analyzer := range builtinSet.Analyzers {
		if !analyzer.Enabled() {
			continue
		}

		// If we have a classification, try classification-aware analysis
		if classification != "" {
			if classificationAware, ok := analyzer.(sdk.ClassificationAwareAnalyzer); ok {
				if err := classificationAware.AnalyzeWithClassification(runner, classification, analyzerConfig); err != nil {
					continue
				}
				continue
			}
		}

		// Fallback to standard analysis (backward compatible)
		if err := analyzer.Analyze(runner); err != nil {
			continue
		}
	}

	return &pb.AnalyzeResponse{}, nil
}

// RunnerClient implements sdk.Runner by calling back to the host via gRPC.
type RunnerClient struct {
	client pb.RunnerServiceClient
}

// NewRunnerClient creates a new runner client.
func NewRunnerClient(conn *grpc.ClientConn) *RunnerClient {
	return &RunnerClient{client: pb.NewRunnerServiceClient(conn)}
}

// ClassificationAwareRunnerClient wraps RunnerClient and automatically sets
// the classification on emitted decisions.
type ClassificationAwareRunnerClient struct {
	*RunnerClient
	classification string
}

// NewClassificationAwareRunnerClient creates a runner client that sets the classification
// on all emitted decisions.
func NewClassificationAwareRunnerClient(conn *grpc.ClientConn, classification string) *ClassificationAwareRunnerClient {
	return &ClassificationAwareRunnerClient{
		RunnerClient:   NewRunnerClient(conn),
		classification: classification,
	}
}

// EmitDecision records a classification decision, automatically setting the classification.
func (r *ClassificationAwareRunnerClient) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	// If the decision doesn't have a classification set, use the one from the request
	if decision.Classification == "" && r.classification != "" {
		decision.Classification = r.classification
	}
	return r.RunnerClient.EmitDecision(analyzer, change, decision)
}

// GetResourceChanges returns resource changes matching the given patterns.
func (r *RunnerClient) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	resp, err := r.client.GetResourceChanges(context.Background(), &pb.GetResourceChangesRequest{Patterns: patterns})
	if err != nil {
		return nil, fmt.Errorf("GetResourceChanges RPC failed: %w", err)
	}

	changes := make([]*sdk.ResourceChange, len(resp.Changes))
	for i, c := range resp.Changes {
		changes[i] = ProtoToSDKResourceChange(c)
	}
	return changes, nil
}

// GetResourceChange returns a specific resource change by address.
func (r *RunnerClient) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	resp, err := r.client.GetResourceChange(context.Background(), &pb.GetResourceChangeRequest{Address: address})
	if err != nil {
		return nil, fmt.Errorf("GetResourceChange RPC failed: %w", err)
	}

	if resp.Change == nil {
		return nil, fmt.Errorf("resource not found: %s", address)
	}
	return ProtoToSDKResourceChange(resp.Change), nil
}

// EmitDecision records a classification decision for a resource.
func (r *RunnerClient) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	req := &pb.EmitDecisionRequest{
		AnalyzerName: analyzer.Name(),
		Change:       SDKToProtoResourceChange(change),
		Decision:     SDKToProtoDecision(decision),
	}

	_, err := r.client.EmitDecision(context.Background(), req)
	if err != nil {
		return fmt.Errorf("EmitDecision RPC failed: %w", err)
	}
	return nil
}

// Conversion functions between SDK types and protobuf types.

// ProtoToSDKResourceChange converts a protobuf ResourceChange to an SDK ResourceChange.
func ProtoToSDKResourceChange(p *pb.ResourceChange) *sdk.ResourceChange {
	if p == nil {
		return nil
	}

	var before, after map[string]interface{}
	var beforeSensitive, afterSensitive interface{}

	if len(p.Before) > 0 {
		json.Unmarshal(p.Before, &before)
	}
	if len(p.After) > 0 {
		json.Unmarshal(p.After, &after)
	}
	if len(p.BeforeSensitive) > 0 {
		json.Unmarshal(p.BeforeSensitive, &beforeSensitive)
	}
	if len(p.AfterSensitive) > 0 {
		json.Unmarshal(p.AfterSensitive, &afterSensitive)
	}

	return &sdk.ResourceChange{
		Address:         p.Address,
		Type:            p.Type,
		ProviderName:    p.ProviderName,
		Mode:            p.Mode,
		Actions:         p.Actions,
		Before:          before,
		After:           after,
		BeforeSensitive: beforeSensitive,
		AfterSensitive:  afterSensitive,
	}
}

// SDKToProtoResourceChange converts an SDK ResourceChange to a protobuf ResourceChange.
func SDKToProtoResourceChange(s *sdk.ResourceChange) *pb.ResourceChange {
	if s == nil {
		return nil
	}

	var before, after, beforeSensitive, afterSensitive []byte

	if s.Before != nil {
		before, _ = json.Marshal(s.Before)
	}
	if s.After != nil {
		after, _ = json.Marshal(s.After)
	}
	if s.BeforeSensitive != nil {
		beforeSensitive, _ = json.Marshal(s.BeforeSensitive)
	}
	if s.AfterSensitive != nil {
		afterSensitive, _ = json.Marshal(s.AfterSensitive)
	}

	return &pb.ResourceChange{
		Address:         s.Address,
		Type:            s.Type,
		ProviderName:    s.ProviderName,
		Mode:            s.Mode,
		Actions:         s.Actions,
		Before:          before,
		After:           after,
		BeforeSensitive: beforeSensitive,
		AfterSensitive:  afterSensitive,
	}
}

// SDKToProtoDecision converts an SDK Decision to a protobuf Decision.
func SDKToProtoDecision(s *sdk.Decision) *pb.Decision {
	if s == nil {
		return nil
	}

	var metadata []byte
	if s.Metadata != nil {
		metadata, _ = json.Marshal(s.Metadata)
	}

	return &pb.Decision{
		Classification: s.Classification,
		Reason:         s.Reason,
		Severity:       int32(s.Severity),
		Metadata:       metadata,
	}
}

// ProtoToSDKDecision converts a protobuf Decision to an SDK Decision.
func ProtoToSDKDecision(p *pb.Decision) *sdk.Decision {
	if p == nil {
		return nil
	}

	var metadata map[string]interface{}
	if len(p.Metadata) > 0 {
		json.Unmarshal(p.Metadata, &metadata)
	}

	return &sdk.Decision{
		Classification: p.Classification,
		Reason:         p.Reason,
		Severity:       int(p.Severity),
		Metadata:       metadata,
	}
}
