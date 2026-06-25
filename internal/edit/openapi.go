package edit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func UpsertRef(openapiPath string, sectionPath []string, key string, ref string, force bool) error {
	doc, err := loadDocument(openapiPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("openapi entry not found: %s (run 'oapi init' first)", openapiPath)
		}
		return err
	}

	root, err := ensureRootMapping(doc)
	if err != nil {
		return err
	}
	target := ensurePath(root, sectionPath)
	target.Style = 0
	if existing := mappingValue(target, key); existing != nil && !force {
		return fmt.Errorf("%s already exists: %s (use --force to overwrite)", strings.Join(sectionPath, "."), key)
	}

	setMappingValue(target, key, mappingNode("$ref", scalarNode(ref)))
	sortMapping(target)
	return writeDocument(openapiPath, doc)
}

func UpsertNamedDefinition(filePath string, name string, value *yaml.Node, force bool) error {
	doc, err := loadOrCreateDocument(filePath)
	if err != nil {
		return err
	}

	root, err := ensureRootMapping(doc)
	if err != nil {
		return err
	}
	root.Style = 0
	if existing := mappingValue(root, name); existing != nil && !force {
		return fmt.Errorf("definition already exists: %s (use --force to overwrite)", name)
	}

	setMappingValue(root, name, cloneNode(value))
	sortMapping(root)
	return writeDocument(filePath, doc)
}

func loadOrCreateDocument(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	return &doc, nil
}

func loadDocument(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid yaml document: %s", path)
	}
	return &doc, nil
}

func writeDocument(path string, doc *yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func ensureRootMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("yaml document must start with a document node")
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind == 0 {
		root.Kind = yaml.MappingNode
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yaml document root must be a mapping")
	}
	return root, nil
}

func ensurePath(root *yaml.Node, sections []string) *yaml.Node {
	current := root
	for _, section := range sections {
		next := mappingValue(current, section)
		if next == nil {
			next = &yaml.Node{Kind: yaml.MappingNode}
			setMappingValue(current, section, next)
		}
		if next.Kind == 0 {
			next.Kind = yaml.MappingNode
		}
		next.Style = 0
		current = next
	}
	return current
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, scalarNode(key), value)
}

func sortMapping(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) < 4 {
		return
	}

	type entry struct {
		key   *yaml.Node
		value *yaml.Node
	}
	entries := make([]entry, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		entries = append(entries, entry{key: node.Content[i], value: node.Content[i+1]})
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].key.Value < entries[j].key.Value
	})

	node.Content = node.Content[:0]
	for _, item := range entries {
		node.Content = append(node.Content, item.key, item.value)
	}
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copyNode := *node
	if len(node.Content) > 0 {
		copyNode.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			copyNode.Content[i] = cloneNode(child)
		}
	}
	return &copyNode
}

func mappingNode(pairs ...interface{}) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(pairs); i += 2 {
		key := pairs[i].(string)
		value := pairs[i+1].(*yaml.Node)
		node.Content = append(node.Content, scalarNode(key), value)
	}
	return node
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
