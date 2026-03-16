package verify

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/poiesic/modelr/internal/model"
	"gopkg.in/yaml.v3"
)

// VerifiedOutput is the structure written to .verified.yaml files.
type VerifiedOutput struct {
	Model              string               `yaml:"model"`
	VerifiedAt         string               `yaml:"verified_at"`
	Verifications      []Verification       `yaml:"verifications"`
	BehavioralFindings []model.Finding      `yaml:"behavioral_findings"`
	KnownUnknowns      []model.KnownUnknown `yaml:"known_unknowns"`
	Assumptions        []model.Assumption   `yaml:"assumptions"`
	Summary            string               `yaml:"summary"`
}

// WriteVerifiedYAML writes the .verified.yaml output file as a sibling of the input.
func WriteVerifiedYAML(inputPath string, modelName string, result *VerificationResult, validation *model.ValidationResult) error {
	output := VerifiedOutput{
		Model:      modelName,
		VerifiedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:    result.Summary,
	}

	output.Verifications = result.Verifications
	if output.Verifications == nil {
		output.Verifications = []Verification{}
	}

	// Build behavioral findings from failed verifications
	for _, v := range result.Verifications {
		if v.Result == "fail" {
			output.BehavioralFindings = append(output.BehavioralFindings, model.Finding{
				Severity:     "error",
				Relationship: v.Pattern,
				Upstream:     v.Upstream,
				Downstream:   v.Downstream,
				Description:  fmt.Sprintf("behavioral verification failed: %s violated after %d simulations", v.ViolatedInvariant, v.Simulations),
				Kind:         "behavioral",
			})
		}
	}
	if output.BehavioralFindings == nil {
		output.BehavioralFindings = []model.Finding{}
	}

	if validation != nil {
		output.KnownUnknowns = validation.KnownUnknowns
		output.Assumptions = validation.Assumptions
	}
	if output.KnownUnknowns == nil {
		output.KnownUnknowns = []model.KnownUnknown{}
	}
	if output.Assumptions == nil {
		output.Assumptions = []model.Assumption{}
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshaling verified output: %w", err)
	}

	outputPath := VerifiedOutputPath(inputPath)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	return nil
}

// VerifiedOutputPath returns the .verified.yaml path for a given input model path.
func VerifiedOutputPath(inputPath string) string {
	if strings.HasSuffix(inputPath, ".model.yaml") {
		return strings.TrimSuffix(inputPath, ".model.yaml") + ".verified.yaml"
	}
	return inputPath + ".verified.yaml"
}
