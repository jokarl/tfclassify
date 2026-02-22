---
status: accepted
date: 2026-02-13
decision-makers: Johan
---

# Plugin SDK Versioning and Protocol Compatibility

## Context and Problem Statement

tfclassify's plugin architecture (ADR-0002) uses hashicorp/go-plugin with gRPC for host-plugin communication. A thorough comparison against TFLint's mature plugin system reveals a significant gap: tfclassify has no mechanism for SDK version negotiation or protocol compatibility checking between the host and plugins.

Currently, our only compatibility mechanism is `ProtocolVersion: 1` in the go-plugin `HandshakeConfig` and a guessable magic cookie value (`"tfclassify"`). If a plugin is built against an incompatible SDK version — e.g., after a breaking change to the proto definitions or Runner interface — the failure mode is an opaque gRPC error rather than a clear "plugin X requires SDK >= Y" message.

As the SDK evolves toward 1.0, plugins built against older SDK versions will inevitably encounter compatibility issues. How should tfclassify handle version negotiation and protocol compatibility between the host and plugins?

## Decision Drivers

* Incompatible plugins must produce clear, actionable error messages — not raw gRPC failures
* Plugin authors need to declare which tfclassify versions their plugin supports
* The host needs to reject plugins built against too-old SDK versions before attempting gRPC communication
* TFLint's proven approach provides a reference implementation familiar to our target audience
* The solution must work with go-plugin's existing `HandshakeConfig` and `GRPCBroker` patterns
* The host needs to validate plugin config schemas before sending `ApplyConfig`, catching HCL errors early

## Considered Options

* Bidirectional version constraints with plugin introspection (TFLint pattern)
* Protocol version only (go-plugin native)
* Capability-based negotiation with feature flags

## Decision Outcome

Chosen option: "Bidirectional version constraints with plugin introspection", because it is the proven approach in the Terraform ecosystem, provides clear error messages for all mismatch scenarios, and enables config schema validation that catches user errors early.

### Consequences

* Good, because incompatible plugins are rejected with clear messages before any analysis runs
* Good, because plugin authors can declare minimum tfclassify version requirements
* Good, because config schema validation catches HCL errors before they reach the plugin
* Good, because TFLint users recognize the pattern and error messages
* Good, because the host can log plugin name/version for debugging in verbose mode
* Bad, because adds 5 new RPCs to the proto definition, increasing protocol surface
* Bad, because SDK interface gains new methods that all plugins must implement (mitigated by `BuiltinPluginSet` defaults)
* Neutral, because go-plugin's `ProtocolVersion` is still used as a coarse compatibility gate alongside the fine-grained checks

### Confirmation

* A plugin built against SDK v0.1.0 connecting to a host requiring >= v0.2.0 produces a clear version mismatch error
* A plugin declaring `tfclassify >= 0.3.0` connecting to host v0.2.0 produces a clear constraint error
* The host logs plugin name and version in verbose mode
* Invalid config for a plugin produces an HCL schema error before `ApplyConfig` is called

## Pros and Cons of the Options

### Bidirectional version constraints with plugin introspection

Following TFLint's pattern, add introspection RPCs to the `PluginService` and version-checking logic to both the SDK and host.

**New PluginService RPCs:**

| RPC | Purpose | TFLint Equivalent |
|-----|---------|-------------------|
| `GetName` | Verify plugin identity | `GetName` |
| `GetVersion` | Log plugin version | `GetVersion` |
| `GetSDKVersion` | Host checks SDK compatibility | `GetSDKVersion` |
| `GetVersionConstraint` | Plugin declares required host version | `GetVersionConstraint` |
| `GetConfigSchema` | Host validates config before `ApplyConfig` | `GetConfigSchema` |

**SDK additions:**

| Addition | Purpose |
|----------|---------|
| `sdk.SDKVersion` constant | Embedded in every plugin binary |
| `PluginSet.VersionConstraint()` method | Plugins declare required tfclassify version |
| `PluginSet.ConfigSchema()` method | Plugins declare their config schema |
| `BuiltinPluginSet` defaults | Empty constraint (any version), nil schema (no validation) |

**Host-side flow:**

```
1. Start plugin process (go-plugin handshake)
2. Call GetName → verify matches config
3. Call GetVersion → log in verbose mode
4. Call GetSDKVersion → check against SDKVersionConstraints
5. Call GetVersionConstraint → check host version satisfies
6. Call GetConfigSchema → validate config block
7. Call ApplyConfig → send validated config
8. Call Analyze → run analysis with Runner callbacks
```

