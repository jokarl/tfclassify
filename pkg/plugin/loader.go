// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gobwas/glob"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jokarl/tfclassify/pkg/classify"
	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/sdk"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
	"google.golang.org/grpc"
)

// SDKVersionConstraints specifies which SDK versions this host is compatible with.
// Plugins built against an SDK version that doesn't satisfy this constraint will be rejected.
const SDKVersionConstraints = ">= 0.1.0"

// Host manages plugin lifecycle and communication.
type Host struct {
	cfg       *config.Config
	plugins   map[string]*DiscoveredPlugin
	clients   map[string]*goplugin.Client
	changes   []plan.ResourceChange
	decisions []classify.ResourceDecision
	mu        sync.Mutex
}

// NewHost creates a new plugin host.
func NewHost(cfg *config.Config) *Host {
	return &Host{
		cfg:       cfg,
		clients:   make(map[string]*goplugin.Client),
		decisions: make([]classify.ResourceDecision, 0),
	}
}

// DiscoverAndStart discovers and starts all enabled plugins.
func (h *Host) DiscoverAndStart(selfPath string) error {
	discovered, err := DiscoverPlugins(h.cfg, selfPath)
	if err != nil {
		return err
	}
	h.plugins = discovered

	for name, plugin := range h.plugins {
		if err := h.startPlugin(name, plugin); err != nil {
			return fmt.Errorf("failed to start plugin %q: %w", name, err)
		}
	}

	return nil
}

// startPlugin starts a single plugin process.
func (h *Host) startPlugin(name string, plugin *DiscoveredPlugin) error {
	var cmd *exec.Cmd
	if plugin.IsBundled {
		cmd = exec.Command(plugin.Path, "--act-as-bundled-plugin")
	} else {
		cmd = exec.Command(plugin.Path)
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: sdkplugin.HandshakeConfig,
		Plugins: map[string]goplugin.Plugin{
			sdkplugin.PluginName: &sdkplugin.GRPCPluginImpl{},
		},
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	h.mu.Lock()
	h.clients[name] = client
	h.mu.Unlock()
	return nil
}

// RunAnalysis runs all plugins against the plan changes.
func (h *Host) RunAnalysis(changes []plan.ResourceChange) ([]classify.ResourceDecision, error) {
	h.changes = changes
	h.decisions = make([]classify.ResourceDecision, 0)

	// Get timeout from config
	timeout := 30 * time.Second
	if h.cfg.Defaults != nil && h.cfg.Defaults.PluginTimeout != "" {
		parsed, err := time.ParseDuration(h.cfg.Defaults.PluginTimeout)
		if err == nil {
			timeout = parsed
		}
	}

	// Run each plugin with timeout
	for name, plugin := range h.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := h.runPluginAnalysis(ctx, name, plugin)
		cancel()

		if err != nil {
			// Log error but continue with other plugins
			fmt.Fprintf(os.Stderr, "Warning: plugin %q failed: %v\n", name, err)
		}
	}

	return h.decisions, nil
}

// runPluginAnalysis runs a single plugin's analysis using gRPC.
func (h *Host) runPluginAnalysis(ctx context.Context, name string, plugin *DiscoveredPlugin) error {
	client, ok := h.clients[name]
	if !ok {
		return fmt.Errorf("plugin client not found: %s", name)
	}

	// Connect to the plugin process via gRPC
	rpcClient, err := client.Client()
	if err != nil {
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	// Get the raw plugin interface
	raw, err := rpcClient.Dispense(sdkplugin.PluginName)
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to PluginClient to access the connection and broker
	pluginClient, ok := raw.(*sdkplugin.PluginClient)
	if !ok {
		return fmt.Errorf("unexpected plugin type: %T", raw)
	}

	conn := pluginClient.Conn()
	broker := pluginClient.Broker()
	if conn == nil || broker == nil {
		return fmt.Errorf("plugin connection not available")
	}

	// Create a Runner for this plugin to call back to
	runner := NewRunner(h)
	runnerServer := NewRunnerServiceServer(runner)

	// Start the Runner server using the broker
	brokerID := broker.NextId()
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		RegisterRunnerServiceServer(s, runnerServer)
		return s
	})

	// First, verify plugin version compatibility (CR-0012)
	pluginInfo, err := h.getPluginInfo(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	if err := VerifyPlugin(name, pluginInfo); err != nil {
		return fmt.Errorf("plugin verification failed: %w", err)
	}

	// Apply configuration to the plugin
	if err := h.applyPluginConfig(ctx, conn, name); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Call Analyze on the plugin, passing the broker ID for callback
	if err := h.callAnalyze(ctx, conn, brokerID); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	return nil
}

