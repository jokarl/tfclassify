// Package plugin provides the entry point for tfclassify plugins.
package plugin

import (
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jokarl/tfclassify/sdk"
)

// HandshakeConfig is used for initial plugin-host verification.
// Both the host and plugin must agree on these values for communication to proceed.
var HandshakeConfig = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TFCLASSIFY_PLUGIN",
	MagicCookieValue: "tfclassify",
}

// PluginName is the name used to identify the plugin in the go-plugin system.
const PluginName = "tfclassify"

// ServeOpts configures the plugin server.
type ServeOpts struct {
	// PluginSet is the plugin implementation to serve.
	PluginSet sdk.PluginSet
}

// Serve starts the plugin gRPC server.
// Call this from the plugin's main() function.
//
// Example:
//
//	func main() {
//	    plugin.Serve(&plugin.ServeOpts{
//	        PluginSet: &MyPluginSet{},
//	    })
//	}
func Serve(opts *ServeOpts) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		Plugins: map[string]goplugin.Plugin{
			PluginName: &GRPCPluginImpl{
				Impl: opts.PluginSet,
			},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}

// GRPCPluginImpl implements the go-plugin Plugin interface for gRPC plugins.
// This is used internally by the SDK and should not be used directly by plugin authors.
type GRPCPluginImpl struct {
	goplugin.Plugin
	Impl sdk.PluginSet
}

// GRPCServer registers the plugin with the gRPC server.
// This will be fully implemented in CR-0006 when the proto definitions are created.
func (p *GRPCPluginImpl) GRPCServer(broker *goplugin.GRPCBroker, s interface{}) error {
	// TODO: Register gRPC server in CR-0006
	return nil
}

// GRPCClient creates a client that can communicate with the plugin.
// This will be fully implemented in CR-0006 when the proto definitions are created.
func (p *GRPCPluginImpl) GRPCClient(ctx interface{}, broker *goplugin.GRPCBroker, c interface{}) (interface{}, error) {
	// TODO: Create gRPC client in CR-0006
	return nil, nil
}
