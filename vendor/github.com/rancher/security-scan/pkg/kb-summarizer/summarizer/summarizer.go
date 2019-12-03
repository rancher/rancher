package summarizer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	kb "github.com/aquasecurity/kube-bench/check"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	DefaultOutputFileName        = "report.json"
	DefaultControlsDirectory     = "/cfg"
	EtcdDefaultControlsDirectory = "/etcdcfg"
	VersionMappingKey            = "version_mapping"
	ConfigFilename               = "config.yaml"
	MasterControlsFilename       = "master.yaml"
	EtcdControlsFilename         = "etcd.yaml"
	NodeControlsFilename         = "node.yaml"
	MasterResultsFilename        = "master.json"
	EtcdResultsFilename          = "etcd.json"
	NodeResultsFilename          = "node.json"
)

type Summarizer struct {
	K8sVersion            string
	ControlsDirectory     string
	EtcdControlsDirectory string
	InputDirectory        string
	OutputDirectory       string
	OutputFilename        string
	FailuresOnly          bool
	fullReport            *SummarizedReport
	groupWrappersMap      map[string]*GroupWrapper
	checkWrappersMaps     map[string]*CheckWrapper
	skip                  map[string]bool
	nodeSeen              map[NodeType]map[string]bool
}

type State string

const (
	Pass  State = "P"
	Fail  State = "F"
	Skip  State = "S"
	Mixed State = "M"

	SKIP kb.State = "SKIP"

	CheckTypeSkip = "skip"
)

type NodeType string

const (
	NodeTypeEtcd   NodeType = "e"
	NodeTypeMaster NodeType = "m"
	NodeTypeNode   NodeType = "n"
)

type CheckWrapper struct {
	ID          string                `yaml:"id" json:"id"`
	Text        string                `json:"d"`
	Type        string                `json:"-"`
	Remediation string                `json:"r"`
	State       State                 `json:"s"`
	Scored      bool                  `json:"-"`
	Result      map[kb.State][]string `json:"-"`
	NodeType    []NodeType            `json:"t"`
	Nodes       []string              `json:"n,omitempty"`
}

type GroupWrapper struct {
	ID            string          `yaml:"id" json:"id"`
	Text          string          `json:"d"`
	CheckWrappers []*CheckWrapper `json:"o"`
}

type SummarizedReport struct {
	Version       string                `json:"-"`
	Total         int                   `json:"t"`
	Fail          int                   `json:"f"`
	Pass          int                   `json:"p"`
	Skip          int                   `json:"s"`
	Nodes         map[NodeType][]string `json:"n"`
	GroupWrappers []*GroupWrapper       `json:"o"`
}

func NewSummarizer(k8sVersion, controlsDir, etcdControlsDir, inputDir, outputDir, outputFilename, skipStr string, failuresOnly bool) (*Summarizer, error) {
	s := &Summarizer{
		K8sVersion:            k8sVersion,
		ControlsDirectory:     controlsDir,
		EtcdControlsDirectory: etcdControlsDir,
		InputDirectory:        inputDir,
		OutputDirectory:       outputDir,
		OutputFilename:        outputFilename,
		FailuresOnly:          failuresOnly,
		fullReport: &SummarizedReport{
			Nodes:         map[NodeType][]string{},
			GroupWrappers: []*GroupWrapper{},
		},
		groupWrappersMap:  map[string]*GroupWrapper{},
		checkWrappersMaps: map[string]*CheckWrapper{},
		skip:              getSkipMap(skipStr),
		nodeSeen:          map[NodeType]map[string]bool{},
	}
	if err := s.loadControls(); err != nil {
		return nil, fmt.Errorf("error loading controls: %v", err)
	}
	return s, nil
}

func getSkipMap(skip string) map[string]bool {
	skipMap := map[string]bool{}
	if skip == "" {
		return skipMap
	}
	splits := strings.Split(skip, ",")
	for _, split := range splits {
		skipMap[split] = true
	}
	return skipMap
}

func (s *Summarizer) processOneResultFileForHost(results *kb.Controls, hostname string) {
	for _, group := range results.Groups {
		for _, check := range group.Checks {
			if !check.Scored {
				continue
			}
			logrus.Infof("host:%v id: %v %v", hostname, check.ID, check.State)
			printCheck(check)
			cw := s.checkWrappersMaps[check.ID]
			if cw == nil {
				logrus.Errorf("check %v found in results but not in spec", check.ID)
				continue
			}
			// Order is important here
			// User passed skip is interpreted as skip
			if s.skip[check.ID] {
				check.State = SKIP
			}
			// skip from backend config is considered as pass
			if check.Type == CheckTypeSkip {
				check.State = kb.PASS
			}

			if cw.Result[check.State] == nil {
				cw.Result[check.State] = []string{hostname}
			} else {
				cw.Result[check.State] = append(cw.Result[check.State], hostname)
			}
		}
	}
}