// getPluginInfo retrieves plugin metadata for version negotiation.
func (h *Host) getPluginInfo(ctx context.Context, conn interface{}) (*PluginInfo, error) {
	// Type assert to *grpc.ClientConn
	grpcConn, ok := conn.(interface {
		Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...interface{}) error
	})
	if !ok {
		// Return default info for bundled plugins
		return &PluginInfo{
			Name:       "unknown",
			Version:    "0.1.0",
			SDKVersion: sdk.SDKVersion,
		}, nil
	}

	req := &sdkplugin.GetPluginInfoRequest{}
	resp := &sdkplugin.GetPluginInfoResponse{}

	if err := grpcConn.Invoke(ctx, "/tfclassify.PluginService/GetPluginInfo", req, resp); err != nil {
		return nil, err
	}

	return &PluginInfo{
		Name:                  resp.Name,
		Version:               resp.Version,
		SDKVersion:            resp.SDKVersion,
		HostVersionConstraint: resp.HostVersionConstraint,
	}, nil
}

// applyPluginConfig sends configuration to the plugin.
func (h *Host) applyPluginConfig(ctx context.Context, conn interface{}, name string) error {
	// Type assert to *grpc.ClientConn
	grpcConn, ok := conn.(interface {
		Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...interface{}) error
	})
	if !ok {
		return nil // Skip config for non-gRPC connections
	}

	// Note: Plugin configuration is handled via the raw HCL body which is
	// deferred for parsing by the plugin. For now, we send an empty config.
	// The plugin can implement its own config parsing from the HCL body.
	req := &sdkplugin.ApplyConfigRequest{Config: nil}
	resp := &sdkplugin.ApplyConfigResponse{}

	if err := grpcConn.Invoke(ctx, "/tfclassify.PluginService/ApplyConfig", req, resp); err != nil {
		return err
	}

	return nil
}

// callAnalyze invokes the Analyze RPC on the plugin.
func (h *Host) callAnalyze(ctx context.Context, conn interface{}, brokerID uint32) error {
	// Type assert to *grpc.ClientConn
	grpcConn, ok := conn.(interface {
		Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...interface{}) error
	})
	if !ok {
		return nil // Skip analysis for non-gRPC connections
	}

	req := &sdkplugin.AnalyzeRequest{BrokerID: brokerID}
	resp := &sdkplugin.AnalyzeResponse{}

	if err := grpcConn.Invoke(ctx, "/tfclassify.PluginService/Analyze", req, resp); err != nil {
		return err
	}

	return nil
}

// Shutdown stops all plugin processes.
func (h *Host) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, client := range h.clients {
		client.Kill()
	}
}

// Runner implements the SDK Runner interface for plugins to call back to the host.
type Runner struct {
	host *Host
	mu   sync.Mutex
}

// NewRunner creates a new Runner for a host.
func NewRunner(host *Host) *Runner {
	return &Runner{host: host}
}

// GetResourceChanges returns resource changes matching the given patterns.
func (r *Runner) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Compile patterns
	var globs []glob.Glob
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		globs = append(globs, g)
	}

	result := make([]*sdk.ResourceChange, 0)
	for _, change := range r.host.changes {
		if len(globs) == 0 || matchesAny(change.Type, globs) {
			result = append(result, toSDKResourceChange(&change))
		}
	}

	return result, nil
}

// GetResourceChange returns a specific resource change by address.
func (r *Runner) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, change := range r.host.changes {
		if change.Address == address {
			return toSDKResourceChange(&change), nil
		}
	}

	return nil, fmt.Errorf("resource change not found: %s", address)
}

// EmitDecision records a classification decision for a resource.
func (r *Runner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	r.host.mu.Lock()
	defer r.host.mu.Unlock()

	r.host.decisions = append(r.host.decisions, classify.ResourceDecision{
		Address:        change.Address,
		ResourceType:   change.Type,
		Actions:        change.Actions,
		Classification: decision.Classification,
		MatchedRule:    fmt.Sprintf("plugin: %s - %s", analyzer.Name(), decision.Reason),
	})

	return nil
}

// matchesAny returns true if the resource type matches any of the patterns.
func matchesAny(resourceType string, globs []glob.Glob) bool {
	for _, g := range globs {
		if g.Match(resourceType) {
			return true
		}
	}
	return false
}

// toSDKResourceChange converts a plan.ResourceChange to an sdk.ResourceChange.
func toSDKResourceChange(change *plan.ResourceChange) *sdk.ResourceChange {
	return &sdk.ResourceChange{
		Address:         change.Address,
		Type:            change.Type,
		ProviderName:    change.ProviderName,
		Mode:            change.Mode,
		Actions:         change.Actions,
		Before:          change.Before,
		After:           change.After,
		BeforeSensitive: change.BeforeSensitive,
		AfterSensitive:  change.AfterSensitive,
	}
}
