// Package plugin provides the entry point for tfclassify plugins.
package plugin

import (
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jokarl/tfclassify/sdk"
	"google.golang.org/grpc"
)

// HandshakeConfig is used for initial plugin-host verification.
// Both the host and plugin must agree on these values for communication to proceed.
var HandshakeConfig = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TFCLASSIFY_PLUGIN",
	MagicCookieValue: "8kP2mXqR7vNwJ4tL9bYcF6gHsE3dA5uZoK1iWxCjT0lDfBnMrQpSaUhVeOyGI",
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
// This creates the PluginServiceServer that handles host requests.
func (p *GRPCPluginImpl) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	// The plugin service server handles ApplyConfig and Analyze calls from the host.
	// It uses the broker to establish bidirectional communication with the Runner service.
	RegisterPluginServiceServer(s, NewPluginServiceServer(p.Impl, broker))
	return nil
}

// GRPCClient creates a client that can communicate with the plugin.
// This is called by the host to get a client for making RPC calls.
func (p *GRPCPluginImpl) GRPCClient(ctx interface{}, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Return a client that wraps the gRPC connection for host-side use
	return NewPluginClient(c, broker), nil
}

// PluginClient wraps a gRPC client connection for host-side plugin communication.
type PluginClient struct {
	conn   *grpc.ClientConn
	broker *goplugin.GRPCBroker
}

// NewPluginClient creates a new plugin client.
func NewPluginClient(conn *grpc.ClientConn, broker *goplugin.GRPCBroker) *PluginClient {
	return &PluginClient{
		conn:   conn,
		broker: broker,
	}
}

// Conn returns the underlying gRPC client connection.
func (c *PluginClient) Conn() *grpc.ClientConn {
	return c.conn
}

// Broker returns the GRPCBroker for bidirectional communication.
func (c *PluginClient) Broker() *goplugin.GRPCBroker {
	return c.broker
}
