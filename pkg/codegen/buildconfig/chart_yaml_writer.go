package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v4"
)

const (
	// imagesAnnotation is the HIP-0015 Chart.yaml annotation listing the images of a chart.
	imagesAnnotation = "helm.sh/images"
	// appVersionPlaceholder is only known at build time and is replaced by the chart build scripts.
	appVersionPlaceholder = "%APP_VERSION%"
	// primeRegistry is the registry prefix of images in the Prime repository.
	primeRegistry = "registry.rancher.com/"
)

// annotationImage is one entry of the helm.sh/images annotation. Name and Image are
// the HIP-0015 standard fields, and Dependency is optional in the standard.
type annotationImage struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
	// Dependency is the name of the subchart that uses the image. Only use it for real
	// subcharts listed in Chart.yaml, Rancher currently has none, see chartImages.
	Dependency string `yaml:"dependency,omitempty"`
	// OS this is the operating system of the image - unset means linux
	OS string `yaml:"os,omitempty"`
}

// chartImage describes how to build an annotationImage from build.yaml.
type chartImage struct {
	Name string
	// Key is the build.yaml key holding the full image reference (repository:tag).
	Key string
	// Ref is a fixed reference, used when the image is not tracked in build.yaml.
	Ref string
	// Repository and VersionKey build the reference from a repository and a build.yaml
	// key holding only the tag, for images tracked by version rather than by reference.
	Repository string
	VersionKey string
	// Dependency is the subchart that deploys the image, see annotationImage.
	Dependency string
	// PrimeOnly lists the image only when generating for Prime.
	PrimeOnly bool
	OS        string
}

// chartImages lists the images of the Rancher chart: the ones used by the chart's own
// files, and the ones Rancher deploys at runtime (agents, machine, system-agent, wins, and scc-operator on Prime),
// which are listed so that air-gapped installs can mirror them.
//
// WARNING: HIP-0015 itself only covers the images used by the chart's own files, plus those
// of real subcharts, which are cited with the dependency field. Rancher has no real
// subcharts, it installs charts like Turtles after it first boots, so do not use dependency
// for the runtime images.
var chartImages = []chartImage{
	{Name: "rancher", Ref: "rancher/rancher:" + appVersionPlaceholder}, // deployment.yaml
	{Name: "rancher-assets", Key: "defaultAssetsImage"},                // deployment.yaml, init container and env
	{Name: "shell", Key: "defaultShellVersion"},                        // pre-upgrade and post-delete hook jobs
	{Name: "mirrored-bci-micro", Key: "chartAuditLogImage"},            // deployment.yaml, audit log sidecar
	{Name: "kube-api-auth", Key: "defaultKubeAPIAuthImage"},
	{Name: "rancher-agent", Ref: "rancher/rancher-agent:" + appVersionPlaceholder}, // runtime, tagged like rancher
	{Name: "system-agent", Repository: "rancher/system-agent", VersionKey: "systemAgentVersion"},
	{Name: "wins", Repository: "rancher/wins", VersionKey: "winsAgentVersion", OS: "windows"},
	{Name: "machine", Key: "machineProvisionImage"},
	{Name: "scc-operator", Key: "defaultSccOperatorImage", PrimeOnly: true},
}

// ChartYamlWriter updates the helm.sh/images annotation of chart/Chart.yaml
// with the images of the chart from build.yaml, leaving the rest of the file untouched.
type ChartYamlWriter struct {
	Config map[string]string // build.yaml parsed config
	Prime  bool              // generate for the Prime repository
	Chart  io.Reader         // chart/Chart.yaml content
	Output io.Writer         // updated chart/Chart.yaml
}

