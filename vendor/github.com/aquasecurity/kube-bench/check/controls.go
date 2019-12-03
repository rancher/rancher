// Copyright © 2017 Aqua Security Software Ltd. <info@aquasec.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package check

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

// Controls holds all controls to check for master nodes.
type Controls struct {
	ID      string   `yaml:"id" json:"id"`
	Version string   `json:"version"`
	Text    string   `json:"text"`
	Type    NodeType `json:"node_type"`
	Groups  []*Group `json:"tests"`
	Summary
}

// Group is a collection of similar checks.
type Group struct {
	ID     string   `yaml:"id" json:"section"`
	Pass   int      `json:"pass"`
	Fail   int      `json:"fail"`
	Warn   int      `json:"warn"`
	Info   int      `json:"info"`
	Text   string   `json:"desc"`
	Checks []*Check `json:"results"`
}

// Summary is a summary of the results of control checks run.
type Summary struct {
	Pass int `json:"total_pass"`
	Fail int `json:"total_fail"`
	Warn int `json:"total_warn"`
	Info int `json:"total_info"`
}

// Predicate a predicate on the given Group and Check arguments.
type Predicate func(group *Group, check *Check) bool

// NewControls instantiates a new master Controls object.
func NewControls(t NodeType, in []byte) (*Controls, error) {
	c := new(Controls)

	err := yaml.Unmarshal(in, c)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %s", err)
	}

	if t != c.Type {
		return nil, fmt.Errorf("non-%s controls file specified", t)
	}

	// Prepare audit commands
	for _, group := range c.Groups {
		for _, check := range group.Checks {
			glog.V(3).Infof("Check.ID %s", check.ID)
			check.Commands = textToCommand(check.Audit)
			if len(check.AuditConfig) > 0 {
				glog.V(3).Infof("Check.ID has audit_config %s", check.ID)
				check.ConfigCommands = textToCommand(check.AuditConfig)
			}
		}
	}

	return c, nil
}

// RunChecks runs the checks with the given Runner. Only checks for which the filter Predicate returns `true` will run.
func (controls *Controls) RunChecks(runner Runner, filter Predicate) Summary {
	var g []*Group
	m := make(map[string]*Group)
	controls.Summary.Pass, controls.Summary.Fail, controls.Summary.Warn, controls.Info = 0, 0, 0, 0

	for _, group := range controls.Groups {
		for _, check := range group.Checks {

			if !filter(group, check) {
				continue
			}

			state := runner.Run(check)
			check.TestInfo = append(check.TestInfo, check.Remediation)

			// Check if we have already added this checks group.
			if v, ok := m[group.ID]; !ok {
				// Create a group with same info
				w := &Group{
					ID:     group.ID,
					Text:   group.Text,
					Checks: []*Check{},
				}

				// Add this check to the new group
				w.Checks = append(w.Checks, check)
				summarizeGroup(w, state)

				// Add to groups we have visited.
				m[w.ID] = w
				g = append(g, w)
			} else {
				v.Checks = append(v.Checks, check)
				summarizeGroup(v, state)
			}

			summarize(controls, state)
		}
	}

	controls.Groups = g
	return controls.Summary
}

// JSON encodes the results of last run to JSON.
func (controls *Controls) JSON() ([]byte, error) {
	return json.Marshal(controls)
}

func summarize(controls *Controls, state State) {
	switch state {
	case PASS:
		controls.Summary.Pass++
	case FAIL:
		controls.Summary.Fail++
	case WARN:
		controls.Summary.Warn++
	case INFO:
		controls.Summary.Info++
	default:
		glog.Warningf("Unrecognized state %s", state)
	}
}

func summarizeGroup(group *Group, state State) {
	switch state {
	case PASS:
		group.Pass++
	case FAIL:
		group.Fail++
	case WARN:
		group.Warn++
	case INFO:
		group.Info++
	default:
		glog.Warningf("Unrecognized state %s", state)
	}
}
