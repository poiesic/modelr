package model

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CheckedOutput is the structure written to .checked.yaml files.
type CheckedOutput struct {
	Model         string         `yaml:"model"`
	CheckedAt     string         `yaml:"checked_at"`
	KnownUnknowns []KnownUnknown `yaml:"known_unknowns"`
	Findings      []Finding      `yaml:"findings"`
	Assumptions   []Assumption   `yaml:"assumptions"`
	Warnings      []string       `yaml:"warnings"`
	Summary       string         `yaml:"summary"`
}

// WriteCheckedYAML writes the .checked.yaml output file as a sibling of the input model file.
func WriteCheckedYAML(inputPath string, modelName string, checkResult *CheckResult, validationResult *ValidationResult) error {
	output := CheckedOutput{
		Model:     modelName,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:   checkResult.Summary,
	}

	// Merge known unknowns from validation and check
	output.KnownUnknowns = append(output.KnownUnknowns, validationResult.KnownUnknowns...)
	output.KnownUnknowns = append(output.KnownUnknowns, checkResult.KnownUnknowns...)
	if output.KnownUnknowns == nil {
		output.KnownUnknowns = []KnownUnknown{}
	}

	output.Findings = checkResult.Findings
	if output.Findings == nil {
		output.Findings = []Finding{}
	}

	output.Assumptions = validationResult.Assumptions
	if output.Assumptions == nil {
		output.Assumptions = []Assumption{}
	}

	// Merge warnings
	output.Warnings = append(output.Warnings, validationResult.Warnings...)
	output.Warnings = append(output.Warnings, checkResult.Warnings...)
	if output.Warnings == nil {
		output.Warnings = []string{}
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshaling checked output: %w", err)
	}

	outputPath := CheckedOutputPath(inputPath)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	return nil
}

// CheckedOutputPath returns the .checked.yaml path for a given input model path.
func CheckedOutputPath(inputPath string) string {
	if strings.HasSuffix(inputPath, ".model.yaml") {
		return strings.TrimSuffix(inputPath, ".model.yaml") + ".checked.yaml"
	}
	return inputPath + ".checked.yaml"
}