// Run reads Chart.yaml, replaces the images annotation block and writes the result.
// Nothing is written to Output if an error occurs.
func (w *ChartYamlWriter) Run() error {
	if w.Config == nil {
		return errors.New("nil config")
	}
	if w.Chart == nil {
		return errors.New("nil chart input")
	}
	if w.Output == nil {
		return errors.New("nil output")
	}

	content, err := io.ReadAll(w.Chart)
	if err != nil {
		return fmt.Errorf("failed to read chart: %w", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return fmt.Errorf("failed to parse chart YAML: %w", err)
	}
	if len(root.Content) == 0 {
		return errors.New("empty chart document")
	}
	keyNode, valueNode, err := findImagesAnnotation(root.Content[0])
	if err != nil {
		return err
	}
	if valueNode.Style&yaml.LiteralStyle == 0 {
		return fmt.Errorf("%s must be a literal block scalar", imagesAnnotation)
	}
	keyIndent := keyNode.Column - 1
	// valueNode.Line is the line of the "|" indicator, so as a zero-based index it is
	// the first line of the block content.
	lines := strings.SplitAfter(string(content), "\n")
	start := valueNode.Line

	block, err := w.renderImages(keyIndent + 2)
	if err != nil {
		return err
	}

	// The block is every line after the key that is blank or indented deeper than the key.
	end := start
	for end < len(lines) {
		l := strings.TrimRight(lines[end], "\r\n")
		if strings.TrimSpace(l) != "" && indentOf(l) <= keyIndent {
			break
		}
		end++
	}
	// Keep blank lines that trail the block as separators.
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	var out bytes.Buffer
	out.WriteString(strings.Join(lines[:start], ""))
	out.WriteString(block)
	out.WriteString(strings.Join(lines[end:], ""))
	_, err = w.Output.Write(out.Bytes())
	return err
}

func findImagesAnnotation(chart *yaml.Node) (key, value *yaml.Node, err error) {
	if chart.Kind != yaml.MappingNode {
		return nil, nil, errors.New("chart document is not a mapping")
	}
	for i := 0; i+1 < len(chart.Content); i += 2 {
		if chart.Content[i].Value != "annotations" {
			continue
		}
		annotations := chart.Content[i+1]
		for j := 0; j+1 < len(annotations.Content); j += 2 {
			if annotations.Content[j].Value == imagesAnnotation {
				return annotations.Content[j], annotations.Content[j+1], nil
			}
		}
	}
	return nil, nil, fmt.Errorf("annotation %s not found in chart", imagesAnnotation)
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// renderImages renders the annotation block content, indented by the given number of spaces.
func (w *ChartYamlWriter) renderImages(indent int) (string, error) {
	var entries []annotationImage
	for _, img := range chartImages {
		if img.PrimeOnly && !w.Prime {
			continue
		}
		ref, err := w.imageRef(img)
		if err != nil {
			return "", err
		}
		if w.Prime {
			ref = primeRegistry + ref
		}
		entry := annotationImage{Name: img.Name, Image: ref, Dependency: img.Dependency}
		if img.OS != "" {
			entry.OS = img.OS
		}
		entries = append(entries, entry)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(entries); err != nil {
		return "", fmt.Errorf("failed to marshal images: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", err
	}

	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	for _, line := range strings.SplitAfter(buf.String(), "\n") {
		if line != "" {
			b.WriteString(pad + line)
		}
	}
	return b.String(), nil
}

func (w *ChartYamlWriter) imageRef(img chartImage) (string, error) {
	if img.Ref != "" {
		return img.Ref, nil
	}
	if img.VersionKey != "" {
		v := w.Config[img.VersionKey]
		if v == "" {
			return "", fmt.Errorf("build.yaml has no value for %s", img.VersionKey)
		}
		return img.Repository + ":" + v, nil
	}
	v := w.Config[img.Key]
	if v == "" {
		return "", fmt.Errorf("build.yaml has no value for %s", img.Key)
	}
	if _, tag, ok := strings.Cut(v, ":"); !ok || tag == "" {
		return "", fmt.Errorf("%s %q has no image tag", img.Key, v)
	}
	return v, nil
}
