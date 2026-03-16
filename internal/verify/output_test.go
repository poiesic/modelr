package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestVerifiedYAMLStructure(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "test.model.yaml")

	result := &VerificationResult{
		Verifications: []Verification{
			{Upstream: "api", Downstream: "db", Pattern: "finite_resource", Result: "pass",
				Simulations: 312, Failures: 0, Confidence: 0.99, FailureRateBound: 0.001},
		},
		Summary: "Accepted: all 1 verifications passed.",
	}

	err := WriteVerifiedYAML(inputPath, "test-system", result, &model.ValidationResult{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "test.verified.yaml"))
	require.NoError(t, err)

	var output VerifiedOutput
	require.NoError(t, yaml.Unmarshal(data, &output))

	assert.Equal(t, "test-system", output.Model)
	assert.NotEmpty(t, output.VerifiedAt)
	assert.Len(t, output.Verifications, 1)
	assert.NotNil(t, output.BehavioralFindings)
	assert.NotNil(t, output.KnownUnknowns)
	assert.NotNil(t, output.Assumptions)
	assert.NotEmpty(t, output.Summary)
}

func TestVerifiedYAMLTimestamp(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "test.model.yaml")

	result := &VerificationResult{Summary: "ok"}
	err := WriteVerifiedYAML(inputPath, "test", result, &model.ValidationResult{})
	require.NoError(t, err)

	data, _ := os.ReadFile(filepath.Join(dir, "test.verified.yaml"))
	var output VerifiedOutput
	yaml.Unmarshal(data, &output)

	assert.Contains(t, output.VerifiedAt, "T")
	assert.Contains(t, output.VerifiedAt, "Z")
}

func TestVerifiedYAMLPassingVerification(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "test.model.yaml")

	result := &VerificationResult{
		Verifications: []Verification{
			{Result: "pass", Simulations: 500, Failures: 0},
		},
	}
	WriteVerifiedYAML(inputPath, "test", result, &model.ValidationResult{})

	data, _ := os.ReadFile(filepath.Join(dir, "test.verified.yaml"))
	var output VerifiedOutput
	yaml.Unmarshal(data, &output)

	assert.Equal(t, "pass", output.Verifications[0].Result)
	assert.Equal(t, 0, output.Verifications[0].Failures)
}

func TestVerifiedYAMLFailingVerification(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "test.model.yaml")

	result := &VerificationResult{
		Verifications: []Verification{
			{
				Result: "fail", Simulations: 23, Failures: 4,
				ViolatedInvariant: "Conservation",
				Pattern:           "finite_resource",
				Upstream:          "api",
				Downstream:        "db",
				MinimalFailure: []Step{
					{Rule: "RequestArrives", Instance: 0, State: map[string]int{"used_resources": 1}},
					{Rule: "RequestArrives", Instance: 1, State: map[string]int{"used_resources": 2}},
				},
			},
		},
	}
	WriteVerifiedYAML(inputPath, "test", result, &model.ValidationResult{})

	data, _ := os.ReadFile(filepath.Join(dir, "test.verified.yaml"))
	var output VerifiedOutput
	yaml.Unmarshal(data, &output)

	assert.Equal(t, "fail", output.Verifications[0].Result)
	assert.NotEmpty(t, output.Verifications[0].MinimalFailure)
	assert.Equal(t, "Conservation", output.Verifications[0].ViolatedInvariant)
	assert.NotEmpty(t, output.BehavioralFindings)
}
