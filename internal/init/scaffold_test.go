package init

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Step 4.1: Skill content ---

func TestModelSkillContent(t *testing.T) {
	content := ModelSkillContent()
	assert.Contains(t, content, "list_definitions")
	assert.Contains(t, content, "check_model")
	assert.Contains(t, content, "version:")
	assert.Contains(t, content, "components:")
}

func TestOutageReportSkillContent(t *testing.T) {
	content := OutageReportSkillContent()
	assert.Contains(t, content, "check_model")
	assert.Contains(t, content, "Incident:")
	assert.Contains(t, content, "Severity")
	assert.Contains(t, content, "Timeline")
}

// --- Step 4.2: MCP config ---

func TestMCPConfigContent(t *testing.T) {
	content := MCPConfigContent()
	assert.Contains(t, content, "modelr")
	assert.Contains(t, content, "mcp")
}

func TestMCPConfigParsesAsJSON(t *testing.T) {
	content := MCPConfigContent()
	var parsed map[string]any
	err := json.Unmarshal([]byte(content), &parsed)
	require.NoError(t, err)
	assert.Contains(t, parsed, "mcpServers")
}

// --- Step 4.3: Scaffold writer ---

func TestScaffoldCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	result, err := Scaffold(dir)
	require.NoError(t, err)

	assert.Len(t, result.Created, 3)
	assert.Empty(t, result.Skipped)

	// All files exist
	_, err = os.Stat(filepath.Join(dir, ".claude", "skills", "model.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".claude", "skills", "outage-report.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".claude", "mcp.json"))
	assert.NoError(t, err)
}

func TestScaffoldCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	_, err := Scaffold(dir)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, ".claude", "skills"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestScaffoldSkipsExistingFile(t *testing.T) {
	dir := t.TempDir()

	// Pre-create one file
	skillDir := filepath.Join(dir, ".claude", "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "model.md"), []byte("existing content"), 0644)

	result, err := Scaffold(dir)
	require.NoError(t, err)

	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped[0], "model.md")
	assert.Len(t, result.Created, 2)

	// Existing file not overwritten
	data, _ := os.ReadFile(filepath.Join(skillDir, "model.md"))
	assert.Equal(t, "existing content", string(data))
}

func TestScaffoldSkipsExistingMCPConfig(t *testing.T) {
	dir := t.TempDir()

	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "mcp.json"), []byte("{}"), 0644)

	result, err := Scaffold(dir)
	require.NoError(t, err)

	var skippedMCP bool
	for _, s := range result.Skipped {
		if filepath.Base(s) == "mcp.json" {
			skippedMCP = true
		}
	}
	assert.True(t, skippedMCP)
}

func TestScaffoldPartialExisting(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, ".claude", "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "model.md"), []byte("existing"), 0644)

	result, err := Scaffold(dir)
	require.NoError(t, err)

	assert.Len(t, result.Skipped, 1)
	assert.Len(t, result.Created, 2)
}

func TestScaffoldAllExist(t *testing.T) {
	dir := t.TempDir()

	// Create all files
	_, err := Scaffold(dir)
	require.NoError(t, err)

	// Run again
	result, err := Scaffold(dir)
	require.NoError(t, err)

	assert.Empty(t, result.Created)
	assert.Len(t, result.Skipped, 3)
}
