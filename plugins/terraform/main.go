// Package main provides the bundled Terraform plugin for tfclassify.
package main

import (
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

func main() {
	sdkplugin.Serve(&sdkplugin.ServeOpts{
		PluginSet: NewTerraformPluginSet(),
	})
}

// ServeBundled is called when tfclassify runs with --act-as-bundled-plugin.
// This is the entry point when the host binary acts as the bundled plugin.
func ServeBundled() {
	sdkplugin.Serve(&sdkplugin.ServeOpts{
		PluginSet: NewTerraformPluginSet(),
	})
}