**Version constraint format:** Semver constraints using `hashicorp/go-version` (same library TFLint uses), e.g., `">= 0.1.0"`, `">= 0.2.0, < 1.0.0"`.

**Handshake config hardening:** Replace the guessable magic cookie value `"tfclassify"` with a cryptographically random string, matching TFLint's approach. This prevents accidental execution of non-plugin binaries.

* Good, because proven pattern — TFLint has used this for years with many community plugins
* Good, because clear error messages for every failure mode (SDK too old, host too old, config invalid, wrong binary)
* Good, because `GetConfigSchema` catches config errors before plugin analysis — HCL errors show file/line info
* Good, because plugin name/version available for verbose logging and debugging
* Good, because `hashicorp/go-version` is battle-tested and already in the TFLint ecosystem
* Neutral, because adds 5 RPCs but they're all simple request/response with minimal message types
* Bad, because SDK interface grows — every `PluginSet` implementation must provide version info (mitigated by defaults in `BuiltinPluginSet`)

### Protocol version only (go-plugin native)

Rely solely on go-plugin's built-in `ProtocolVersion` field in `HandshakeConfig`. When the proto definition changes incompatibly, bump the protocol version. Plugins built against the old version fail at the go-plugin handshake with a "incompatible API version" error.

* Good, because zero additional code — go-plugin handles it natively
* Good, because simplest possible approach
* Bad, because coarse-grained — any proto change (even adding a field) forces all plugins to rebuild simultaneously
* Bad, because no way for plugins to declare host version requirements
* Bad, because no config schema validation — config errors surface as plugin-internal failures
* Bad, because error messages are generic ("incompatible API version") with no guidance on which versions are compatible
* Bad, because no plugin name/version in error output — hard to debug multi-plugin setups

### Capability-based negotiation with feature flags

Instead of version numbers, exchange capability flags. The host advertises what it supports (e.g., `["config-schema", "decision-metadata", "resource-patterns"]`), and the plugin advertises what it requires. Incompatibility is detected when a required capability is missing.

* Good, because more flexible than version ranges — capabilities can be added independently
* Good, because fine-grained — a plugin can require only the specific features it uses
* Bad, because no established pattern in the Terraform/HCL ecosystem — unfamiliar to users
* Bad, because capability strings are ad-hoc and error-prone — typos in capability names silently pass
* Bad, because harder to communicate in error messages — "missing capability X" is less intuitive than "requires SDK >= 0.2.0"
* Bad, because doesn't help with config schema validation (orthogonal concern)
* Bad, because more complex to maintain — every new feature needs a capability flag defined

## More Information

### TFLint Reference

TFLint's implementation (validated via DeepWiki for `terraform-linters/tflint` and `terraform-linters/tflint-plugin-sdk`):

- `ProtocolVersion: 11` (incremented over time with breaking changes)
- `MagicCookieValue`: 60-character random string
- `SDKVersionConstraints: ">= 0.16.0"` — host rejects plugins built with older SDK
- `GetVersionConstraint` returns semver constraints using `hashicorp/go-version`
- `GetConfigSchema` returns an `hclext.BodySchema` for config validation
- `ApplyGlobalConfig` sends global settings (disabled_by_default, fix mode) — lower priority for tfclassify since our global settings are host-only

### Comparison Summary

| Aspect | TFLint | tfclassify (current) | tfclassify (proposed) |
|--------|--------|---------------------|----------------------|
| Protocol version | `ProtocolVersion: 11` | `ProtocolVersion: 1` | `ProtocolVersion: 1` (+ fine-grained) |
| Magic cookie | Random 60-char string | `"tfclassify"` | Random string |
| SDK version check | `GetSDKVersion` + constraints | None | `GetSDKVersion` + constraints |
| Host version check | `GetVersionConstraint` | None | `GetVersionConstraint` |
| Config validation | `GetConfigSchema` | None | `GetConfigSchema` |
| Plugin identity | `GetName` + `GetVersion` | None | `GetName` + `GetVersion` |
| PluginService RPCs | 9 | 2 | 7 |

The `ApplyGlobalConfig` RPC (TFLint's 8th) and one metadata RPC are intentionally omitted as they address TFLint-specific concerns (fix mode, `disabled_by_default`) that don't apply to tfclassify's classification model.

Related: [ADR-0002](ADR-0002-grpc-plugin-architecture.md) — gRPC plugin architecture decision.
Related: [CR-0006](../cr/CR-0006-grpc-protocol-and-plugin-host.md) — gRPC protocol implementation (to be updated with new RPCs).
