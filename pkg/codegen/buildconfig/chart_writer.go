package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartValuesWriter updates chart/values.yaml with values from build.yaml
// It parses the chart as YAML and updates specific paths with values from the build config
type ChartValuesWriter struct {
	Config map[string]string // build.yaml parsed config
	Chart  io.Reader         // chart/values.yaml content
	Output io.Writer         // updated chart/values.yaml
}

// Run reads the chart values, updates specific paths from the config, and writes the updated chart
func (w *ChartValuesWriter) Run() error {
	if w.Config == nil {
		return errors.New("nil config")
	}
	if err := w.processChart(); err != nil {
		return err
	}
	return nil
}

func (w *ChartValuesWriter) processChart() error {
	if w.Chart == nil {
		return errors.New("nil chart input")
	}
	if w.Output == nil {
		return errors.New("nil output")
	}

	chartContent, err := io.ReadAll(w.Chart)
	if err != nil {
		return fmt.Errorf("failed to read chart: %w", err)
	}

	// Parse chart as YAML to get line numbers for values
	var chartRoot yaml.Node
	if err := yaml.Unmarshal(chartContent, &chartRoot); err != nil {
		return fmt.Errorf("failed to parse chart YAML: %w", err)
	}

	// Collect line-based replacements instead of modifying the AST
	replacements := make(map[int]string) // line number -> new value
	if err := w.collectReplacements(&chartRoot, replacements); err != nil {
		return fmt.Errorf("failed to collect replacements: %w", err)
	}

	// Apply replacements line-by-line to preserve all formatting
	return w.applyReplacements(chartContent, replacements)
}

// applyReplacements applies line-based value replacements while preserving all formatting
func (w *ChartValuesWriter) applyReplacements(content []byte, replacements map[int]string) error {
	if len(replacements) == 0 {
		// No changes needed, write original content
		_, err := w.Output.Write(content)
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if newValue, ok := replacements[lineNum]; ok {
			// This line needs replacement - preserve indentation and key, update value only
			// Find the colon that separates key from value
			colonIdx := strings.Index(line, ":")
			if colonIdx == -1 {
				return fmt.Errorf("line %d marked for replacement but has no colon", lineNum)
			}

			// Preserve everything up to and including the colon and space
			prefix := line[:colonIdx+1]

			// Determine if the original value was quoted
			remainder := strings.TrimSpace(line[colonIdx+1:])
			var quoted bool
			if len(remainder) > 0 && remainder[0] == '"' {
				quoted = true
			}

			// Build the new line with proper quoting
			if quoted {
				fmt.Fprintf(w.Output, "%s \"%s\"\n", prefix, newValue)
			} else {
				fmt.Fprintf(w.Output, "%s %s\n", prefix, newValue)
			}
		} else {
			// No change, write original line
			fmt.Fprintln(w.Output, line)
		}
	}

	return scanner.Err()
}

func (w *ChartValuesWriter) collectReplacements(root *yaml.Node, replacements map[int]string) error {
	// The root is a document node, navigate to the mapping
	if len(root.Content) == 0 {
		return errors.New("empty chart document")
	}
	chart := root.Content[0]

	// Update audit log image from chartAuditLogImage
	if auditLogImage, ok := w.Config["chartAuditLogImage"]; ok {
		parts := strings.SplitN(auditLogImage, ":", 2)
		if len(parts) == 2 {
			if err := w.recordReplacement(chart, []string{"auditLog", "image", "repository"}, parts[0], replacements); err != nil {
				return fmt.Errorf("failed to set auditLog.image.repository: %w", err)
			}
			if err := w.recordReplacement(chart, []string{"auditLog", "image", "tag"}, parts[1], replacements); err != nil {
				return fmt.Errorf("failed to set auditLog.image.tag: %w", err)
			}
		}
	}

	// Update hooks images from defaultShellVersion
	if shellVersion, ok := w.Config["defaultShellVersion"]; ok {
		parts := strings.SplitN(shellVersion, ":", 2)
		if len(parts) == 2 {
			// Update postDelete hook image
			if err := w.recordReplacement(chart, []string{"postDelete", "image", "repository"}, parts[0], replacements); err != nil {
				return fmt.Errorf("failed to set postDelete.image.repository: %w", err)
			}
			if err := w.recordReplacement(chart, []string{"postDelete", "image", "tag"}, parts[1], replacements); err != nil {
				return fmt.Errorf("failed to set postDelete.image.tag: %w", err)
			}

			// Update preUpgrade hook image
			if err := w.recordReplacement(chart, []string{"preUpgrade", "image", "repository"}, parts[0], replacements); err != nil {
				return fmt.Errorf("failed to set preUpgrade.image.repository: %w", err)
			}
			if err := w.recordReplacement(chart, []string{"preUpgrade", "image", "tag"}, parts[1], replacements); err != nil {
				return fmt.Errorf("failed to set preUpgrade.image.tag: %w", err)
			}
		}
	}

	return nil
}

// recordReplacement navigates to a path in the YAML tree and records the line number for replacement
func (w *ChartValuesWriter) recordReplacement(node *yaml.Node, path []string, value string, replacements map[int]string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}

	// Navigate to the parent of the target
	current := node
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, err := w.findMapKey(current, key)
		if err != nil {
			return err
		}
		current = next
	}

	// Find the final value node and record its line number
	targetKey := path[len(path)-1]
	valueNode, err := w.findValueNode(current, targetKey)
	if err != nil {
		return err
	}

	// Record the replacement at this line number
	replacements[valueNode.Line] = value
	return nil
}

// findMapKey finds a key in a mapping node and returns its value node
func (w *ChartValuesWriter) findMapKey(node *yaml.Node, key string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node but got %v", node.Kind)
	}

	// MappingNode content is stored as alternating key/value pairs
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Value == key {
			return valueNode, nil
		}
	}

	return nil, fmt.Errorf("key %s not found (path doesn't exist in chart)", key)
}

// findValueNode finds the value node for a given key in a mapping
func (w *ChartValuesWriter) findValueNode(node *yaml.Node, key string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node but got %v", node.Kind)
	}

	// Find the key and return its value node
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Value == key {
			return valueNode, nil
		}
	}

	return nil, fmt.Errorf("key %s not found", key)
}
