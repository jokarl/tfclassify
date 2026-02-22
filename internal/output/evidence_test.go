package output

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/config"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	return path
}

func generateTestKeyPair(t *testing.T, dir string) (string, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating key pair: %v", err)
	}

	privPKCS8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshaling private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privPKCS8})
	privPath := filepath.Join(dir, "private.pem")
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		t.Fatalf("writing private key: %v", err)
	}

	pubPKIX, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshaling public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubPKIX})
	pubPath := filepath.Join(dir, "public.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		t.Fatalf("writing public key: %v", err)
	}

	return privPath, pubPath
}

func testResult() *classify.Result {
	return &classify.Result{
		Overall:            "critical",
		OverallDescription: "Requires security team approval",
		OverallExitCode:    2,
		NoChanges:          false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:                   "azurerm_role_assignment.admin",
				ResourceType:              "azurerm_role_assignment",
				Actions:                   []string{"delete"},
				Classification:            "critical",
				ClassificationDescription: "Requires security team approval",
				MatchedRules:              []string{"critical rule 1 (resource: *_role_*)"},
			},
		},
	}
}

func TestBuildEvidence_Basic(t *testing.T) {
	dir := t.TempDir()
	planPath := writeTestFile(t, dir, "plan.json", `{"resource_changes":[]}`)
	configPath := writeTestFile(t, dir, "config.hcl", `precedence = ["critical"]`)

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "test-v1",
		Timestamp:        "2026-02-18T19:30:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if artifact.SchemaVersion != "1.0" {
		t.Errorf("expected schema_version 1.0, got %q", artifact.SchemaVersion)
	}
	if artifact.TfclassifyVersion != "test-v1" {
		t.Errorf("expected version test-v1, got %q", artifact.TfclassifyVersion)
	}
	if artifact.Overall != "critical" {
		t.Errorf("expected overall critical, got %q", artifact.Overall)
	}
	if len(artifact.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(artifact.Resources))
	}
	if artifact.Resources[0].Address != "azurerm_role_assignment.admin" {
		t.Errorf("unexpected resource address: %q", artifact.Resources[0].Address)
	}
	if artifact.PlanFileHash == "" {
		t.Error("expected non-empty plan file hash")
	}
	if artifact.ConfigFileHash == "" {
		t.Error("expected non-empty config file hash")
	}
	if len(artifact.Trace) != 0 {
		t.Errorf("expected no trace (not requested), got %d entries", len(artifact.Trace))
	}
}

func TestBuildEvidence_PlanHash(t *testing.T) {
	dir := t.TempDir()
	content := "known content for hashing"
	planPath := writeTestFile(t, dir, "plan.json", content)
	configPath := writeTestFile(t, dir, "config.hcl", "test config")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	// Compute expected hash
	hash := sha256.Sum256([]byte(content))
	expected := "sha256:" + hashHex(hash[:])
	if artifact.PlanFileHash != expected {
		t.Errorf("plan hash mismatch:\n  got:  %s\n  want: %s", artifact.PlanFileHash, expected)
	}
}

func hashHex(b []byte) string {
	const hextable = "0123456789abcdef"
	s := make([]byte, len(b)*2)
	for i, v := range b {
		s[i*2] = hextable[v>>4]
		s[i*2+1] = hextable[v&0x0f]
	}
	return string(s)
}

func TestBuildEvidence_WithoutResources(t *testing.T) {
	dir := t.TempDir()
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: false,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if len(artifact.Resources) != 0 {
		t.Errorf("expected no resources, got %d", len(artifact.Resources))
	}
}

