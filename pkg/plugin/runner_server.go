// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jokarl/tfclassify/pkg/classify"
	"github.com/jokarl/tfclassify/sdk"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
	"google.golang.org/grpc"
)

// RunnerServiceServer implements the gRPC server for RunnerService.
// This is the server side that runs in the host process and handles
// callbacks from plugins during analysis.
type RunnerServiceServer struct {
	runner *Runner
}

// NewRunnerServiceServer creates a new runner service server.
func NewRunnerServiceServer(runner *Runner) *RunnerServiceServer {
	return &RunnerServiceServer{runner: runner}
}

// GetResourceChanges handles the GetResourceChanges RPC from plugins.
func (s *RunnerServiceServer) GetResourceChanges(ctx context.Context, req *sdkplugin.GetResourceChangesRequest) (*sdkplugin.GetResourceChangesResponse, error) {
	changes, err := s.runner.GetResourceChanges(req.Patterns)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource changes: %w", err)
	}

	protoChanges := make([]*sdkplugin.ResourceChange, len(changes))
	for i, c := range changes {
		protoChanges[i] = sdkToProtoResourceChange(c)
	}

	return &sdkplugin.GetResourceChangesResponse{Changes: protoChanges}, nil
}

// GetResourceChange handles the GetResourceChange RPC from plugins.
func (s *RunnerServiceServer) GetResourceChange(ctx context.Context, req *sdkplugin.GetResourceChangeRequest) (*sdkplugin.GetResourceChangeResponse, error) {
	change, err := s.runner.GetResourceChange(req.Address)
	if err != nil {
		return nil, err
	}

	return &sdkplugin.GetResourceChangeResponse{Change: sdkToProtoResourceChange(change)}, nil
}

// EmitDecision handles the EmitDecision RPC from plugins.
func (s *RunnerServiceServer) EmitDecision(ctx context.Context, req *sdkplugin.EmitDecisionRequest) (*sdkplugin.EmitDecisionResponse, error) {
	// Convert proto types to SDK types
	change := protoToSDKResourceChange(req.Change)
	decision := protoToSDKDecision(req.Decision)

	// Create a minimal analyzer wrapper for the runner call
	analyzer := &pluginAnalyzerWrapper{name: req.AnalyzerName}

	if err := s.runner.EmitDecision(analyzer, change, decision); err != nil {
		return nil, fmt.Errorf("failed to emit decision: %w", err)
	}

	return &sdkplugin.EmitDecisionResponse{}, nil
}

// pluginAnalyzerWrapper wraps an analyzer name from a plugin call.
type pluginAnalyzerWrapper struct {
	name string
}

func (a *pluginAnalyzerWrapper) Name() string               { return a.name }
func (a *pluginAnalyzerWrapper) Enabled() bool              { return true }
func (a *pluginAnalyzerWrapper) ResourcePatterns() []string { return nil }
func (a *pluginAnalyzerWrapper) Analyze(sdk.Runner) error   { return nil }

// RunnerServiceHandler defines the interface for the RunnerService.
type RunnerServiceHandler interface {
	GetResourceChanges(context.Context, *sdkplugin.GetResourceChangesRequest) (*sdkplugin.GetResourceChangesResponse, error)
	GetResourceChange(context.Context, *sdkplugin.GetResourceChangeRequest) (*sdkplugin.GetResourceChangeResponse, error)
	EmitDecision(context.Context, *sdkplugin.EmitDecisionRequest) (*sdkplugin.EmitDecisionResponse, error)
}

// RegisterRunnerServiceServer registers the RunnerServiceServer with a gRPC server.
func RegisterRunnerServiceServer(s *grpc.Server, srv RunnerServiceHandler) {
	s.RegisterService(&runnerServiceDesc, srv)
}

// Service description for RunnerService
var runnerServiceDesc = grpc.ServiceDesc{
	ServiceName: "tfclassify.RunnerService",
	HandlerType: (*RunnerServiceHandler)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetResourceChanges",
			Handler:    runnerServiceGetResourceChangesHandler,
		},
		{
			MethodName: "GetResourceChange",
			Handler:    runnerServiceGetResourceChangeHandler,
		},
		{
			MethodName: "EmitDecision",
			Handler:    runnerServiceEmitDecisionHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "tfclassify.proto",
}

func runnerServiceGetResourceChangesHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(sdkplugin.GetResourceChangesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RunnerServiceHandler).GetResourceChanges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.RunnerService/GetResourceChanges",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RunnerServiceHandler).GetResourceChanges(ctx, req.(*sdkplugin.GetResourceChangesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func runnerServiceGetResourceChangeHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(sdkplugin.GetResourceChangeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RunnerServiceHandler).GetResourceChange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.RunnerService/GetResourceChange",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RunnerServiceHandler).GetResourceChange(ctx, req.(*sdkplugin.GetResourceChangeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func runnerServiceEmitDecisionHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(sdkplugin.EmitDecisionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RunnerServiceHandler).EmitDecision(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.RunnerService/EmitDecision",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RunnerServiceHandler).EmitDecision(ctx, req.(*sdkplugin.EmitDecisionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Conversion functions

func sdkToProtoResourceChange(s *sdk.ResourceChange) *sdkplugin.ResourceChange {
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

	return &sdkplugin.ResourceChange{
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

func protoToSDKResourceChange(p *sdkplugin.ResourceChange) *sdk.ResourceChange {
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

func protoToSDKDecision(p *sdkplugin.Decision) *sdk.Decision {
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

// EmitDecisionWithMetadata records a plugin decision, properly handling
// metadata-only decisions (empty Classification) per CR-0006.
func (r *Runner) EmitDecisionWithMetadata(analyzerName string, change *sdk.ResourceChange, decision *sdk.Decision) error {
	r.host.mu.Lock()
	defer r.host.mu.Unlock()

	rd := classify.ResourceDecision{
		Address:      change.Address,
		ResourceType: change.Type,
		Actions:      change.Actions,
		MatchedRule:  fmt.Sprintf("plugin: %s - %s", analyzerName, decision.Reason),
	}

	// If Classification is empty, this is metadata-only augmentation
	// The core classification will be preserved and plugin reason/severity/metadata appended
	if decision.Classification != "" {
		rd.Classification = decision.Classification
	}

	// Store severity and reason for aggregation
	if decision.Severity > 0 {
		if rd.MatchedRule != "" {
			rd.MatchedRule = fmt.Sprintf("%s (severity: %d)", rd.MatchedRule, decision.Severity)
		}
	}

	r.host.decisions = append(r.host.decisions, rd)
	return nil
}
