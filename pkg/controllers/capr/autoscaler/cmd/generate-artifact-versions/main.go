package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	defaultEndpoint = "https://api.apps.rancher.io/v1/components/kubernetes-cluster-autoscaler"
	defaultOutput   = "pkg/controllers/capr/autoscaler/artifact_versions_zz_generated.go"
)

type component struct {
	Branches []branch `json:"branches"`
}

type branch struct {
	BranchName string             `json:"branch_name"`
	Versions   []componentVersion `json:"versions"`
}

type componentVersion struct {
	VersionNumber string     `json:"version_number"`
	Artifacts     []artifact `json:"artifacts"`
}

type artifact struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	PackagingFormat string `json:"packaging_format"`
}

type mapping struct {
	ImageTag     string
	ChartVersion string
}

func main() {
	endpoint := flag.String("endpoint", defaultEndpoint, "component API endpoint")
	output := flag.String("output", defaultOutput, "generated Go file")
	flag.Parse()

	existing, err := readExisting(*output)
	if err != nil {
		log.Fatalf("read existing mappings: %v", err)
	}

	body, err := fetch(context.Background(), http.DefaultClient, *endpoint)
	if err != nil {
		log.Fatalf("fetch component metadata: %v", err)
	}

	var metadata component
	if err := parseMetadata(body, &metadata); err != nil {
		log.Fatalf("parse component metadata: %v", err)
	}

	mappings, warnings := selectMappings(metadata, existing)
	for _, warning := range warnings {
		log.Printf("warning: %s", warning)
	}

	if err := writeGenerated(*output, render(mappings)); err != nil {
		log.Fatalf("write generated mappings: %v", err)
	}
}

func writeGenerated(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".artifact_versions_zz_generated.go-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

func fetch(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	clientCopy := *client
	clientCopy.Timeout = 30 * time.Second
	response, err := clientCopy.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}

	return io.ReadAll(response.Body)
}

func parseMetadata(data []byte, metadata *component) error {
	if err := json.Unmarshal(data, metadata); err != nil {
		return err
	}
	if metadata.Branches == nil {
		return errors.New("response does not contain branches")
	}
	return nil
}

func selectMappings(metadata component, existing map[int]mapping) (map[int]mapping, []string) {
	result := cloneMappings(existing)
	var warnings []string

	branches := append([]branch(nil), metadata.Branches...)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].BranchName < branches[j].BranchName
	})

	for _, currentBranch := range branches {
		minor, ok := kubernetesMinor(currentBranch.BranchName)
		if !ok {
			continue
		}

		selected, ok := newestCompleteVersion(currentBranch.Versions)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Kubernetes stream %s has no version with both HELM_CHART and CONTAINER artifacts", currentBranch.BranchName))
			continue
		}

		chart, ok := newestArtifact(selected.Artifacts, "HELM_CHART")
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Kubernetes stream %s version %s has no usable HELM_CHART artifact", currentBranch.BranchName, selected.VersionNumber))
			continue
		}
		container, ok := newestArtifact(selected.Artifacts, "CONTAINER")
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Kubernetes stream %s version %s has no usable CONTAINER artifact", currentBranch.BranchName, selected.VersionNumber))
			continue
		}

		result[minor] = mapping{
			ImageTag:     containerValue(container),
			ChartVersion: chart.Version,
		}
	}

	return result, warnings
}

func kubernetesMinor(branchName string) (int, bool) {
	if !strings.HasPrefix(branchName, "1.") {
		return 0, false
	}
	minor, err := strconv.Atoi(strings.TrimPrefix(branchName, "1."))
	if err != nil || minor < 0 {
		return 0, false
	}
	return minor, true
}

func newestCompleteVersion(versions []componentVersion) (componentVersion, bool) {
	sortedVersions := append([]componentVersion(nil), versions...)
	sort.SliceStable(sortedVersions, func(i, j int) bool {
		left, leftErr := semver.NewVersion(sortedVersions[i].VersionNumber)
		right, rightErr := semver.NewVersion(sortedVersions[j].VersionNumber)
		if leftErr != nil && rightErr != nil {
			return sortedVersions[i].VersionNumber > sortedVersions[j].VersionNumber
		}
		if leftErr != nil {
			return false
		}
		if rightErr != nil {
			return true
		}
		if left.Equal(right) {
			return sortedVersions[i].VersionNumber > sortedVersions[j].VersionNumber
		}
		return left.GreaterThan(right)
	})

	for _, version := range sortedVersions {
		if _, err := semver.NewVersion(version.VersionNumber); err != nil {
			continue
		}
		if _, ok := newestArtifact(version.Artifacts, "HELM_CHART"); !ok {
			continue
		}
		if _, ok := newestArtifact(version.Artifacts, "CONTAINER"); !ok {
			continue
		}
		return version, true
	}
	return componentVersion{}, false
}

