// Package plugin provides the entry point for tfclassify plugins.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jokarl/tfclassify/sdk"
	"google.golang.org/grpc"
)

// PluginServiceServer implements the gRPC server for PluginService.
// This is the server side that runs inside the plugin process.
type PluginServiceServer struct {
	impl   sdk.PluginSet
	broker *goplugin.GRPCBroker

	// analyzers caches analyzer lookup by name
	analyzers map[string]sdk.Analyzer
}

// NewPluginServiceServer creates a new plugin service server.
func NewPluginServiceServer(impl sdk.PluginSet, broker *goplugin.GRPCBroker) *PluginServiceServer {
	// Build analyzer cache
	analyzers := make(map[string]sdk.Analyzer)
	if builtinSet, ok := impl.(*sdk.BuiltinPluginSet); ok {
		for _, a := range builtinSet.Analyzers {
			analyzers[a.Name()] = a
		}
	}

	return &PluginServiceServer{
		impl:      impl,
		broker:    broker,
		analyzers: analyzers,
	}
}

// GetPluginInfo returns plugin metadata for version negotiation.
func (s *PluginServiceServer) GetPluginInfo(ctx context.Context, req *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	return &GetPluginInfoResponse{
		Name:                  s.impl.PluginSetName(),
		Version:               s.impl.PluginSetVersion(),
		SDKVersion:            sdk.SDKVersion,
		HostVersionConstraint: s.impl.VersionConstraint(),
	}, nil
}

// GetConfigSchema returns the configuration schema for validation.
func (s *PluginServiceServer) GetConfigSchema(ctx context.Context, req *GetConfigSchemaRequest) (*GetConfigSchemaResponse, error) {
	schema := s.impl.ConfigSchema()
	if schema == nil {
		return &GetConfigSchemaResponse{Attributes: nil}, nil
	}

	attrs := make([]*ConfigAttribute, len(schema.Attributes))
	for i, a := range schema.Attributes {
		attrs[i] = &ConfigAttribute{
			Name:     a.Name,
			Type:     a.Type,
			Required: a.Required,
		}
	}

	return &GetConfigSchemaResponse{Attributes: attrs}, nil
}

// ApplyConfig applies the plugin-specific configuration.
func (s *PluginServiceServer) ApplyConfig(ctx context.Context, req *ApplyConfigRequest) (*ApplyConfigResponse, error) {
	// For now, configuration is passed but not deeply processed.
	// Individual plugins can override this via their PluginSet implementation.
	return &ApplyConfigResponse{}, nil
}

// Analyze runs all enabled analyzers in the plugin.
func (s *PluginServiceServer) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	// Create a connection back to the host's Runner service
	conn, err := s.broker.Dial(req.BrokerID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runner: %w", err)
	}
	defer conn.Close()

	// Create a Runner client that calls back to the host
	runner := NewRunnerClient(conn)

	// Get the builtin plugin set to access analyzers
	builtinSet, ok := s.impl.(*sdk.BuiltinPluginSet)
	if !ok {
		// For custom implementations, they need to provide their own analysis
		return &AnalyzeResponse{}, nil
	}

	// Run each enabled analyzer
	for _, analyzer := range builtinSet.Analyzers {
		if !analyzer.Enabled() {
			continue
		}

		if err := analyzer.Analyze(runner); err != nil {
			// Log error but continue with other analyzers
			continue
		}
	}

	return &AnalyzeResponse{}, nil
}

// PluginServiceClient implements the gRPC client for calling PluginService.
// This is the client side that runs in the host process.
type PluginServiceClient struct {
	client *grpc.ClientConn
	broker *goplugin.GRPCBroker
}

// NewPluginServiceClient creates a new plugin service client.
func NewPluginServiceClient(conn *grpc.ClientConn, broker *goplugin.GRPCBroker) *PluginServiceClient {
	return &PluginServiceClient{
		client: conn,
		broker: broker,
	}
}

// RunnerClient implements sdk.Runner by calling back to the host via gRPC.
type RunnerClient struct {
	conn *grpc.ClientConn
}

// NewRunnerClient creates a new runner client.
func NewRunnerClient(conn *grpc.ClientConn) *RunnerClient {
	return &RunnerClient{conn: conn}
}

// GetResourceChanges returns resource changes matching the given patterns.
func (r *RunnerClient) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	req := &GetResourceChangesRequest{Patterns: patterns}

	var resp GetResourceChangesResponse
	ctx := context.Background()

	err := r.conn.Invoke(ctx, "/tfclassify.RunnerService/GetResourceChanges", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("GetResourceChanges RPC failed: %w", err)
	}

	changes := make([]*sdk.ResourceChange, len(resp.Changes))
	for i, c := range resp.Changes {
		changes[i] = protoToSDKResourceChange(c)
	}

	return changes, nil
}

// GetResourceChange returns a specific resource change by address.
func (r *RunnerClient) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	req := &GetResourceChangeRequest{Address: address}

	var resp GetResourceChangeResponse
	ctx := context.Background()

	err := r.conn.Invoke(ctx, "/tfclassify.RunnerService/GetResourceChange", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("GetResourceChange RPC failed: %w", err)
	}

	if resp.Change == nil {
		return nil, fmt.Errorf("resource not found: %s", address)
	}

	return protoToSDKResourceChange(resp.Change), nil
}

// EmitDecision records a classification decision for a resource.
func (r *RunnerClient) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	protoChange := sdkToProtoResourceChange(change)
	protoDecision := sdkToProtoDecision(decision)

	req := &EmitDecisionRequest{
		AnalyzerName: analyzer.Name(),
		Change:       protoChange,
		Decision:     protoDecision,
	}

	var resp EmitDecisionResponse
	ctx := context.Background()

	err := r.conn.Invoke(ctx, "/tfclassify.RunnerService/EmitDecision", req, &resp)
	if err != nil {
		return fmt.Errorf("EmitDecision RPC failed: %w", err)
	}

	return nil
}

