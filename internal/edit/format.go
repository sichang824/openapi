package edit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"openapi/internal/workspace"

	"gopkg.in/yaml.v3"
)

func FormatSplitWorkspace(dir string) ([]string, error) {
	resolvedDir := filepath.Clean(dir)
	rootPath := workspace.SourceEntryPath(resolvedDir)
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("openapi entry not found: %s (run 'oapi init' first)", rootPath)
		}
		return nil, err
	}

	paths := []string{rootPath}
	files, err := collectWorkspaceYAMLFiles(resolvedDir)
	if err != nil {
		return nil, err
	}
	paths = append(paths, files...)

	for _, path := range paths {
		if err := FormatFile(path); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func FormatFile(path string) error {
	doc, err := loadDocument(path)
	if err != nil {
		return err
	}
	if err := normalizeDocument(doc); err != nil {
		return err
	}
	return writeDocument(path, doc)
}

func normalizeDocument(doc *yaml.Node) error {
	if _, err := ensureRootMapping(doc); err != nil {
		return err
	}
	normalizeNode(doc)
	return nil
}

func normalizeNode(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			normalizeNode(child)
		}
	case yaml.MappingNode:
		node.Style = 0
		for i := 0; i < len(node.Content); i += 2 {
			normalizeNode(node.Content[i])
			normalizeNode(node.Content[i+1])
		}
		sortMapping(node)
	case yaml.SequenceNode:
		node.Style = 0
		for _, child := range node.Content {
			normalizeNode(child)
		}
	}
}

func collectYAMLFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("expected directory: %s", dir)
	}

	paths := make([]string, 0)
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func collectWorkspaceYAMLFiles(root string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if entry.IsDir() {
			if relPath == "dist" || strings.HasPrefix(relPath, "dist"+string(os.PathSeparator)) {
				return filepath.SkipDir
			}
			return nil
		}
		if relPath == workspace.SourceEntryName {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
