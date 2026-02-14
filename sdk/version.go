// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

// SDKVersion is the version of the SDK embedded in every plugin binary.
// The host checks this against its SDKVersionConstraints to ensure compatibility.
// Plugin binaries built against this SDK will report this version to the host.
const SDKVersion = "0.0.1"
