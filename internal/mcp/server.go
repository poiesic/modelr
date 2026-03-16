package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/poiesic/modelr/internal/check"
	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
	"github.com/poiesic/modelr/internal/viz"
)

// EnvConfig holds environment values needed by MCP tool handlers.
type EnvConfig struct {
	ModelrPath string
	HomeDir    string
}

// NewServer creates a new MCP server with all modelr tools registered.
func NewServer(env EnvConfig) *server.MCPServer {
	s := server.NewMCPServer(
		"modelr",
		"0.2",
		server.WithToolCapabilities(false),
	)

	s.AddTool(
		mcp.NewTool("list_definitions",
			mcp.WithDescription("List available node and relationship definitions"),
		),
		listDefinitionsHandler(env),
	)

	s.AddTool(
		mcp.NewTool("check_model",
			mcp.WithDescription("Parse, validate, check; write .checked.yaml"),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the .model.yaml file"),
			),
		),
		checkModelHandler(env),
	)

	s.AddTool(
		mcp.NewTool("validate_model",
			mcp.WithDescription("Parse, validate; return model with warnings and assumptions"),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the .model.yaml file"),
			),
		),
		validateModelHandler(env),
	)

	s.AddTool(
		mcp.NewTool("visualize_model",
			mcp.WithDescription("Check + generate DOT/SVG visualization"),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the .model.yaml file"),
			),
		),
		visualizeModelHandler(env),
	)

	s.AddTool(
		mcp.NewTool("verify_model",
			mcp.WithDescription("Behavioral verification via state exploration"),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the .model.yaml file"),
			),
		),
		verifyModelHandler(),
	)

	return s
}

func loadPathDefsForMCP(env EnvConfig) ([]loader.NodeDef, []loader.RelationshipDef, error) {
	if env.ModelrPath == "" {
		return nil, nil, nil
	}

	cache, err := loader.ReadCache(env.HomeDir)
	if err != nil {
		return nil, nil, err
	}

	if cache == nil {
		newCache, _, err := loader.BuildCache(env.ModelrPath)
		if err != nil {
			return nil, nil, err
		}
		_ = loader.WriteCache(newCache, env.HomeDir)
		return loader.DefsFromCache(newCache)
	}

	return loader.DefsFromCache(cache)
}

