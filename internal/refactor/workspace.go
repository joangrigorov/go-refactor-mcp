package refactor

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// ModuleInfo describes a single Go module within a workspace.
type ModuleInfo struct {
	Root string // Absolute directory path containing go.mod
	Path string // Module path defined in go.mod
}

// Workspace represents a Go workspace (single go.mod or go.work).
type Workspace struct {
	Root      string
	IsWork    bool
	Modules   []*ModuleInfo
	BuildTags []string
}

// FindWorkspace finds the enclosing workspace (go.work, monorepo, or go.mod) starting from startDir.
func FindWorkspace(startDir string) (*Workspace, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory path: %w", err)
	}

	info, err := os.Stat(absStart)
	if err == nil && !info.IsDir() {
		absStart = filepath.Dir(absStart)
	}

	workRoot, modRoots := searchWorkspaceRoots(absStart)

	if workRoot != "" {
		return loadGoWorkWorkspace(workRoot)
	}

	if len(modRoots) > 1 {
		return loadMultiModWorkspace(absStart, modRoots)
	}

	if len(modRoots) == 1 {
		return loadGoModWorkspace(modRoots[0])
	}

	return nil, fmt.Errorf("go.mod not found starting from %s", startDir)
}

func searchWorkspaceRoots(startDir string) (string, []string) {
	var workRoot string
	var modRoots []string

	curr := startDir
	for {
		if workRoot == "" {
			if _, err := os.Stat(filepath.Join(curr, "go.work")); err == nil {
				workRoot = curr
			}
		}
		if _, err := os.Stat(filepath.Join(curr, "go.mod")); err == nil {
			modRoots = append(modRoots, curr)
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	// If no go.work was found, check if startDir or its parent is part of a multi-module monorepo
	if workRoot == "" && len(modRoots) <= 1 {
		searchBase := startDir
		if len(modRoots) == 1 {
			searchBase = filepath.Dir(modRoots[0])
		}
		discovered := discoverSubModules(searchBase)
		if len(discovered) > 1 {
			modRoots = discovered
		}
	}

	return workRoot, modRoots
}

func discoverSubModules(baseDir string) []string {
	var modRoots []string
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == ".github" || name == "node_modules" || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(baseDir, path)
			if relErr == nil && strings.Count(rel, string(filepath.Separator)) > 3 {
				return filepath.SkipDir
			}
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
				modRoots = append(modRoots, path)
			}
		}
		return nil
	})
	return modRoots
}

func loadGoWorkWorkspace(workRoot string) (*Workspace, error) {
	workPath := filepath.Join(workRoot, "go.work")
	content, err := os.ReadFile(workPath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed reading go.work: %w", err)
	}

	parsedWork, err := modfile.ParseWork(workPath, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed parsing go.work: %w", err)
	}

	ws := &Workspace{
		Root:   workRoot,
		IsWork: true,
	}

	for _, use := range parsedWork.Use {
		usePath := filepath.Join(workRoot, filepath.FromSlash(use.Path))
		absModRoot, absErr := filepath.Abs(usePath)
		if absErr != nil {
			continue
		}
		modInfo, parseErr := parseModuleInfo(absModRoot)
		if parseErr == nil {
			ws.Modules = append(ws.Modules, modInfo)
		}
	}

	return ws, nil
}

func loadGoModWorkspace(modRoot string) (*Workspace, error) {
	modInfo, err := parseModuleInfo(modRoot)
	if err != nil {
		return nil, err
	}

	return &Workspace{
		Root:    modRoot,
		IsWork:  false,
		Modules: []*ModuleInfo{modInfo},
	}, nil
}

func loadMultiModWorkspace(root string, modRoots []string) (*Workspace, error) {
	ws := &Workspace{
		Root:   root,
		IsWork: false,
	}
	seen := make(map[string]bool)
	for _, mr := range modRoots {
		if seen[mr] {
			continue
		}
		seen[mr] = true
		info, err := parseModuleInfo(mr)
		if err == nil {
			ws.Modules = append(ws.Modules, info)
		}
	}
	if len(ws.Modules) == 0 {
		return nil, fmt.Errorf("no valid modules found in monorepo at %s", root)
	}
	return ws, nil
}