func (s *Summarizer) addNode(nodeType NodeType, hostname string) {
	if !s.nodeSeen[nodeType][hostname] {
		s.nodeSeen[nodeType][hostname] = true
		s.fullReport.Nodes[nodeType] = append(s.fullReport.Nodes[nodeType], hostname)
	}
}

func (s *Summarizer) summarizeForHost(hostname string) error {
	logrus.Debugf("summarizeForHost: %v", hostname)

	hostDir := fmt.Sprintf("%v/%v", s.InputDirectory, hostname)
	resultFilesPaths, err := filepath.Glob(fmt.Sprintf("%v/*.json", hostDir))
	if err != nil {
		return err
	}

	nodeTypeMapping := getResultsFileNodeTypeMapping()

	for _, resultFilePath := range resultFilesPaths {
		resultFile := filepath.Base(resultFilePath)
		nodeType, ok := nodeTypeMapping[resultFile]
		if !ok {
			logrus.Errorf("unknown result file found: %v", resultFilePath)
			continue
		}
		s.addNode(nodeType, hostname)
		logrus.Debugf("host: %v resultFile: %v", hostname, resultFile)
		// Load one result file
		// Marshal it into the results
		contents, err := ioutil.ReadFile(filepath.Clean(resultFilePath))
		if err != nil {
			return fmt.Errorf("error reading file %+v: %v", resultFilePath, err)
		}

		results := &kb.Controls{}
		err = json.Unmarshal(contents, results)
		if err != nil {
			return fmt.Errorf("error unmarshalling: %v", err)
		}
		logrus.Debugf("results: %+v", results)

		s.processOneResultFileForHost(results, hostname)
	}
	return nil
}

func (s *Summarizer) save() error {
	data, err := json.MarshalIndent(s.fullReport, "", " ")
	if err != nil {
		return fmt.Errorf("error marshaling summarized report: %v", err)
	}
	if _, err := os.Stat(s.OutputDirectory); os.IsNotExist(err) {
		if err2 := os.Mkdir(s.OutputDirectory, 0755); err2 != nil {
			return fmt.Errorf("error creating output directory: %v", err)
		}
	}
	outputFilePath := fmt.Sprintf("%s/%s", s.OutputDirectory, s.OutputFilename)
	err = ioutil.WriteFile(outputFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing report file: %v", err)
	}
	logrus.Infof("successfully saved report file: %v", outputFilePath)

	return nil
}

func (s *Summarizer) loadVersionMapping() (map[string]string, error) {
	configFileName := fmt.Sprintf("%v/%v", s.ControlsDirectory, ConfigFilename)
	v := viper.New()
	v.SetConfigFile(configFileName)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	kubeToBenchmarkMap := v.GetStringMapString(VersionMappingKey)
	if kubeToBenchmarkMap == nil || (len(kubeToBenchmarkMap) == 0) {
		return nil, fmt.Errorf("config file is missing '%v' section", VersionMappingKey)
	}
	logrus.Debugf("%v: %v", VersionMappingKey, kubeToBenchmarkMap)

	return kubeToBenchmarkMap, nil
}

func (s *Summarizer) loadControlsFromFile(filePath string) (*kb.Controls, error) {
	controls := &kb.Controls{}
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file %+v: %v", filePath, err)
	}
	err = yaml.Unmarshal(fileContents, controls)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling master controls file: %v", err)
	}
	logrus.Debugf("filePath: %v, controls: %+v", filePath, controls)
	return controls, nil
}

func getNodeTypes() []NodeType {
	return []NodeType{
		NodeTypeMaster,
		NodeTypeEtcd,
		NodeTypeNode,
	}
}

func getResultsFileNodeTypeMapping() map[string]NodeType {
	return map[string]NodeType{
		MasterResultsFilename: NodeTypeMaster,
		EtcdResultsFilename:   NodeTypeEtcd,
		NodeResultsFilename:   NodeTypeNode,
	}
}