func runModelPipeline(env EnvConfig, path string) (*model.ParseResult, *loader.Registry, *model.ValidationResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	parseResult, err := model.Parse(f)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	pathNodes, pathRels, err := loadPathDefsForMCP(env)
	if err != nil {
		return nil, nil, nil, err
	}

	registry, err := loader.BuildRegistry(loader.DefinitionSources{
		InlineNodes: parseResult.InlineNodes,
		InlineRels:  parseResult.InlineRels,
		PathNodes:   pathNodes,
		PathRels:    pathRels,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	validation, err := model.Validate(parseResult.Model, registry)
	if err != nil {
		return nil, nil, nil, err
	}

	return parseResult, registry, validation, nil
}

func listDefinitionsHandler(env EnvConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathNodes, pathRels, err := loadPathDefsForMCP(env)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		registry, err := loader.BuildRegistry(loader.DefinitionSources{
			PathNodes: pathNodes,
			PathRels:  pathRels,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var b strings.Builder
		b.WriteString("Node Definitions:\n")
		for _, src := range registry.Sources() {
			if src.Kind == "node" {
				def, _ := registry.LookupNode(src.Name)
				b.WriteString(fmt.Sprintf("  %s - %s (%s)\n", src.Name, def.Description, src.Origin))
				for propName, prop := range def.Properties {
					defStr := ""
					if prop.Default != nil {
						defStr = fmt.Sprintf(" default: %v", prop.Default)
					}
					b.WriteString(fmt.Sprintf("    %s: %s (%s)%s\n", propName, prop.Type, prop.Unit, defStr))
				}
			}
		}
		b.WriteString("\nRelationship Definitions:\n")
		for _, src := range registry.Sources() {
			if src.Kind == "relationship" {
				def, _ := registry.LookupRelationship(src.Name)
				b.WriteString(fmt.Sprintf("  %s - %s (%s)\n", src.Name, def.Description, src.Origin))
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

func checkModelHandler(env EnvConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		parseResult, registry, validation, err := runModelPipeline(env, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		checkResult, err := check.Check(parseResult.Model, registry)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := model.WriteCheckedYAML(path, parseResult.Model.Name, checkResult, validation); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var b strings.Builder
		formatCheckOutput(&b, path, checkResult, validation)
		return mcp.NewToolResultText(b.String()), nil
	}
}

func validateModelHandler(env EnvConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		_, _, validation, err := runModelPipeline(env, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var b strings.Builder
		formatValidationOutput(&b, validation)
		return mcp.NewToolResultText(b.String()), nil
	}
}

func visualizeModelHandler(env EnvConfig) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		parseResult, registry, validation, err := runModelPipeline(env, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		checkResult, err := check.Check(parseResult.Model, registry)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dotContent := viz.GenerateDOT(&viz.DOTInput{
			Model:            parseResult.Model,
			CheckResult:      checkResult,
			ValidationResult: validation,
		})

		// Write .dot file
		dotPath := dotOutputPath(path)
		if err := os.WriteFile(dotPath, []byte(dotContent), 0644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("writing %s: %v", dotPath, err)), nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Wrote %s\n\n", dotPath))

		if viz.GraphvizAvailable() {
			svgBytes, err := viz.RenderDOT(dotContent, "svg")
			if err == nil {
				svgPath := svgOutputPath(path)
				if err := os.WriteFile(svgPath, svgBytes, 0644); err == nil {
					b.WriteString(fmt.Sprintf("Wrote %s\n\n", svgPath))
				}
			}
		}

		b.WriteString("DOT content:\n")
		b.WriteString(dotContent)
		return mcp.NewToolResultText(b.String()), nil
	}
}

func verifyModelHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Behavioral verification is not yet implemented."), nil
	}
}

func formatCheckOutput(b *strings.Builder, path string, cr *model.CheckResult, vr *model.ValidationResult) {
	if len(vr.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, w := range vr.Warnings {
			b.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}
	if len(vr.Assumptions) > 0 {
		b.WriteString("Assumptions:\n")
		for _, a := range vr.Assumptions {
			b.WriteString(fmt.Sprintf("  - %s.%s = %v (from %s)\n", a.Component, a.Property, a.Value, a.Source))
		}
	}
	if len(cr.Findings) > 0 {
		b.WriteString("Findings:\n")
		for _, f := range cr.Findings {
			b.WriteString(fmt.Sprintf("  - [%s] %s → %s: %s\n", f.Severity, f.Upstream, f.Downstream, f.Description))
			if f.Suggestion != "" {
				b.WriteString(fmt.Sprintf("    Suggestion: %s\n", f.Suggestion))
			}
		}
	}
	b.WriteString(fmt.Sprintf("\n%s\n", cr.Summary))
	b.WriteString(fmt.Sprintf("Wrote %s\n", model.CheckedOutputPath(path)))
}

func formatValidationOutput(b *strings.Builder, vr *model.ValidationResult) {
	if len(vr.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, w := range vr.Warnings {
			b.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}
	if len(vr.Assumptions) > 0 {
		b.WriteString("Assumptions:\n")
		for _, a := range vr.Assumptions {
			b.WriteString(fmt.Sprintf("  - %s.%s = %v (from %s)\n", a.Component, a.Property, a.Value, a.Source))
		}
	}
	if len(vr.KnownUnknowns) > 0 {
		b.WriteString(fmt.Sprintf("Known unknowns: %d\n", len(vr.KnownUnknowns)))
		for _, ku := range vr.KnownUnknowns {
			b.WriteString(fmt.Sprintf("  - [%s] %s\n", ku.Category, ku.Description))
		}
	}
}

func dotOutputPath(inputPath string) string {
	if strings.HasSuffix(inputPath, ".model.yaml") {
		return strings.TrimSuffix(inputPath, ".model.yaml") + ".dot"
	}
	return inputPath + ".dot"
}

func svgOutputPath(inputPath string) string {
	if strings.HasSuffix(inputPath, ".model.yaml") {
		return strings.TrimSuffix(inputPath, ".model.yaml") + ".svg"
	}
	return inputPath + ".svg"
}