// Proto message types (matching proto/tfclassify.proto)

// GetPluginInfoRequest requests plugin metadata.
type GetPluginInfoRequest struct{}

// GetPluginInfoResponse contains plugin metadata.
type GetPluginInfoResponse struct {
	Name                  string
	Version               string
	SDKVersion            string
	HostVersionConstraint string
}

// GetConfigSchemaRequest requests the config schema.
type GetConfigSchemaRequest struct{}

// GetConfigSchemaResponse contains the config schema.
type GetConfigSchemaResponse struct {
	Attributes []*ConfigAttribute
}

// ConfigAttribute describes a config attribute.
type ConfigAttribute struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// ApplyConfigRequest contains plugin configuration.
type ApplyConfigRequest struct {
	Config []byte
}

// ApplyConfigResponse is empty on success.
type ApplyConfigResponse struct{}

// AnalyzeRequest starts the analysis.
type AnalyzeRequest struct {
	BrokerID uint32
}

// AnalyzeResponse indicates analysis completion.
type AnalyzeResponse struct{}

// GetResourceChangesRequest queries for matching resources.
type GetResourceChangesRequest struct {
	Patterns []string
}

// GetResourceChangesResponse contains matching resources.
type GetResourceChangesResponse struct {
	Changes []*ResourceChange
}

// GetResourceChangeRequest queries for a specific resource.
type GetResourceChangeRequest struct {
	Address string
}

// GetResourceChangeResponse contains the resource.
type GetResourceChangeResponse struct {
	Change *ResourceChange
}

// EmitDecisionRequest records a decision.
type EmitDecisionRequest struct {
	AnalyzerName string
	Change       *ResourceChange
	Decision     *Decision
}

// EmitDecisionResponse is empty on success.
type EmitDecisionResponse struct{}

// ResourceChange represents a resource change in proto format.
type ResourceChange struct {
	Address         string
	Type            string
	ProviderName    string
	Mode            string
	Actions         []string
	Before          []byte // JSON-encoded
	After           []byte // JSON-encoded
	BeforeSensitive []byte // JSON-encoded
	AfterSensitive  []byte // JSON-encoded
}

// Decision represents a classification decision in proto format.
type Decision struct {
	Classification string
	Reason         string
	Severity       int32
	Metadata       []byte // JSON-encoded
}

// Conversion functions

func protoToSDKResourceChange(p *ResourceChange) *sdk.ResourceChange {
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

func sdkToProtoResourceChange(s *sdk.ResourceChange) *ResourceChange {
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

	return &ResourceChange{
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

func sdkToProtoDecision(s *sdk.Decision) *Decision {
	if s == nil {
		return nil
	}

	var metadata []byte
	if s.Metadata != nil {
		metadata, _ = json.Marshal(s.Metadata)
	}

	return &Decision{
		Classification: s.Classification,
		Reason:         s.Reason,
		Severity:       int32(s.Severity),
		Metadata:       metadata,
	}
}

// PluginServiceHandler defines the interface for handling plugin service RPCs.
// This is required by gRPC's service registration.
type PluginServiceHandler interface {
	GetPluginInfo(context.Context, *GetPluginInfoRequest) (*GetPluginInfoResponse, error)
	GetConfigSchema(context.Context, *GetConfigSchemaRequest) (*GetConfigSchemaResponse, error)
	ApplyConfig(context.Context, *ApplyConfigRequest) (*ApplyConfigResponse, error)
	Analyze(context.Context, *AnalyzeRequest) (*AnalyzeResponse, error)
}

// RegisterPluginServiceServer registers the PluginServiceServer with a gRPC server.
// This is called by the GRPCPluginImpl.GRPCServer method.
func RegisterPluginServiceServer(s *grpc.Server, srv PluginServiceHandler) {
	// Register the service description with the gRPC server.
	// This uses manual registration since we're not using protoc-generated code.
	s.RegisterService(&pluginServiceDesc, srv)
}

// Service description for PluginService
var pluginServiceDesc = grpc.ServiceDesc{
	ServiceName: "tfclassify.PluginService",
	HandlerType: (*PluginServiceHandler)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetPluginInfo",
			Handler:    pluginServiceGetPluginInfoHandler,
		},
		{
			MethodName: "GetConfigSchema",
			Handler:    pluginServiceGetConfigSchemaHandler,
		},
		{
			MethodName: "ApplyConfig",
			Handler:    pluginServiceApplyConfigHandler,
		},
		{
			MethodName: "Analyze",
			Handler:    pluginServiceAnalyzeHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "tfclassify.proto",
}

func pluginServiceGetPluginInfoHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPluginInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*PluginServiceServer).GetPluginInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.PluginService/GetPluginInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(*PluginServiceServer).GetPluginInfo(ctx, req.(*GetPluginInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func pluginServiceGetConfigSchemaHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetConfigSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*PluginServiceServer).GetConfigSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.PluginService/GetConfigSchema",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(*PluginServiceServer).GetConfigSchema(ctx, req.(*GetConfigSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func pluginServiceApplyConfigHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ApplyConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*PluginServiceServer).ApplyConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.PluginService/ApplyConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(*PluginServiceServer).ApplyConfig(ctx, req.(*ApplyConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func pluginServiceAnalyzeHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AnalyzeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*PluginServiceServer).Analyze(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tfclassify.PluginService/Analyze",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(*PluginServiceServer).Analyze(ctx, req.(*AnalyzeRequest))
	}
	return interceptor(ctx, in, info, handler)
}
