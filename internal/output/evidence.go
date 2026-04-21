package output

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// EvidenceArtifact is the JSON structure written as the evidence file.
type EvidenceArtifact struct {
	SchemaVersion      string              `json:"schema_version"`
	Timestamp          string              `json:"timestamp"`
	TfclassifyVersion  string              `json:"tfclassify_version"`
	PlanFileHash       string              `json:"plan_file_hash"`
	ConfigFileHash     string              `json:"config_file_hash"`
	Overall            string              `json:"overall"`
	OverallDescription string              `json:"overall_description,omitempty"`
	ExitCode           int                 `json:"exit_code"`
	NoChanges          bool                `json:"no_changes"`
	Resources          []EvidenceResource  `json:"resources,omitempty"`
	Trace              []EvidenceTrace     `json:"trace,omitempty"`
	Signature          string              `json:"signature,omitempty"`
	SignedContentHash  string              `json:"signed_content_hash,omitempty"`
}

// EvidenceResource represents a single resource in the evidence artifact.
type EvidenceResource struct {
	Address                   string                 `json:"address"`
	Type                      string                 `json:"type"`
	Actions                   []string               `json:"actions"`
	Classification            string                 `json:"classification"`
	ClassificationDescription string                 `json:"classification_description,omitempty"`
	MatchedRules              []string               `json:"matched_rules"`
	OriginalActions           []string               `json:"original_actions,omitempty"`
	IgnoredAttributes         []string               `json:"ignored_attributes,omitempty"`
	IgnoreRuleMatches         []plan.IgnoreRuleMatch `json:"ignore_rule_matches,omitempty"`
}

// EvidenceTrace represents a single trace entry in the evidence artifact.
type EvidenceTrace struct {
	Address        string `json:"address"`
	Classification string `json:"classification"`
	Source         string `json:"source"`
	Rule           string `json:"rule"`
	Result         string `json:"result"`
	Reason         string `json:"reason,omitempty"`
}

// EvidenceOptions configures how the evidence artifact is built.
type EvidenceOptions struct {
	Version          string
	Timestamp        string
	PlanFilePath     string
	ConfigFilePath   string
	IncludeResources bool
	IncludeTrace     bool
	SigningKeyPath   string
}

// BuildEvidence creates an evidence artifact from classification results.
func BuildEvidence(result *classify.Result, explainResult *classify.ExplainResult, opts EvidenceOptions) (*EvidenceArtifact, error) {
	planHash, err := hashFile(opts.PlanFilePath)
	if err != nil {
		return nil, fmt.Errorf("hashing plan file: %w", err)
	}

	configHash, err := hashFile(opts.ConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("hashing config file: %w", err)
	}

	artifact := &EvidenceArtifact{
		SchemaVersion:      "1.0",
		Timestamp:          opts.Timestamp,
		TfclassifyVersion:  opts.Version,
		PlanFileHash:       planHash,
		ConfigFileHash:     configHash,
		Overall:            result.Overall,
		OverallDescription: result.OverallDescription,
		ExitCode:           result.OverallExitCode,
		NoChanges:          result.NoChanges,
	}

	if opts.IncludeResources {
		resources := make([]EvidenceResource, 0, len(result.ResourceDecisions))
		for _, d := range result.ResourceDecisions {
			resources = append(resources, EvidenceResource{
				Address:                   d.Address,
				Type:                      d.ResourceType,
				Actions:                   d.Actions,
				Classification:            d.Classification,
				ClassificationDescription: d.ClassificationDescription,
				MatchedRules:              d.MatchedRules,
				OriginalActions:           d.OriginalActions,
				IgnoredAttributes:         d.IgnoredAttributes,
				IgnoreRuleMatches:         d.IgnoreRuleMatches,
			})
		}
		artifact.Resources = resources
	}

	if opts.IncludeTrace && explainResult != nil {
		var traces []EvidenceTrace
		for _, explanation := range explainResult.Resources {
			for _, entry := range explanation.Trace {
				resultStr := "skip"
				if entry.Result == classify.TraceMatch {
					resultStr = "match"
				}
				traces = append(traces, EvidenceTrace{
					Address:        explanation.Address,
					Classification: entry.Classification,
					Source:         entry.Source,
					Rule:           entry.Rule,
					Result:         resultStr,
					Reason:         entry.Reason,
				})
			}
		}
		artifact.Trace = traces
	}

	return artifact, nil
}

