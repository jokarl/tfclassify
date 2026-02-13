// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

func main() {
	sdkplugin.Serve(&sdkplugin.ServeOpts{
		PluginSet: NewAzurermPluginSet(),
	})
}