func newestArtifact(artifacts []artifact, format string) (artifact, bool) {
	var selected artifact
	found := false
	for _, current := range artifacts {
		if current.PackagingFormat != format {
			continue
		}
		if format == "HELM_CHART" {
			if _, err := semver.NewVersion(current.Version); err != nil {
				continue
			}
		} else if _, err := semver.NewVersion(containerValue(current)); err != nil {
			continue
		}
		if !found || artifactGreater(current, selected, format) {
			selected = current
			found = true
		}
	}
	return selected, found
}

func artifactGreater(left, right artifact, format string) bool {
	leftVersion := artifactVersion(left, format)
	rightVersion := artifactVersion(right, format)
	leftSemver, leftErr := semver.NewVersion(leftVersion)
	rightSemver, rightErr := semver.NewVersion(rightVersion)
	if leftErr == nil && rightErr == nil && !leftSemver.Equal(rightSemver) {
		return leftSemver.GreaterThan(rightSemver)
	}
	return leftVersion > rightVersion || (leftVersion == rightVersion && left.Name > right.Name)
}

func artifactVersion(current artifact, format string) string {
	if format == "HELM_CHART" {
		return current.Version
	}
	return containerValue(current)
}

func containerValue(current artifact) string {
	if index := strings.LastIndex(current.Name, ":"); index >= 0 {
		return current.Name[index+1:]
	}
	return ""
}

func cloneMappings(source map[int]mapping) map[int]mapping {
	result := make(map[int]mapping, len(source))
	for minor, value := range source {
		result[minor] = value
	}
	return result
}

func readExisting(path string) (map[int]mapping, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[int]mapping{}, nil
	}
	if err != nil {
		return nil, err
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
	if err != nil {
		return nil, err
	}

	result := map[int]mapping{}
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.ValueSpec)
		if !ok || len(declaration.Names) != 1 || declaration.Names[0].Name != "k8sVersionToAutoscalerChartVersions" || len(declaration.Values) != 1 {
			return true
		}

		literal, ok := declaration.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, element := range literal.Elts {
			entry, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			minor, err := strconv.Atoi(astText(entry.Key))
			if err != nil {
				continue
			}
			value := entry.Value
			if unary, ok := value.(*ast.UnaryExpr); ok && unary.Op.String() == "&" {
				value = unary.X
			}
			config, ok := value.(*ast.CompositeLit)
			if !ok {
				continue
			}
			current := mapping{}
			for _, field := range config.Elts {
				fieldValue, ok := field.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				text := astText(fieldValue.Value)
				value, err := strconv.Unquote(text)
				if err != nil {
					continue
				}
				switch astText(fieldValue.Key) {
				case "imageTag":
					current.ImageTag = value
				case "chartVersion":
					current.ChartVersion = value
				}
			}
			if current.ImageTag != "" && current.ChartVersion != "" {
				result[minor] = current
			}
		}
		return false
	})
	return result, nil
}

func astText(expression ast.Expr) string {
	var buffer bytes.Buffer
	_ = formatNode(&buffer, expression)
	return buffer.String()
}

func formatNode(buffer *bytes.Buffer, node ast.Node) error {
	return format.Node(buffer, token.NewFileSet(), node)
}

func render(mappings map[int]mapping) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("// Code generated by generate-artifact-versions; DO NOT EDIT.\n\n")
	buffer.WriteString("package autoscaler\n\n")
	buffer.WriteString("// k8sVersionToAutoscalerChartVersions maps Kubernetes minor versions to vetted\n")
	buffer.WriteString("// cluster-autoscaler image and chart versions.\n")
	buffer.WriteString("var k8sVersionToAutoscalerChartVersions = map[int]*k8sToAutoscalerVersion{\n")

	minors := make([]int, 0, len(mappings))
	for minor := range mappings {
		minors = append(minors, minor)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(minors)))
	for _, minor := range minors {
		value := mappings[minor]
		fmt.Fprintf(&buffer, "\t%d: {\n\t\timageTag:     %q,\n\t\tchartVersion: %q,\n\t},\n", minor, value.ImageTag, value.ChartVersion)
	}
	buffer.WriteString("}\n\n")
	buffer.WriteString("// This is a default value, so we never fall back to what is in the chart. This\n")
	buffer.WriteString("// ensures that we run a vetted image that exists in the Prime registry.\n")
	buffer.WriteString("var defaultChartVersionConfigs = k8sVersionToAutoscalerChartVersions[34]\n")
	return buffer.Bytes()
}