func getNodeTypeControlsFileMapping() map[NodeType]string {
	return map[NodeType]string{
		NodeTypeMaster: MasterControlsFilename,
		NodeTypeEtcd:   EtcdControlsFilename,
		NodeTypeNode:   NodeControlsFilename,
	}
}

func (s *Summarizer) getControlsDir(nodeType NodeType) string {
	if nodeType == NodeTypeEtcd {
		return s.EtcdControlsDirectory
	}
	return s.ControlsDirectory
}

func (s *Summarizer) loadControls() error {
	mapping, err := s.loadVersionMapping()
	if err != nil {
		return fmt.Errorf("error loading version mapping: %v", err)
	}
	benchmarkVersion, ok := mapping[s.K8sVersion]
	if !ok {
		return fmt.Errorf("k8s version: %v not supported", s.K8sVersion)
	}

	controlsFiles := getNodeTypeControlsFileMapping()

	var groupWrappers []*GroupWrapper
	for nodeType, controlsFile := range controlsFiles {
		s.nodeSeen[nodeType] = map[string]bool{}
		filePath := fmt.Sprintf("%v/%v/%v", s.getControlsDir(nodeType), benchmarkVersion, controlsFile)
		controls, err := s.loadControlsFromFile(filePath)
		if err != nil {
			logrus.Errorf("error loading controls from file %v: %v", filePath, err)
			continue
		}
		for _, g := range controls.Groups {
			var gw *GroupWrapper
			if gw, ok = s.groupWrappersMap[g.ID]; !ok {
				gw = getGroupWrapper(g)
				groupWrappers = append(groupWrappers, gw)
				s.groupWrappersMap[g.ID] = gw
			}
			for _, check := range g.Checks {
				if !check.Scored {
					continue
				}
				if cw, ok := s.checkWrappersMaps[check.ID]; !ok {
					s.fullReport.Total++
					c := getCheckWrapper(check)
					c.NodeType = []NodeType{nodeType}
					gw.CheckWrappers = append(gw.CheckWrappers, c)
					s.checkWrappersMaps[check.ID] = c
				} else {
					cw.NodeType = append(cw.NodeType, nodeType)
				}
			}
		}
	}

	sort.Slice(groupWrappers, func(i, j int) bool {
		return groupWrappers[i].ID < groupWrappers[j].ID
	})
	s.fullReport.GroupWrappers = groupWrappers
	logrus.Debugf("total groups loaded: %v", len(s.fullReport.GroupWrappers))
	logrus.Debugf("total controls loaded: %v", s.fullReport.Total)
	return nil
}

func getGroupWrapper(group *kb.Group) *GroupWrapper {
	return &GroupWrapper{
		ID:            group.ID,
		Text:          group.Text,
		CheckWrappers: []*CheckWrapper{},
	}
}

func getMappedState(state kb.State) State {
	switch state {
	case kb.PASS:
		return Pass
	case kb.FAIL:
		return Fail
	case kb.WARN:
		return Fail
	case kb.INFO:
		return Fail
	case SKIP:
		return Skip
	}
	return Fail
}

func getCheckWrapper(check *kb.Check) *CheckWrapper {
	return &CheckWrapper{
		ID:          check.ID,
		Text:        check.Text,
		Type:        check.Type,
		Remediation: check.Remediation,
		Scored:      check.Scored,
		Result:      map[kb.State][]string{},
	}
}

func (s *Summarizer) getNodesMapOfCheckWrapper(check *CheckWrapper) map[string]bool {
	nodes := map[string]bool{}
	for _, nodeType := range check.NodeType {
		for _, v := range s.fullReport.Nodes[nodeType] {
			nodes[v] = true
		}
	}
	return nodes
}

func (s *Summarizer) getMissingNodesMapOfCheckWrapper(check *CheckWrapper, nodes []string) []string {
	allNodes := map[string]bool{}
	for _, nodeType := range check.NodeType {
		for _, v := range s.fullReport.Nodes[nodeType] {
			allNodes[v] = true
		}
	}
	for _, n := range nodes {
		if _, ok := allNodes[n]; ok {
			delete(allNodes, n)
		}
	}
	logrus.Debugf("ID: %v, missing nodes: %v", check.ID, allNodes)
	var missingNodes []string
	for k := range allNodes {
		missingNodes = append(missingNodes, k)
	}
	return missingNodes
}

