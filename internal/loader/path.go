package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ParseModelrPath splits a colon-separated path value into directory entries,
// filtering out empty segments.
func ParseModelrPath(envValue string) []string {
	if envValue == "" {
		return nil
	}
	parts := strings.Split(envValue, ":")
	var dirs []string
	for _, p := range parts {
		if p != "" {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

// ScanDirectory returns sorted paths of *.yaml files in the given directory.
// It does not recurse into subdirectories.
func ScanDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// ParseDefinitionFile parses a YAML file containing one or more definition documents.
// Each definition's Source field is set to the file path.
func ParseDefinitionFile(path string) ([]NodeDef, []RelationshipDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	nodes, rels, err := ParseDefinitions(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	for i := range nodes {
		nodes[i].Source = path
	}
	for i := range rels {
		rels[i].Source = path
	}

	return nodes, rels, nil
}

// LoadFromPath walks directories left to right, scanning for YAML definition files.
// Returns all definitions found plus any warnings (e.g., missing directories).
// Does not deduplicate — callers handle first-match-wins.
func LoadFromPath(dirs []string) ([]NodeDef, []RelationshipDef, []string, error) {
	var allNodes []NodeDef
	var allRels []RelationshipDef
	var warnings []string

	for _, dir := range dirs {
		files, err := ScanDirectory(dir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %s: %v", dir, err))
			continue
		}

		for _, f := range files {
			nodes, rels, err := ParseDefinitionFile(f)
			if err != nil {
				return nil, nil, nil, err
			}
			allNodes = append(allNodes, nodes...)
			allRels = append(allRels, rels...)
		}
	}

	return allNodes, allRels, warnings, nil
}