func TestBuildEvidence_WithTrace(t *testing.T) {
	dir := t.TempDir()
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	explainResult := &classify.ExplainResult{
		Resources: []classify.ResourceExplanation{
			{
				Address: "azurerm_role_assignment.admin",
				Trace: []classify.TraceEntry{
					{
						Classification: "critical",
						Source:         "core-rule",
						Rule:           "critical rule 1",
						Result:         classify.TraceMatch,
						Reason:         "",
					},
				},
			},
		},
	}

	artifact, err := BuildEvidence(testResult(), explainResult, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
		IncludeTrace:     true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if len(artifact.Trace) != 1 {
		t.Fatalf("expected 1 trace entry, got %d", len(artifact.Trace))
	}
	if artifact.Trace[0].Source != "core-rule" {
		t.Errorf("unexpected trace source: %q", artifact.Trace[0].Source)
	}
	if artifact.Trace[0].Result != "match" {
		t.Errorf("expected trace result 'match', got %q", artifact.Trace[0].Result)
	}
}

func TestSignEvidence(t *testing.T) {
	dir := t.TempDir()
	privPath, pubPath := generateTestKeyPair(t, dir)
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if err := SignEvidence(artifact, privPath); err != nil {
		t.Fatalf("SignEvidence: %v", err)
	}

	if artifact.Signature == "" {
		t.Error("expected non-empty signature")
	}
	if artifact.SignedContentHash == "" {
		t.Error("expected non-empty signed_content_hash")
	}

	// Write and verify
	artifactPath := filepath.Join(dir, "evidence.json")
	if err := WriteEvidence(artifact, artifactPath); err != nil {
		t.Fatalf("WriteEvidence: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("reading evidence: %v", err)
	}

	if err := VerifyEvidence(data, pubPath); err != nil {
		t.Fatalf("VerifyEvidence should succeed: %v", err)
	}
}

func TestVerifyEvidence_Tampered(t *testing.T) {
	dir := t.TempDir()
	privPath, pubPath := generateTestKeyPair(t, dir)
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if err := SignEvidence(artifact, privPath); err != nil {
		t.Fatalf("SignEvidence: %v", err)
	}

	// Tamper with the artifact
	artifact.Overall = "standard"
	data, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshaling tampered artifact: %v", err)
	}

	if err := VerifyEvidence(data, pubPath); err == nil {
		t.Error("expected verification to fail for tampered artifact")
	}
}

func TestVerifyEvidence_WrongKey(t *testing.T) {
	dir := t.TempDir()
	privPath, _ := generateTestKeyPair(t, dir)

	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if err := SignEvidence(artifact, privPath); err != nil {
		t.Fatalf("SignEvidence: %v", err)
	}

	data, _ := json.Marshal(artifact)

	// Generate a different key pair for verification
	_, pub2, _ := ed25519.GenerateKey(nil)
	pub2PKIX, _ := x509.MarshalPKIXPublicKey(pub2)
	pub2PEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub2PKIX})
	wrongKeyPath := filepath.Join(dir, "wrong_public.pem")
	if err := os.WriteFile(wrongKeyPath, pub2PEM, 0644); err != nil {
		t.Fatalf("writing wrong public key: %v", err)
	}

	if err := VerifyEvidence(data, wrongKeyPath); err == nil {
		t.Error("expected verification to fail with wrong key")
	}
}

func TestVerifyEvidence_Unsigned(t *testing.T) {
	dir := t.TempDir()
	_, pubPath := generateTestKeyPair(t, dir)
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	data, _ := json.Marshal(artifact)

	if err := VerifyEvidence(data, pubPath); err == nil {
		t.Error("expected verification to fail for unsigned artifact")
	}
}

func TestSignEvidence_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	planPath := writeTestFile(t, dir, "plan.json", "plan")
	configPath := writeTestFile(t, dir, "config.hcl", "cfg")

	// Write an RSA-like PEM
	badKeyPath := writeTestFile(t, dir, "bad.pem", "-----BEGIN RSA PRIVATE KEY-----\nnotavalidkey\n-----END RSA PRIVATE KEY-----\n")

	artifact, err := BuildEvidence(testResult(), nil, EvidenceOptions{
		Version:          "v1",
		Timestamp:        "2026-01-01T00:00:00Z",
		PlanFilePath:     planPath,
		ConfigFilePath:   configPath,
		IncludeResources: true,
	})
	if err != nil {
		t.Fatalf("BuildEvidence: %v", err)
	}

	if err := SignEvidence(artifact, badKeyPath); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestResolveEvidenceOptions(t *testing.T) {
	// No evidence config
	cfg := &config.Config{}
	opts := ResolveEvidenceOptions(cfg, "v1", "ts", "plan.json", "config.hcl")
	if !opts.IncludeResources {
		t.Error("expected default include_resources=true")
	}
	if opts.IncludeTrace {
		t.Error("expected default include_trace=false")
	}
	if opts.SigningKeyPath != "" {
		t.Error("expected empty signing key")
	}

	// With evidence config
	boolFalse := false
	cfg.Evidence = &config.EvidenceConfig{
		IncludeTrace:     true,
		IncludeResources: &boolFalse,
		SigningKey:        "/path/to/key.pem",
	}
	opts = ResolveEvidenceOptions(cfg, "v2", "ts2", "plan.json", "config.hcl")
	if opts.IncludeResources {
		t.Error("expected include_resources=false")
	}
	if !opts.IncludeTrace {
		t.Error("expected include_trace=true")
	}
	if opts.SigningKeyPath != "/path/to/key.pem" {
		t.Errorf("unexpected signing key path: %q", opts.SigningKeyPath)
	}
}

func TestExpandEnvVar(t *testing.T) {
	t.Setenv("TFCLASSIFY_TEST_KEY", "/tmp/my-key.pem")

	if got := expandEnvVar("$TFCLASSIFY_TEST_KEY"); got != "/tmp/my-key.pem" {
		t.Errorf("expected /tmp/my-key.pem, got %q", got)
	}

	if got := expandEnvVar("/static/path.pem"); got != "/static/path.pem" {
		t.Errorf("expected unchanged path, got %q", got)
	}

	if got := expandEnvVar("$NONEXISTENT_VAR_12345"); got != "$NONEXISTENT_VAR_12345" {
		t.Errorf("expected unchanged var, got %q", got)
	}
}
