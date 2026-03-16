package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache is a lightweight index mapping kind/name to file path and document index.
// It does not store parsed definitions — those are loaded lazily on demand.
type Cache struct {
	Path        string       `json:"path"`
	RefreshedAt time.Time    `json:"refreshed_at"`
	Files       []CacheFile  `json:"files"`
	Entries     []CacheEntry `json:"entries"`
}

// CacheFile stores metadata about a YAML file on the path.
type CacheFile struct {
	Path  string    `json:"path"`
	Mtime time.Time `json:"mtime"`
	Size  int64     `json:"size"`
}

// CacheEntry maps a definition (kind/name) to its file and document index.
type CacheEntry struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	File     string `json:"file"`
	DocIndex int    `json:"doc"`
}

// CachePath returns the path to the definitions cache file.
func CachePath(homeDir string) string {
	return filepath.Join(homeDir, ".modelr", "cache", "definitions.json")
}

// WriteCache writes the cache to disk, creating directories as needed.
func WriteCache(cache *Cache, homeDir string) error {
	path := CachePath(homeDir)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing cache temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}

// ReadCache reads the cache from disk. Returns nil, nil if the file does not exist.
func ReadCache(homeDir string) (*Cache, error) {
	path := CachePath(homeDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("cache file is empty")
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parsing cache: %w", err)
	}

	return &cache, nil
}

// StalenessResult describes whether the cache is up to date.
type StalenessResult struct {
	Stale  bool
	Reason string
}

// CheckStaleness compares the cache against the current filesystem state.
func CheckStaleness(cache *Cache, currentPath string) (*StalenessResult, error) {
	if cache.Path != currentPath {
		return &StalenessResult{Stale: true, Reason: "$MODELR_PATH changed"}, nil
	}

	cachedFiles := make(map[string]CacheFile)
	for _, f := range cache.Files {
		cachedFiles[f.Path] = f
	}

	for _, cf := range cache.Files {
		info, err := os.Stat(cf.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return &StalenessResult{Stale: true, Reason: fmt.Sprintf("file no longer exists: %s", cf.Path)}, nil
			}
			return nil, fmt.Errorf("checking file %s: %w", cf.Path, err)
		}
		if info.ModTime().After(cf.Mtime) {
			return &StalenessResult{Stale: true, Reason: fmt.Sprintf("file modified: %s", cf.Path)}, nil
		}
	}

	dirs := ParseModelrPath(currentPath)
	for _, dir := range dirs {
		files, err := ScanDirectory(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if _, ok := cachedFiles[f]; !ok {
				return &StalenessResult{Stale: true, Reason: fmt.Sprintf("new file: %s", f)}, nil
			}
		}
	}

	return &StalenessResult{Stale: false}, nil
}

// BuildCache walks $MODELR_PATH, parses all definitions, and builds a cache index.
// Returns the cache and any shadow events (path defs shadowing each other or embedded defaults).
func BuildCache(modelrPath string) (*Cache, []ShadowEvent, error) {
	dirs := ParseModelrPath(modelrPath)

	pathNodes, pathRels, _, err := LoadFromPath(dirs)
	if err != nil {
		return nil, nil, err
	}

	// Collect file metadata
	fileSet := make(map[string]bool)
	var cacheFiles []CacheFile

	for _, dir := range dirs {
		files, err := ScanDirectory(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if fileSet[f] {
				continue
			}
			fileSet[f] = true
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			cacheFiles = append(cacheFiles, CacheFile{
				Path:  f,
				Mtime: info.ModTime(),
				Size:  info.Size(),
			})
		}
	}

	// Build cache entries (index only — no full definitions stored)
	var entries []CacheEntry
	docIdx := make(map[string]int) // file → next doc index

	for _, n := range pathNodes {
		idx := docIdx[n.Source]
		docIdx[n.Source] = idx + 1
		entries = append(entries, CacheEntry{
			Kind:     "node",
			Name:     n.Name,
			File:     n.Source,
			DocIndex: idx,
		})
	}
	for _, r := range pathRels {
		idx := docIdx[r.Source]
		docIdx[r.Source] = idx + 1
		entries = append(entries, CacheEntry{
			Kind:     "relationship",
			Name:     r.Name,
			File:     r.Source,
			DocIndex: idx,
		})
	}

	// Detect shadows
	tempReg := NewRegistry()
	for _, n := range pathNodes {
		tempReg.AddNode(n)
	}
	for _, r := range pathRels {
		tempReg.AddRelationship(r)
	}
	embNodes, embRels, _ := LoadEmbedded()
	for _, n := range embNodes {
		tempReg.AddNode(n)
	}
	for _, r := range embRels {
		tempReg.AddRelationship(r)
	}

	cache := &Cache{
		Path:        modelrPath,
		RefreshedAt: time.Now().UTC(),
		Files:       cacheFiles,
		Entries:     entries,
	}

	return cache, tempReg.Shadows(), nil
}

// parsedFile holds the parsed definitions from a single YAML file.
type parsedFile struct {
	nodes []NodeDef
	rels  []RelationshipDef
	err   error
}

// DefinitionLoader lazily parses definition files and memoizes results.
// Thread-safe: multiple goroutines can call LoadEntry concurrently.
type DefinitionLoader struct {
	mu    sync.Mutex
	files map[string]*parsedFile
}

// NewDefinitionLoader creates a new memoizing loader.
func NewDefinitionLoader() *DefinitionLoader {
	return &DefinitionLoader{
		files: make(map[string]*parsedFile),
	}
}

func (dl *DefinitionLoader) parseFile(path string) *parsedFile {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if pf, ok := dl.files[path]; ok {
		return pf
	}

	nodes, rels, err := ParseDefinitionFile(path)
	pf := &parsedFile{nodes: nodes, rels: rels, err: err}
	dl.files[path] = pf
	return pf
}

// LoadEntry resolves a cache entry to its definition by parsing the referenced
// file (or returning a memoized result) and extracting the document at the given index.
func (dl *DefinitionLoader) LoadEntry(entry CacheEntry) (*NodeDef, *RelationshipDef, error) {
	pf := dl.parseFile(entry.File)
	if pf.err != nil {
		return nil, nil, pf.err
	}

	// Walk through definitions in file order to find the one at DocIndex
	idx := 0
	for i := range pf.nodes {
		if pf.nodes[i].Name == entry.Name && entry.Kind == "node" && idx == entry.DocIndex {
			return &pf.nodes[i], nil, nil
		}
		idx++
	}
	for i := range pf.rels {
		if pf.rels[i].Name == entry.Name && entry.Kind == "relationship" && idx == entry.DocIndex {
			return nil, &pf.rels[i], nil
		}
		idx++
	}

	return nil, nil, fmt.Errorf("definition %s/%s not found at doc %d in %s", entry.Kind, entry.Name, entry.DocIndex, entry.File)
}

// DefsFromCache lazily loads all definitions referenced by cache entries.
// Files are parsed at most once regardless of how many entries reference them.
func DefsFromCache(cache *Cache) ([]NodeDef, []RelationshipDef, error) {
	dl := NewDefinitionLoader()

	var nodes []NodeDef
	var rels []RelationshipDef

	for _, entry := range cache.Entries {
		nd, rd, err := dl.LoadEntry(entry)
		if err != nil {
			return nil, nil, fmt.Errorf("loading %s/%s from cache: %w", entry.Kind, entry.Name, err)
		}
		if nd != nil {
			nodes = append(nodes, *nd)
		}
		if rd != nil {
			rels = append(rels, *rd)
		}
	}

	return nodes, rels, nil
}
