// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"context"
	"fmt"

	"github.com/jokarl/tfclassify/pkg/classify"
	"github.com/jokarl/tfclassify/sdk"
	"github.com/jokarl/tfclassify/sdk/pb"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

// RunnerServiceServer implements the gRPC server for RunnerService.
// This is the server side that runs in the host process and handles
// callbacks from plugins during analysis.
type RunnerServiceServer struct {
	pb.UnimplementedRunnerServiceServer
	runner *Runner
}

// NewRunnerServiceServer creates a new runner service server.
func NewRunnerServiceServer(runner *Runner) *RunnerServiceServer {
	return &RunnerServiceServer{runner: runner}
}

// GetResourceChanges handles the GetResourceChanges RPC from plugins.
func (s *RunnerServiceServer) GetResourceChanges(ctx context.Context, req *pb.GetResourceChangesRequest) (*pb.GetResourceChangesResponse, error) {
	changes, err := s.runner.GetResourceChanges(req.Patterns)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource changes: %w", err)
	}

	protoChanges := make([]*pb.ResourceChange, len(changes))
	for i, c := range changes {
		protoChanges[i] = sdkplugin.SDKToProtoResourceChange(c)
	}

	return &pb.GetResourceChangesResponse{Changes: protoChanges}, nil
}

// GetResourceChange handles the GetResourceChange RPC from plugins.
func (s *RunnerServiceServer) GetResourceChange(ctx context.Context, req *pb.GetResourceChangeRequest) (*pb.GetResourceChangeResponse, error) {
	change, err := s.runner.GetResourceChange(req.Address)
	if err != nil {
		return nil, err
	}

	return &pb.GetResourceChangeResponse{Change: sdkplugin.SDKToProtoResourceChange(change)}, nil
}

// EmitDecision handles the EmitDecision RPC from plugins.
func (s *RunnerServiceServer) EmitDecision(ctx context.Context, req *pb.EmitDecisionRequest) (*pb.EmitDecisionResponse, error) {
	change := sdkplugin.ProtoToSDKResourceChange(req.Change)
	decision := sdkplugin.ProtoToSDKDecision(req.Decision)

	analyzer := &pluginAnalyzerWrapper{name: req.AnalyzerName}

	if err := s.runner.EmitDecision(analyzer, change, decision); err != nil {
		return nil, fmt.Errorf("failed to emit decision: %w", err)
	}

	return &pb.EmitDecisionResponse{}, nil
}

// pluginAnalyzerWrapper wraps an analyzer name from a plugin call.
type pluginAnalyzerWrapper struct {
	name string
}

func (a *pluginAnalyzerWrapper) Name() string               { return a.name }
func (a *pluginAnalyzerWrapper) Enabled() bool              { return true }
func (a *pluginAnalyzerWrapper) ResourcePatterns() []string { return nil }
func (a *pluginAnalyzerWrapper) Analyze(sdk.Runner) error   { return nil }

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
