package init

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skills/modelr-model/SKILL.md
var modelSkill string

//go:embed skills/modelr-outage-report/SKILL.md
var outageReportSkill string

// ModelSkillContent returns the embedded model skill markdown.
func ModelSkillContent() string {
	return modelSkill
}

// OutageReportSkillContent returns the embedded outage report skill markdown.
func OutageReportSkillContent() string {
	return outageReportSkill
}

// MCPConfigContent returns the .claude/mcp.json content for modelr.
func MCPConfigContent() string {
	config := map[string]any{
		"mcpServers": map[string]any{
			"modelr": map[string]any{
				"command": "modelr",
				"args":    []string{"mcp"},
			},
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	return string(data)
}

// ScaffoldResult reports what happened during scaffolding.
type ScaffoldResult struct {
	Created []string
	Skipped []string
}

// Scaffold creates Claude Code integration files in the target directory.
// Existing files are skipped, never overwritten.
func Scaffold(targetDir string) (*ScaffoldResult, error) {
	result := &ScaffoldResult{}

	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(".claude", "skills", "modelr-model", "SKILL.md"), modelSkill},
		{filepath.Join(".claude", "skills", "modelr-outage-report", "SKILL.md"), outageReportSkill},
		{filepath.Join(".claude", "mcp.json"), MCPConfigContent()},
	}

	for _, f := range files {
		fullPath := filepath.Join(targetDir, f.path)

		if _, err := os.Stat(fullPath); err == nil {
			result.Skipped = append(result.Skipped, f.path)
			continue
		}

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(f.content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", f.path, err)
		}

		result.Created = append(result.Created, f.path)
	}

	return result, nil
}