func parseModuleInfo(modRoot string) (*ModuleInfo, error) {
	modPath := filepath.Join(modRoot, "go.mod")
	content, err := os.ReadFile(modPath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed reading go.mod at %s: %w", modRoot, err)
	}

	parsedMod, err := modfile.Parse(modPath, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed parsing go.mod at %s: %w", modRoot, err)
	}

	if parsedMod.Module == nil || parsedMod.Module.Mod.Path == "" {
		return nil, fmt.Errorf("go.mod at %s missing module declaration", modRoot)
	}

	return &ModuleInfo{
		Root: modRoot,
		Path: parsedMod.Module.Mod.Path,
	}, nil
}

// ModuleForPath finds the module within the workspace that owns the given path.
func (ws *Workspace) ModuleForPath(targetPath string) (*ModuleInfo, error) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path %s: %w", targetPath, err)
	}

	var bestMatch *ModuleInfo
	var bestLen int

	for _, mod := range ws.Modules {
		rel, err := filepath.Rel(mod.Root, absPath)
		if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if len(mod.Root) > bestLen {
			bestMatch = mod
			bestLen = len(mod.Root)
		}
	}

	if bestMatch != nil {
		return bestMatch, nil
	}

	if len(ws.Modules) == 1 {
		return ws.Modules[0], nil
	}

	return nil, fmt.Errorf("no module in workspace contains path %s", targetPath)
}

// CalculateImportPath determines the canonical Go import path for an absolute directory.
func (ws *Workspace) CalculateImportPath(dirPath string) (string, error) {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	mod, err := ws.ModuleForPath(absDir)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(mod.Root, absDir)
	if err != nil {
		return "", err
	}

	if rel == "." || rel == "" {
		return mod.Path, nil
	}

	return filepath.ToSlash(filepath.Join(mod.Path, rel)), nil
}

// WalkGoFiles walks all non-vendor Go source files across all modules in the workspace.
func (ws *Workspace) WalkGoFiles(fn func(path string, mod *ModuleInfo) error) error {
	for _, mod := range ws.Modules {
		err := filepath.Walk(mod.Root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == ".git" || name == ".github" || name == "node_modules" || (strings.HasPrefix(name, ".") && name != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || IsVendorPath(path) {
				return nil
			}
			return fn(path, mod)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadPackages loads packages across all workspace modules with specified or auto-discovered build tags.
func (ws *Workspace) LoadPackages(userTags []string) ([]*packages.Package, error) {
	tags := userTags
	if len(tags) == 0 {
		discovered, discErr := DiscoverWorkspaceBuildTags(ws.Root)
		if discErr == nil {
			tags = discovered
		}
	}

	sharedFset := token.NewFileSet()

	if ws.IsWork {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
				packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
				packages.NeedSyntax | packages.NeedDeps,
			Dir:   ws.Root,
			Fset:  sharedFset,
			Tests: true,
		}

		if len(tags) > 0 {
			cfg.BuildFlags = []string{"-tags=" + strings.Join(tags, ",")}
		}

		var patterns []string
		for _, mod := range ws.Modules {
			rel, err := filepath.Rel(ws.Root, mod.Root)
			if err == nil {
				if rel == "." || rel == "" {
					patterns = append(patterns, "./...")
				} else {
					patterns = append(patterns, "./"+filepath.ToSlash(rel)+"/...")
				}
			}
		}
		if len(patterns) == 0 {
			patterns = []string{"./..."}
		}

		pkgs, err := packages.Load(cfg, patterns...)
		if err == nil && len(pkgs) > 0 {
			return pkgs, nil
		}
	}

	var allPkgs []*packages.Package
	for _, mod := range ws.Modules {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
				packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
				packages.NeedSyntax | packages.NeedDeps,
			Dir:   mod.Root,
			Fset:  sharedFset,
			Tests: true,
		}

		if len(tags) > 0 {
			cfg.BuildFlags = []string{"-tags=" + strings.Join(tags, ",")}
		}

		pkgs, err := packages.Load(cfg, "./...")
		if err != nil {
			return nil, fmt.Errorf("failed loading packages in %s: %w", mod.Root, err)
		}
		allPkgs = append(allPkgs, pkgs...)
	}

	return allPkgs, nil
}
