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
	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
	"github.com/jokarl/tfclassify/sdk"
	"github.com/jokarl/tfclassify/sdk/pb"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
	"google.golang.org/grpc"
)

// SDKVersionConstraints specifies which SDK versions this host is compatible with.
// Plugins built against an SDK version that doesn't satisfy this constraint will be rejected.
const SDKVersionConstraints = ">= 0.4.0"

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
		h.startPlugin(name, plugin)
	}

	return nil
}

// startPlugin starts a single plugin process.
func (h *Host) startPlugin(name string, plugin *DiscoveredPlugin) {
	cmd := exec.CommandContext(context.TODO(), plugin.Path)

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
}

// RunAnalysis runs all plugins against the plan changes.
// For each classification that has plugin analyzer config, the plugin is called
// with the classification name and analyzer config. This enables graduated
// thresholds per classification level.
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

	// Collect all classification-scoped plugin configs
	// Map of plugin name -> list of (classification name, analyzer config)
	classificationConfigs := make(map[string][]classificationAnalysisRequest)
	for _, classification := range h.cfg.Classifications {
		for pluginName, analyzerConfig := range classification.PluginAnalyzerConfigs {
			configJSON, err := analyzerConfig.ToJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to serialize analyzer config for %s/%s: %w",
					classification.Name, pluginName, err)
			}
			classificationConfigs[pluginName] = append(classificationConfigs[pluginName],
				classificationAnalysisRequest{
					classificationName: classification.Name,
					analyzerConfig:     configJSON,
				})
		}
	}

	// Run each plugin
	for name := range h.plugins {
		// Get classification-scoped configs for this plugin
		configs := classificationConfigs[name]

		// If no classification-scoped configs, run with empty classification
		if len(configs) == 0 {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := h.runPluginAnalysis(ctx, name, "", nil)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plugin %q failed: %v\n", name, err)
			}
			continue
		}

		// Run analysis for each classification that has config for this plugin
		for _, cfg := range configs {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := h.runPluginAnalysis(ctx, name, cfg.classificationName, cfg.analyzerConfig)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plugin %q failed for classification %q: %v\n",
					name, cfg.classificationName, err)
			}
		}
	}

	return h.decisions, nil
}

// classificationAnalysisRequest holds the classification name and serialized analyzer config
// for a per-classification plugin analysis call.
type classificationAnalysisRequest struct {
	classificationName string
	analyzerConfig     []byte
}

// runPluginAnalysis runs a single plugin's analysis using gRPC.
// If classification is non-empty, it's passed to the plugin for classification-scoped analysis.
func (h *Host) runPluginAnalysis(ctx context.Context, name string, classification string, analyzerConfig []byte) error {
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
		pb.RegisterRunnerServiceServer(s, runnerServer)
		return s
	})

	// Create a typed gRPC client for the plugin service
	pluginSvcClient := pb.NewPluginServiceClient(conn)

	// First, verify plugin version compatibility
	infoResp, err := pluginSvcClient.GetPluginInfo(ctx, &pb.GetPluginInfoRequest{})
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	pluginInfo := &PluginInfo{
		Name:                  infoResp.Name,
		Version:               infoResp.Version,
		SDKVersion:            infoResp.SdkVersion,
		HostVersionConstraint: infoResp.HostVersionConstraint,
	}
	if err := VerifyPlugin(name, pluginInfo); err != nil {
		return fmt.Errorf("plugin verification failed: %w", err)
	}

	// Apply configuration to the plugin
	if _, err := pluginSvcClient.ApplyConfig(ctx, &pb.ApplyConfigRequest{Config: nil}); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Call Analyze on the plugin, passing the broker ID, classification, and analyzer config
	req := &pb.AnalyzeRequest{
		BrokerId:       brokerID,
		Classification: classification,
		AnalyzerConfig: analyzerConfig,
	}
	if _, err := pluginSvcClient.Analyze(ctx, req); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
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
	globs := make([]glob.Glob, 0, len(patterns))
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		globs = append(globs, g)
	}

	result := make([]*sdk.ResourceChange, 0, len(r.host.changes))
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
		MatchedRules:   []string{fmt.Sprintf("plugin: %s - %s", analyzer.Name(), decision.Reason)},
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
