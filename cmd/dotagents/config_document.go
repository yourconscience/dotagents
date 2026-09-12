package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type configLayer string

const (
	configLayerShared    configLayer = "shared"
	configLayerLocal     configLayer = "local"
	configLayerEffective configLayer = "effective"
)

var (
	errStaleRevision = errors.New("stale config revision")
	errReadOnlyLayer = errors.New("effective config is read-only")
)

type configDocument struct {
	home       string
	sharedPath string
	localPath  string

	sharedBytes []byte
	sharedNode  yaml.Node
	shared      config

	localBytes []byte
	localNode  *yaml.Node
	local      config

	effective config
}

type configSave struct {
	Layer    configLayer
	Before   []byte
	After    []byte
	Revision string
	Diff     string
}

type configOperation struct {
	Op      string          `json:"op"`
	Path    string          `json:"path"`
	Section string          `json:"section,omitempty"`
	Key     string          `json:"key,omitempty"`
	Field   string          `json:"field,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
}

func newConfigDocument(path string, home string) (*configDocument, error) {
	if strings.TrimSpace(path) == "" {
		resolved, err := resolveConfigPath("", home)
		if err != nil {
			return nil, err
		}
		path = resolved
	} else {
		var err error
		path, err = absoluteExpandedPath(path, home)
		if err != nil {
			return nil, err
		}
	}
	path = filepath.Clean(path)
	if err := refuseWorktreeRoot(filepath.Dir(path)); err != nil {
		return nil, err
	}
	doc := &configDocument{home: home, sharedPath: path, localPath: filepath.Join(filepath.Dir(path), "dotagents.local.yaml")}
	if err := doc.reload(); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *configDocument) reload() error {
	sharedBytes, err := os.ReadFile(d.sharedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("canonical config %s not found; run dotagents setup", d.sharedPath)
		}
		return fmt.Errorf("read config %s: %w", d.sharedPath, err)
	}
	sharedNode, shared, err := decodeConfigDocument(sharedBytes, d.home, false)
	if err != nil {
		return fmt.Errorf("validate shared config %s: %w", d.sharedPath, err)
	}
	d.sharedBytes, d.sharedNode, d.shared = sharedBytes, sharedNode, shared

	localBytes, err := os.ReadFile(d.localPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		d.localBytes, d.localNode, d.local = nil, nil, config{}
	case err != nil:
		return fmt.Errorf("read local config %s: %w", d.localPath, err)
	default:
		localNode, local, decodeErr := decodeConfigDocument(localBytes, d.home, false)
		if decodeErr != nil {
			return fmt.Errorf("validate local config %s: %w", d.localPath, decodeErr)
		}
		d.localBytes, d.localNode, d.local = localBytes, &localNode, local
	}

	effective, err := cloneConfig(d.shared)
	if err != nil {
		return fmt.Errorf("clone shared config: %w", err)
	}
	mergeConfig(&effective, d.local)
	if err := validateConfig(&effective, d.home, true); err != nil {
		return fmt.Errorf("validate effective config: %w", err)
	}
	d.effective = effective
	return nil
}

func decodeConfigDocument(data []byte, home string, expand bool) (yaml.Node, config, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return yaml.Node{}, config{}, fmt.Errorf("yaml decode: %w", err)
	}
	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return yaml.Node{}, config{}, fmt.Errorf("yaml decode: %w", err)
	}
	if err := validateConfig(&cfg, home, expand); err != nil {
		return yaml.Node{}, config{}, err
	}
	return node, cfg, nil
}
func cloneConfig(cfg config) (config, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return config{}, err
	}
	var clone config
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return config{}, err
	}
	return clone, nil
}

func (d *configDocument) path(layer configLayer) string {
	if layer == configLayerLocal {
		return d.localPath
	}
	return d.sharedPath
}

func (d *configDocument) bytes(layer configLayer) []byte {
	switch layer {
	case configLayerLocal:
		return append([]byte(nil), d.localBytes...)
	case configLayerEffective:
		data, _ := yaml.Marshal(d.effective)
		return data
	default:
		return append([]byte(nil), d.sharedBytes...)
	}
}

func (d *configDocument) typed(layer configLayer) config {
	switch layer {
	case configLayerLocal:
		return d.local
	case configLayerEffective:
		return d.effective
	default:
		return d.shared
	}
}

func (d *configDocument) revision(layer configLayer) string {
	if layer == configLayerEffective {
		return revisionOf(d.bytes(layer))
	}
	return revisionOf(d.bytes(layer))
}

func revisionOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (d *configDocument) validateRaw(layer configLayer, raw []byte) (config, error) {
	if layer == configLayerEffective {
		return config{}, errReadOnlyLayer
	}
	_, candidate, err := decodeConfigDocument(raw, d.home, false)
	if err != nil {
		return config{}, err
	}
	var effective config
	if layer == configLayerShared {
		effective, err = cloneConfig(candidate)
		if err != nil {
			return config{}, fmt.Errorf("clone shared candidate: %w", err)
		}
		mergeConfig(&effective, d.local)
	} else {
		effective, err = cloneConfig(d.shared)
		if err != nil {
			return config{}, fmt.Errorf("clone shared config: %w", err)
		}
		mergeConfig(&effective, candidate)
	}
	if err := validateConfig(&effective, d.home, true); err != nil {
		return config{}, fmt.Errorf("effective config: %w", err)
	}
	return candidate, nil
}

func (d *configDocument) saveRaw(layer configLayer, expectedRevision string, raw []byte) (configSave, error) {
	if layer == configLayerEffective {
		return configSave{}, errReadOnlyLayer
	}
	if _, err := d.validateRaw(layer, raw); err != nil {
		return configSave{}, err
	}
	before, err := os.ReadFile(d.path(layer))
	if errors.Is(err, fs.ErrNotExist) {
		before = nil
	} else if err != nil {
		return configSave{}, fmt.Errorf("read current config: %w", err)
	}
	if expectedRevision != "" && revisionOf(before) != expectedRevision {
		return configSave{}, errStaleRevision
	}
	if expectedRevision == "" && len(before) != 0 {
		return configSave{}, errStaleRevision
	}
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(d.path(layer)); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return configSave{}, fmt.Errorf("stat config: %w", statErr)
	}
	if err := atomicConfigWrite(d.path(layer), raw, mode); err != nil {
		return configSave{}, err
	}
	if err := d.reload(); err != nil {
		return configSave{}, err
	}
	return configSave{Layer: layer, Before: before, After: append([]byte(nil), raw...), Revision: d.revision(layer), Diff: unifiedConfigDiff(d.path(layer), before, raw)}, nil
}

func (d *configDocument) operationsRaw(layer configLayer, operations []configOperation) ([]byte, error) {
	if layer == configLayerEffective {
		return nil, errReadOnlyLayer
	}
	node, err := d.layerNode(layer)
	if err != nil {
		return nil, err
	}
	for _, operation := range operations {
		if err := applyConfigOperation(&node, operation); err != nil {
			return nil, err
		}
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	if _, err := d.validateRaw(layer, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (d *configDocument) applyOperations(layer configLayer, expectedRevision string, operations []configOperation) (configSave, error) {
	raw, err := d.operationsRaw(layer, operations)
	if err != nil {
		return configSave{}, err
	}
	return d.saveRaw(layer, expectedRevision, raw)
}

func (d *configDocument) layerNode(layer configLayer) (yaml.Node, error) {
	switch layer {
	case configLayerShared:
		return cloneYAMLNode(d.sharedNode)
	case configLayerLocal:
		if d.localNode == nil {
			return yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}, nil
		}
		return cloneYAMLNode(*d.localNode)
	default:
		return yaml.Node{}, errReadOnlyLayer
	}
}

func cloneYAMLNode(node yaml.Node) (yaml.Node, error) {
	data, err := yaml.Marshal(&node)
	if err != nil {
		return yaml.Node{}, err
	}
	var clone yaml.Node
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return yaml.Node{}, err
	}
	return clone, nil
}

func (d *configDocument) replaceTyped(layer configLayer, expectedRevision string, cfg config) (configSave, error) {
	if layer == configLayerEffective {
		return configSave{}, errReadOnlyLayer
	}
	node, err := d.layerNode(layer)
	if err != nil {
		return configSave{}, err
	}
	sourceData, err := yaml.Marshal(cfg)
	if err != nil {
		return configSave{}, fmt.Errorf("marshal config: %w", err)
	}
	var source yaml.Node
	if err := yaml.Unmarshal(sourceData, &source); err != nil {
		return configSave{}, err
	}
	mergeKnownMapping(rootMapping(&node), rootMapping(&source))
	if len(cfg.Hooks) == 0 {
		removeMappingKey(rootMapping(&node), "hooks")
	}
	if cfg.UI == nil {
		removeMappingKey(rootMapping(&node), "ui")
	}
	if cfg.ContextNoteTokens == nil {
		removeMappingKey(rootMapping(&node), "context_note_tokens")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		return configSave{}, fmt.Errorf("marshal config: %w", err)
	}
	return d.saveRaw(layer, expectedRevision, raw)
}

func atomicConfigWrite(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dotagents-config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("preserve config mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func saveConfigDocument(path string, home string, cfg config) error {
	if _, err := os.ReadFile(path); err == nil {
		doc, openErr := newConfigDocument(path, home)
		if openErr != nil {
			return openErr
		}
		_, err = doc.replaceTyped(configLayerShared, doc.revision(configLayerShared), cfg)
		return err
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read config %s: %w", path, err)
	} else {
		if err := validateConfig(&cfg, home, false); err != nil {
			return err
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("yaml encode: %w", err)
		}
		_, _, err = decodeConfigDocument(data, home, false)
		if err != nil {
			return err
		}
		return atomicConfigWrite(path, data, 0o644)
	}
}

func rootMapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func mappingIndex(mapping *yaml.Node, key string) int {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}
func removeMappingKey(mapping *yaml.Node, key string) {
	if idx := mappingIndex(mapping, key); idx >= 0 {
		mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
	}
}

func stableNodeKey(node *yaml.Node, section string) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	if section == "external_skills" {
		return repoName(valueString(mappingValue(node, "url")))
	}
	return strings.TrimSpace(valueString(mappingValue(node, "name")))
}

func valueString(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func findSequenceEntry(sequence *yaml.Node, section string, key string) int {
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return -1
	}
	for i, item := range sequence.Content {
		if stableNodeKey(item, section) == key || (section == "agents" && normalizeAgentName(stableNodeKey(item, section)) == normalizeAgentName(key)) {
			return i
		}
	}
	return -1
}

func mergeKnownMapping(dst, src *yaml.Node) {
	if dst == nil || src == nil || dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		key, value := src.Content[i], src.Content[i+1]
		idx := mappingIndex(dst, key.Value)
		if idx < 0 {
			dst.Content = append(dst.Content, cloneNodePtr(key), cloneNodePtr(value))
			continue
		}
		existing := dst.Content[idx+1]
		if existing.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergeKnownMapping(existing, value)
			continue
		}
		if existing.Kind == yaml.SequenceNode && value.Kind == yaml.SequenceNode {
			mergeKnownSequence(existing, value, key.Value)
			continue
		}
		preserveNodeStyle(existing, value)
	}
}

func mergeKnownSequence(dst, src *yaml.Node, section string) {
	if len(src.Content) == 0 {
		dst.Content = nil
		return
	}
	if len(dst.Content) == 0 || dst.Content[0].Kind != yaml.MappingNode || src.Content[0].Kind != yaml.MappingNode {
		dst.Content = cloneNodePtrs(src.Content)
		return
	}
	old := append([]*yaml.Node(nil), dst.Content...)
	dst.Content = nil
	for _, sourceItem := range src.Content {
		key := stableNodeKey(sourceItem, section)
		matched := -1
		for i, oldItem := range old {
			if stableNodeKey(oldItem, section) == key {
				matched = i
				break
			}
		}
		if matched >= 0 {
			item := cloneNodePtr(old[matched])
			mergeKnownMapping(item, sourceItem)
			dst.Content = append(dst.Content, item)
		} else {
			dst.Content = append(dst.Content, cloneNodePtr(sourceItem))
		}
	}
}

func preserveNodeStyle(dst, src *yaml.Node) {
	style := dst.Style
	*dst = *cloneNodePtr(src)
	dst.Style = style
}

func cloneNodePtr(node *yaml.Node) *yaml.Node {
	copy := *node
	copy.Content = cloneNodePtrs(node.Content)
	return &copy
}

func cloneNodePtrs(nodes []*yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, len(nodes))
	for i, node := range nodes {
		out[i] = cloneNodePtr(node)
	}
	return out
}

func applyConfigOperation(doc *yaml.Node, operation configOperation) error {
	segments := operationSegments(operation)
	if len(segments) == 0 {
		return errors.New("config operation path cannot be empty")
	}
	root := rootMapping(doc)
	if root.Kind != yaml.MappingNode {
		return errors.New("config document root must be a mapping")
	}
	if len(segments) == 1 {
		return mutateMapping(root, segments[0], operation, true)
	}
	section := segments[0]
	sequence := mappingValue(root, section)
	if sequence == nil {
		return fmt.Errorf("config section %q not found", section)
	}
	if sequence.Kind != yaml.SequenceNode {
		return mutateNode(sequence, segments[1:], operation)
	}
	key := segments[1]
	if key == "-" {
		if operation.Op != "add" || len(operation.Value) == 0 {
			return errors.New("appending a config entry requires op add and value")
		}
		value, err := operationValue(operation.Value)
		if err != nil {
			return err
		}
		sequence.Content = append(sequence.Content, value)
		return nil
	}
	idx := findSequenceEntry(sequence, section, key)
	if idx < 0 {
		return fmt.Errorf("config %s entry %q not found", section, key)
	}
	if len(segments) == 2 {
		if operation.Op != "replace" && operation.Op != "set" {
			return fmt.Errorf("unsupported operation %q for a config entry", operation.Op)
		}
		value, err := operationValue(operation.Value)
		if err != nil {
			return err
		}
		sequence.Content[idx] = value
		return nil
	}
	return mutateNode(sequence.Content[idx], segments[2:], operation)
}

func operationSegments(operation configOperation) []string {
	if strings.TrimSpace(operation.Path) != "" {
		parts := strings.Split(strings.TrimPrefix(operation.Path, "/"), "/")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if part == "" {
				continue
			}
			out = append(out, part)
		}
		return out
	}
	var out []string
	if operation.Section != "" {
		out = append(out, operation.Section)
	}
	if operation.Key != "" {
		out = append(out, operation.Key)
	}
	if operation.Field != "" {
		out = append(out, strings.Split(operation.Field, ".")...)
	}
	return out
}

func mutateNode(node *yaml.Node, segments []string, operation configOperation) error {
	if len(segments) == 0 {
		return errors.New("config operation path cannot be empty")
	}
	if node.Kind == yaml.MappingNode {
		return mutateMapping(node, segments[0], operation, len(segments) == 1, segments[1:]...)
	}
	if node.Kind == yaml.SequenceNode {
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= len(node.Content) {
			return fmt.Errorf("config list index %q is invalid", segments[0])
		}
		if len(segments) == 1 {
			if operation.Op == "remove" {
				node.Content = append(node.Content[:index], node.Content[index+1:]...)
				return nil
			}
			value, valueErr := operationValue(operation.Value)
			if valueErr != nil {
				return valueErr
			}
			node.Content[index] = value
			return nil
		}
		return mutateNode(node.Content[index], segments[1:], operation)
	}
	return fmt.Errorf("config path traverses scalar %q", segments[0])
}

func mutateMapping(mapping *yaml.Node, key string, operation configOperation, leaf bool, rest ...string) error {
	idx := mappingIndex(mapping, key)
	if leaf {
		if operation.Op == "remove" {
			if idx < 0 {
				return fmt.Errorf("config field %q not found", key)
			}
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
			return nil
		}
		value, err := operationValue(operation.Value)
		if err != nil {
			return err
		}
		if idx < 0 {
			mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
		} else {
			preserveNodeStyle(mapping.Content[idx+1], value)
		}
		return nil
	}
	if idx < 0 {
		return fmt.Errorf("config field %q not found", key)
	}
	return mutateNode(mapping.Content[idx+1], rest, operation)
}

func operationValue(raw json.RawMessage) (*yaml.Node, error) {
	if len(raw) == 0 {
		return nil, errors.New("config operation value is required")
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("decode config operation value: %w", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return cloneNodePtr(node.Content[0]), nil
	}
	return &node, nil
}

func unifiedConfigDiff(path string, before, after []byte) string {
	if bytes.Equal(before, after) {
		return ""
	}
	oldLines := strings.Split(string(before), "\n")
	newLines := strings.Split(string(after), "\n")
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n@@ -1,%d +1,%d @@\n", path, path, len(oldLines), len(newLines))
	for _, line := range oldLines {
		out.WriteByte('-')
		out.WriteString(line)
		out.WriteByte('\n')
	}
	for _, line := range newLines {
		out.WriteByte('+')
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

func configPathFor(opts runOptions, home string) (string, error) {
	path, err := resolveConfigPath(opts.ConfigPath, home)
	if err != nil {
		return "", err
	}
	if err := refuseWorktreeRoot(filepath.Dir(path)); err != nil {
		return "", err
	}
	return path, nil
}