// SignEvidence signs the evidence artifact using an Ed25519 private key.
// It modifies the artifact in place, adding Signature and SignedContentHash fields.
func SignEvidence(artifact *EvidenceArtifact, keyPath string) error {
	keyPath = expandEnvVar(keyPath)

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("reading signing key: %w", err)
	}

	privKey, err := parseEd25519PrivateKey(keyPEM)
	if err != nil {
		return fmt.Errorf("parsing signing key: %w", err)
	}

	// Marshal without signature fields to get content to sign
	artifact.Signature = ""
	artifact.SignedContentHash = ""
	contentBytes, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("marshaling evidence for signing: %w", err)
	}

	// Compute SHA-256 of the content
	hash := sha256.Sum256(contentBytes)
	contentHash := fmt.Sprintf("sha256:%x", hash)

	// Sign the hash
	sig := ed25519.Sign(privKey, hash[:])

	artifact.SignedContentHash = contentHash
	artifact.Signature = base64.RawStdEncoding.EncodeToString(sig)

	return nil
}

// VerifyEvidence verifies the signature of an evidence artifact.
// Returns nil if valid, error if invalid.
func VerifyEvidence(artifactJSON []byte, pubKeyPath string) error {
	pubKeyPEM, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	pubKey, err := parseEd25519PublicKey(pubKeyPEM)
	if err != nil {
		return fmt.Errorf("parsing public key: %w", err)
	}

	// Parse the artifact to extract signature fields
	var artifact EvidenceArtifact
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		return fmt.Errorf("parsing evidence artifact: %w", err)
	}

	if artifact.Signature == "" {
		return fmt.Errorf("evidence artifact is not signed")
	}

	sig, err := base64.RawStdEncoding.DecodeString(artifact.Signature)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	// Reconstruct content without signature fields
	artifact.Signature = ""
	artifact.SignedContentHash = ""
	contentBytes, err := json.Marshal(&artifact)
	if err != nil {
		return fmt.Errorf("marshaling evidence for verification: %w", err)
	}

	// Recompute hash
	hash := sha256.Sum256(contentBytes)

	if !ed25519.Verify(pubKey, hash[:], sig) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// WriteEvidence writes the evidence artifact to a file.
func WriteEvidence(artifact *EvidenceArtifact, path string) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling evidence: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// ResolveEvidenceOptions builds EvidenceOptions from config and CLI flags.
func ResolveEvidenceOptions(cfg *config.Config, version, timestamp, planPath, configPath string) EvidenceOptions {
	opts := EvidenceOptions{
		Version:          version,
		Timestamp:        timestamp,
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
		IncludeTrace:     false,
	}

	if cfg.Evidence != nil {
		opts.IncludeResources = cfg.Evidence.ShouldIncludeResources()
		opts.IncludeTrace = cfg.Evidence.IncludeTrace
		opts.SigningKeyPath = cfg.Evidence.SigningKey
	}

	return opts
}

// hashFile computes SHA-256 of a file's raw bytes.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash), nil
}

// expandEnvVar expands environment variable references in a string.
// If the string starts with $, the rest is treated as an env var name.
func expandEnvVar(s string) string {
	if strings.HasPrefix(s, "$") {
		envName := s[1:]
		if val := os.Getenv(envName); val != "" {
			return val
		}
	}
	return s
}

// parseEd25519PrivateKey parses an Ed25519 private key from PEM data.
// Supports both PKCS8 (BEGIN PRIVATE KEY) and raw (BEGIN ED25519 PRIVATE KEY) formats.
func parseEd25519PrivateKey(pemData []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS8 private key: %w", err)
		}
		edKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not Ed25519 (got %T)", key)
		}
		return edKey, nil
	case "ED25519 PRIVATE KEY":
		if len(block.Bytes) != ed25519.PrivateKeySize {
			// Try PKCS8 format with ED25519 header
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parsing Ed25519 private key: invalid key size %d", len(block.Bytes))
			}
			edKey, ok := key.(ed25519.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("key is not Ed25519")
			}
			return edKey, nil
		}
		return ed25519.PrivateKey(block.Bytes), nil
	default:
		return nil, fmt.Errorf("unsupported PEM type %q (expected PRIVATE KEY or ED25519 PRIVATE KEY)", block.Type)
	}
}

// parseEd25519PublicKey parses an Ed25519 public key from PEM data.
func parseEd25519PublicKey(pemData []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}
		edKey, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("key is not Ed25519 (got %T)", key)
		}
		return edKey, nil
	case "ED25519 PUBLIC KEY":
		if len(block.Bytes) != ed25519.PublicKeySize {
			// Try PKIX format
			key, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parsing Ed25519 public key: invalid key size %d", len(block.Bytes))
			}
			edKey, ok := key.(ed25519.PublicKey)
			if !ok {
				return nil, fmt.Errorf("key is not Ed25519")
			}
			return edKey, nil
		}
		return ed25519.PublicKey(block.Bytes), nil
	default:
		return nil, fmt.Errorf("unsupported PEM type %q (expected PUBLIC KEY or ED25519 PUBLIC KEY)", block.Type)
	}
}
