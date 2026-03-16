package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/poiesic/modelr/internal/check"
	modelrinit "github.com/poiesic/modelr/internal/init"
	"github.com/poiesic/modelr/internal/loader"
	mcpserver "github.com/poiesic/modelr/internal/mcp"
	"github.com/poiesic/modelr/internal/model"
	"github.com/poiesic/modelr/internal/verify"
	"github.com/poiesic/modelr/internal/viz"
	"github.com/urfave/cli/v3"
)

// envConfig holds environment values injected for testability.
type envConfig struct {
	modelrPath string
	homeDir    string
}

func envFromOS() envConfig {
	return envConfig{
		modelrPath: os.Getenv("MODELR_PATH"),
		homeDir:    os.Getenv("HOME"),
	}
}

func buildApp(stdout, stderr io.Writer) *cli.Command {
	return buildAppWithEnv(stdout, stderr, envFromOS())
}

func buildAppWithEnv(stdout, stderr io.Writer, env envConfig) *cli.Command {
	return &cli.Command{
		Name:      "modelr",
		Usage:     "Composable system modeling with explicit uncertainty",
		Version:   "0.2",
		Writer:    stdout,
		ErrWriter: stderr,
		Commands: []*cli.Command{
			{
				Name:      "validate",
				Usage:     "Parse and validate a model file",
				ArgsUsage: "<path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show definition resolution details"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runValidate(cmd, env, stderr)
				},
			},
			{
				Name:      "check",
				Usage:     "Parse, validate, and check a model file",
				ArgsUsage: "<path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show definition resolution details"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runCheck(cmd, env, stderr)
				},
			},
			{
				Name:      "verify",
				Usage:     "Behavioral verification via state exploration",
				ArgsUsage: "<path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show definition resolution details"},
					&cli.FloatFlag{Name: "failure-rate", Value: 0.001, Usage: "Target failure rate"},
					&cli.FloatFlag{Name: "confidence", Value: 0.99, Usage: "Required confidence level"},
					&cli.IntFlag{Name: "shrink-budget", Value: 2000, Usage: "Maximum shrink attempts for failure minimization"},
					&cli.IntFlag{Name: "seed", Value: 0, Usage: "Random seed (0 = random)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runVerify(cmd, env, stderr)
				},
			},
			{
				Name:      "visualize",
				Usage:     "Check and generate DOT/SVG visualization",
				ArgsUsage: "<path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show definition resolution details"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runVisualize(cmd, env, stderr)
				},
			},
			{
				Name:  "cache",
				Usage: "Manage the definition cache",
				Commands: []*cli.Command{
					{
						Name:  "refresh",
						Usage: "Refresh the definition cache",
						Flags: []cli.Flag{
							&cli.BoolFlag{Name: "rebuild", Usage: "Delete and rebuild cache from scratch"},
							&cli.BoolFlag{Name: "auto", Usage: "Only rebuild if stale"},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return runCacheRefresh(cmd, env, stderr)
						},
					},
					{
						Name:  "status",
						Usage: "Show cache status",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return runCacheStatus(cmd, env)
						},
					},
				},
			},
			{
				Name:  "definitions",
				Usage: "List available node and relationship definitions",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Show property details"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runDefinitions(cmd, env, stderr)
				},
			},
			{
				Name:  "init",
				Usage: "Initialize Claude Code integration (skills + MCP config)",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runInit(cmd)
				},
			},
			{
				Name:  "mcp",
				Usage: "Start MCP server on stdio",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					s := mcpserver.NewServer(mcpserver.EnvConfig{
						ModelrPath: env.modelrPath,
						HomeDir:    env.homeDir,
					})
					return server.ServeStdio(s)
				},
			},
		},
	}
}

type pipelineResult struct {
	parseResult *model.ParseResult
	registry    *loader.Registry
	validation  *model.ValidationResult
}