// Logic:
// - If a check has a non-PASS state on any host, the check is considered mixed.
//   Nodes will list the ones where the check has failed.
// - If a check has all pass, then nodes is empty. All nodes in that host type have passed.
// - If a check has all fail, then nodes is empty. All nodes in that host type have failed.
// - If a check is skipped, then nodes is empty.
func (s *Summarizer) runFinalPassOnCheckWrapper(cw *CheckWrapper) {
	nodesMap := s.getNodesMapOfCheckWrapper(cw)
	nodeCount := len(nodesMap)
	logrus.Debugf("id: %v nodeCount: %v", cw.ID, nodeCount)
	if len(cw.Result) == 1 {
		if _, ok := cw.Result[kb.FAIL]; ok {
			if len(cw.Result[kb.FAIL]) == nodeCount {
				cw.State = Fail
				s.fullReport.Fail++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[kb.FAIL])
			}
			return
		}
		if _, ok := cw.Result[kb.PASS]; ok {
			if len(cw.Result[kb.PASS]) == nodeCount {
				cw.State = Pass
				s.fullReport.Pass++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[kb.PASS])
			}
			return
		}
		if _, ok := cw.Result[SKIP]; ok {
			if len(cw.Result[SKIP]) == nodeCount {
				cw.State = Skip
				s.fullReport.Skip++
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[SKIP])
			}
			return
		}
		for k := range cw.Result {
			if len(cw.Result[k]) == nodeCount {
				cw.State = Fail
				s.fullReport.Fail++
				cw.Result[k] = nil
			} else {
				cw.State = Mixed
				s.fullReport.Fail++
				cw.Nodes = s.getMissingNodesMapOfCheckWrapper(cw, cw.Result[k])
			}
		}
		return
	}
	s.fullReport.Fail++
	cw.State = Mixed
	for k := range cw.Result {
		if k == kb.PASS {
			continue
		}
		cw.Nodes = append(cw.Nodes, cw.Result[k]...)
	}
}

func (s *Summarizer) runFinalPass() error {
	logrus.Debugf("running final pass")
	groups := s.fullReport.GroupWrappers
	for _, group := range groups {
		for _, cw := range group.CheckWrappers {
			logrus.Debugf("before final pass on check")
			printCheckWrapper(cw)
			s.runFinalPassOnCheckWrapper(cw)
			logrus.Debugf("after final pass on check")
			printCheckWrapper(cw)
		}
	}

	return nil
}

func (s *Summarizer) Summarize() error {
	logrus.Infof("summarize")
	logrus.Debugf("inputDir: %v", s.InputDirectory)

	// Walk through the host folders
	hostsDir, err := ioutil.ReadDir(s.InputDirectory)
	if err != nil {
		return fmt.Errorf("error listing directory: %v", err)
	}

	for _, hostDir := range hostsDir {
		if !hostDir.IsDir() {
			continue
		}
		hostname := hostDir.Name()
		logrus.Debugf("hostDir: %v", hostname)

		if err := s.summarizeForHost(hostname); err != nil {
			return fmt.Errorf("error summarizeForHost: %v", hostname)
		}
	}

	logrus.Debugf("--- before final pass")
	s.printReport()
	if err := s.runFinalPass(); err != nil {
		return fmt.Errorf("error running final pass on the report: %v", err)
	}
	logrus.Debugf("--- before final pass")
	s.printReport()
	return s.save()
}

func (s *Summarizer) printReport() error {
	logrus.Debugf("printing report")

	for _, gw := range s.fullReport.GroupWrappers {
		for _, cw := range gw.CheckWrappers {
			printCheckWrapper(cw)
		}
	}

	bytes, err := json.MarshalIndent(s.fullReport, "", " ")
	if err != nil {
		return fmt.Errorf("error marshalling report: %v", err)
	}

	txt := string(bytes)
	logrus.Debugf("json txt: %+v", txt)
	return nil
}

func printCheck(check *kb.Check) {
	logrus.Debugf("check: ")
	logrus.Debugf("ID: %v", check.ID)
	logrus.Debugf("State: %v", check.State)
	logrus.Debugf("Text: %v", check.Text)
	logrus.Debugf("Audit: %v", check.Audit)
	logrus.Debugf("ActualValue: %v", check.ActualValue)
}
func printCheckWrapper(cw *CheckWrapper) {
	logrus.Debugf("checkWrapper:")
	logrus.Debugf("id: %v", cw.ID)
	logrus.Debugf("state: %v", cw.State)
	logrus.Debugf("node_type: %+v", cw.NodeType)
	logrus.Debugf("nodes: %+v", cw.Nodes)
	logrus.Debugf("result: %+v", cw.Result)
}
