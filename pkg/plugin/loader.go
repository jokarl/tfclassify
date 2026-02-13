// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"context"
	"encoding/json"
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
)

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

	h.clients[name] = client
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

// runPluginAnalysis runs a single plugin's analysis.
func (h *Host) runPluginAnalysis(ctx context.Context, name string, plugin *DiscoveredPlugin) error {
	// For the bundled plugin, we handle it specially
	// The full gRPC implementation would go here

	// For now, this is a placeholder that will be completed when
	// the plugin binary is built with gRPC support

	_ = ctx
	_ = name
	_ = plugin

	return nil
}

// Shutdown stops all plugin processes.
func (h *Host) Shutdown() {
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

// toProtoResourceChange converts a plan.ResourceChange to proto format.
func toProtoResourceChange(change *plan.ResourceChange) ([]byte, []byte, []byte, []byte, error) {
	var before, after, beforeSens, afterSens []byte
	var err error

	if change.Before != nil {
		before, err = json.Marshal(change.Before)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	if change.After != nil {
		after, err = json.Marshal(change.After)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	if change.BeforeSensitive != nil {
		beforeSens, err = json.Marshal(change.BeforeSensitive)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	if change.AfterSensitive != nil {
		afterSens, err = json.Marshal(change.AfterSensitive)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	return before, after, beforeSens, afterSens, nil
}