func loadPathDefs(env envConfig, stderr io.Writer) ([]loader.NodeDef, []loader.RelationshipDef, error) {
	if env.modelrPath == "" {
		return nil, nil, nil
	}

	cache, err := loader.ReadCache(env.homeDir)
	if err != nil {
		return nil, nil, err
	}

	if cache == nil {
		// Auto-build cache on first run
		newCache, shadows, err := loader.BuildCache(env.modelrPath)
		if err != nil {
			return nil, nil, err
		}
		for _, s := range shadows {
			fmt.Fprintf(stderr, "info: %s/%s shadowed (%s → %s)\n", s.Kind, s.Name, s.Shadowed, s.Winner)
		}
		if err := loader.WriteCache(newCache, env.homeDir); err != nil {
			return nil, nil, err
		}
		return loader.DefsFromCache(newCache)
	}

	// Cache exists — check staleness
	staleness, err := loader.CheckStaleness(cache, env.modelrPath)
	if err != nil {
		return nil, nil, err
	}
	if staleness.Stale {
		fmt.Fprintf(stderr, "warning: definition cache is stale (run 'modelr cache refresh' to update)\n")
	}

	return loader.DefsFromCache(cache)
}

func runPipeline(cmd *cli.Command, env envConfig, stderr io.Writer) (*pipelineResult, error) {
	path := cmd.Args().First()
	if path == "" {
		return nil, fmt.Errorf("path argument is required")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	parseResult, err := model.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	pathNodes, pathRels, err := loadPathDefs(env, stderr)
	if err != nil {
		return nil, fmt.Errorf("loading path definitions: %w", err)
	}

	registry, err := loader.BuildRegistry(loader.DefinitionSources{
		InlineNodes: parseResult.InlineNodes,
		InlineRels:  parseResult.InlineRels,
		PathNodes:   pathNodes,
		PathRels:    pathRels,
	})
	if err != nil {
		return nil, fmt.Errorf("building registry: %w", err)
	}

	if cmd.Bool("verbose") {
		printRegistryResolution(cmd.Root().Writer, registry)
	}

	validation, err := model.Validate(parseResult.Model, registry)
	if err != nil {
		return nil, fmt.Errorf("validating: %w", err)
	}

	return &pipelineResult{
		parseResult: parseResult,
		registry:    registry,
		validation:  validation,
	}, nil
}

func runValidate(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	result, err := runPipeline(cmd, env, stderr)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	printValidationResults(w, result.validation)
	return nil
}

func runCheck(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	path := cmd.Args().First()

	result, err := runPipeline(cmd, env, stderr)
	if err != nil {
		return err
	}

	checkResult, err := check.Check(result.parseResult.Model, result.registry)
	if err != nil {
		return fmt.Errorf("checking: %w", err)
	}

	if err := model.WriteCheckedYAML(path, result.parseResult.Model.Name, checkResult, result.validation); err != nil {
		return err
	}

	w := cmd.Root().Writer
	printValidationResults(w, result.validation)
	printCheckResults(w, checkResult)

	outputPath := model.CheckedOutputPath(path)
	fmt.Fprintf(w, "\nWrote %s\n", outputPath)

	return nil
}

func runVisualize(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	path := cmd.Args().First()

	result, err := runPipeline(cmd, env, stderr)
	if err != nil {
		return err
	}

	checkResult, err := check.Check(result.parseResult.Model, result.registry)
	if err != nil {
		return fmt.Errorf("checking: %w", err)
	}

	dotContent := viz.GenerateDOT(&viz.DOTInput{
		Model:            result.parseResult.Model,
		CheckResult:      checkResult,
		ValidationResult: result.validation,
	})

	// Write .dot file
	dotPath := dotOutputPath(path)
	if err := os.WriteFile(dotPath, []byte(dotContent), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", dotPath, err)
	}

	w := cmd.Root().Writer
	fmt.Fprintf(w, "Wrote %s\n", dotPath)

	// Try to render SVG if graphviz is available
	if viz.GraphvizAvailable() {
		svgBytes, err := viz.RenderDOT(dotContent, "svg")
		if err != nil {
			fmt.Fprintf(stderr, "warning: SVG rendering failed: %v\n", err)
		} else {
			svgPath := svgOutputPath(path)
			if err := os.WriteFile(svgPath, svgBytes, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", svgPath, err)
			}
			fmt.Fprintf(w, "Wrote %s\n", svgPath)
		}
	} else {
		fmt.Fprintln(w, "Install graphviz to also generate SVG (e.g., apt install graphviz)")
	}

	return nil
}

func runVerify(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	path := cmd.Args().First()

	result, err := runPipeline(cmd, env, stderr)
	if err != nil {
		return err
	}

	config := verify.DefaultVerifyConfig()
	config.TargetFailureRate = cmd.Float("failure-rate")
	config.Confidence = cmd.Float("confidence")
	config.ShrinkAttempts = int(cmd.Int("shrink-budget"))
	config.Seed = int64(cmd.Int("seed"))

	if cmd.Bool("verbose") {
		config.OnShrinkProgress = func(upstream, downstream string, p verify.ShrinkProgress) {
			suffix := ""
			if p.Improved {
				suffix = " (improved)"
			}
			fmt.Fprintf(stderr, "[shrink] %s → %s: phase %s, attempt %d/%d, best: %d bytes, %d steps%s\n",
				upstream, downstream, p.Phase, p.Attempt, p.MaxAttempts, p.BestLength, p.BestSteps, suffix)
		}
	}

	verResult, err := verify.Verify(result.parseResult.Model, result.registry, result.validation, config)
	if err != nil {
		return fmt.Errorf("verifying: %w", err)
	}

	if err := verify.WriteVerifiedYAML(path, result.parseResult.Model.Name, verResult, result.validation); err != nil {
		return err
	}

	w := cmd.Root().Writer
	for _, v := range verResult.Verifications {
		if v.Result == "pass" {
			fmt.Fprintf(w, "Accepted %s → %s after %d simulations (0 failures)\n",
				v.Upstream, v.Downstream, v.Simulations)
			fmt.Fprintf(w, "Confidence: %.0f%% that failure rate < %.1f%%\n",
				v.Confidence*100, v.FailureRateBound*100)
		} else {
			fmt.Fprintf(w, "Rejected %s → %s after %d simulations (%d failures)\n",
				v.Upstream, v.Downstream, v.Simulations, v.Failures)
			fmt.Fprintf(w, "Estimated failure rate: %.1f%%\n",
				float64(v.Failures)/float64(v.Simulations)*100)
			if len(v.MinimalFailure) > 0 {
				fmt.Fprintln(w, "Minimal failure case:")
				for i, step := range v.MinimalFailure {
					fmt.Fprintf(w, "  %d. %s(instance=%d) → %v\n", i+1, step.Rule, step.Instance, step.State)
				}
				fmt.Fprintf(w, "  Property violated: %s\n", v.ViolatedInvariant)
			}
		}
	}

	fmt.Fprintf(w, "\n%s\n", verResult.Summary)
	outputPath := verify.VerifiedOutputPath(path)
	fmt.Fprintf(w, "Wrote %s\n", outputPath)

	return nil
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

func runCacheRefresh(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	if cmd.Bool("rebuild") && cmd.Bool("auto") {
		return fmt.Errorf("--rebuild and --auto are mutually exclusive")
	}

	w := cmd.Root().Writer

	if cmd.Bool("auto") {
		existing, err := loader.ReadCache(env.homeDir)
		if err != nil {
			return err
		}
		if existing != nil {
			staleness, err := loader.CheckStaleness(existing, env.modelrPath)
			if err != nil {
				return err
			}
			if !staleness.Stale {
				fmt.Fprintln(w, "Cache is current, no refresh needed.")
				return nil
			}
		}
	}

	cache, shadows, err := loader.BuildCache(env.modelrPath)
	if err != nil {
		return err
	}

	for _, s := range shadows {
		fmt.Fprintf(stderr, "info: %s/%s shadowed (%s → %s)\n", s.Kind, s.Name, s.Shadowed, s.Winner)
	}

	if err := loader.WriteCache(cache, env.homeDir); err != nil {
		return err
	}

	fmt.Fprintf(w, "Cache refreshed: %d definitions cached.\n", len(cache.Entries))
	return nil
}

func runCacheStatus(cmd *cli.Command, env envConfig) error {
	w := cmd.Root().Writer

	cache, err := loader.ReadCache(env.homeDir)
	if err != nil {
		return err
	}

	if cache == nil {
		fmt.Fprintln(w, "No cache found. Run 'modelr cache refresh' to build.")
		return nil
	}

	age := time.Since(cache.RefreshedAt).Truncate(time.Second)
	fmt.Fprintf(w, "Last refreshed: %s (%s ago)\n", cache.RefreshedAt.Format(time.RFC3339), age)
	fmt.Fprintf(w, "Definitions:    %d\n", len(cache.Entries))

	info, err := os.Stat(loader.CachePath(env.homeDir))
	if err == nil {
		fmt.Fprintf(w, "Size on disk:   %s\n", formatBytes(info.Size()))
	}

	staleness, err := loader.CheckStaleness(cache, env.modelrPath)
	if err != nil {
		return err
	}
	if staleness.Stale {
		fmt.Fprintf(w, "Status:         stale (%s)\n", staleness.Reason)
	} else {
		fmt.Fprintln(w, "Status:         current")
	}

	return nil
}

func runDefinitions(cmd *cli.Command, env envConfig, stderr io.Writer) error {
	w := cmd.Root().Writer
	verbose := cmd.Bool("verbose")

	pathNodes, pathRels, err := loadPathDefs(env, stderr)
	if err != nil {
		return err
	}

	registry, err := loader.BuildRegistry(loader.DefinitionSources{
		PathNodes: pathNodes,
		PathRels:  pathRels,
	})
	if err != nil {
		return err
	}

	sources := registry.Sources()

	fmt.Fprintln(w, "Node Definitions:")
	for _, s := range sources {
		if s.Kind != "node" {
			continue
		}
		nodeDef, _ := registry.LookupNode(s.Name)
		fmt.Fprintf(w, "  %-20s %-40s (%s)\n", s.Name, nodeDef.Description, s.Origin)
		if verbose {
			for propName, prop := range nodeDef.Properties {
				defStr := ""
				if prop.Default != nil {
					defStr = fmt.Sprintf("  default: %v", prop.Default)
				}
				fmt.Fprintf(w, "    %-20s %-8s %-14s%s\n", propName, prop.Type, prop.Unit, defStr)
			}
		}
	}

	fmt.Fprintln(w, "\nRelationship Definitions:")
	for _, s := range sources {
		if s.Kind != "relationship" {
			continue
		}
		relDef, _ := registry.LookupRelationship(s.Name)
		fmt.Fprintf(w, "  %-20s %-40s (%s)\n", s.Name, relDef.Description, s.Origin)
	}

	return nil
}

func runInit(cmd *cli.Command) error {
	w := cmd.Root().Writer

	result, err := modelrinit.Scaffold(".")
	if err != nil {
		return err
	}

	for _, f := range result.Created {
		fmt.Fprintf(w, "  created: %s\n", f)
	}
	for _, f := range result.Skipped {
		fmt.Fprintf(w, "  skipped: %s (already exists)\n", f)
	}

	if len(result.Created) > 0 {
		fmt.Fprintln(w, "\nClaude Code integration initialized.")
	} else {
		fmt.Fprintln(w, "\nAll files already exist, nothing to do.")
	}

	return nil
}

func printValidationResults(w io.Writer, v *model.ValidationResult) {
	if len(v.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, warning := range v.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}

	if len(v.Assumptions) > 0 {
		fmt.Fprintln(w, "Assumptions:")
		for _, a := range v.Assumptions {
			fmt.Fprintf(w, "  - %s.%s = %v (from %s)\n", a.Component, a.Property, a.Value, a.Source)
		}
	}

	if len(v.KnownUnknowns) > 0 {
		fmt.Fprintf(w, "Known unknowns: %d\n", len(v.KnownUnknowns))
		for _, ku := range v.KnownUnknowns {
			fmt.Fprintf(w, "  - [%s] %s\n", ku.Category, ku.Description)
		}
	}
}

func printCheckResults(w io.Writer, cr *model.CheckResult) {
	if len(cr.Findings) > 0 {
		fmt.Fprintln(w, "Findings:")
		for _, f := range cr.Findings {
			fmt.Fprintf(w, "  - [%s] %s → %s: %s\n", f.Severity, f.Upstream, f.Downstream, f.Description)
			if f.Suggestion != "" {
				fmt.Fprintf(w, "    Suggestion: %s\n", f.Suggestion)
			}
		}
	}

	if len(cr.KnownUnknowns) > 0 {
		fmt.Fprintf(w, "Check known unknowns: %d\n", len(cr.KnownUnknowns))
		for _, ku := range cr.KnownUnknowns {
			fmt.Fprintf(w, "  - [%s] %s\n", ku.Category, ku.Description)
		}
	}

	fmt.Fprintf(w, "\n%s\n", cr.Summary)
}

func printRegistryResolution(w io.Writer, registry *loader.Registry) {
	fmt.Fprintln(w, "Loading definitions:")
	for _, src := range registry.Sources() {
		fmt.Fprintf(w, "  %-36s ← %s\n", src.Kind+"/"+src.Name, src.Origin)
	}
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func main() {
	app := buildApp(os.Stdout, os.Stderr)
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
