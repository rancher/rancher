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
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

// NodeType indicates the type of node (master, node).
type NodeType string

// State is the state of a control check.
type State string

const (
	// PASS check passed.
	PASS State = "PASS"
	// FAIL check failed.
	FAIL State = "FAIL"
	// WARN could not carry out check.
	WARN State = "WARN"
	// INFO informational message
	INFO State = "INFO"

	// MASTER a master node
	MASTER NodeType = "master"
	// NODE a node
	NODE NodeType = "node"
	// FEDERATED a federated deployment.
	FEDERATED NodeType = "federated"

	// MANUAL Check Type
	MANUAL string = "manual"
)

// Check contains information about a recommendation in the
// CIS Kubernetes document.
type Check struct {
	ID             string      `yaml:"id" json:"test_number"`
	Text           string      `json:"test_desc"`
	Audit          string      `json:"audit"`
	AuditConfig    string      `yaml:"audit_config"`
	Type           string      `json:"type"`
	Commands       []*exec.Cmd `json:"omit"`
	ConfigCommands []*exec.Cmd `json:"omit"`
	Tests          *tests      `json:"omit"`
	Set            bool        `json:"omit"`
	Remediation    string      `json:"remediation"`
	TestInfo       []string    `json:"test_info"`
	State          `json:"status"`
	ActualValue    string `json:"actual_value"`
	Scored         bool   `json:"scored"`
	ExpectedResult string `json:"expected_result"`
}

// Runner wraps the basic Run method.
type Runner interface {
	// Run runs a given check and returns the execution state.
	Run(c *Check) State
}

// NewRunner constructs a default Runner.
func NewRunner() Runner {
	return &defaultRunner{}
}

type defaultRunner struct{}

func (r *defaultRunner) Run(c *Check) State {
	return c.run()
}

// Run executes the audit commands specified in a check and outputs
// the results.
func (c *Check) run() State {

	// Since this is an Scored check
	// without tests return a 'WARN' to alert
	// the user that this check needs attention
	if c.Scored && len(strings.TrimSpace(c.Type)) == 0 && c.Tests == nil {
		c.State = WARN
		return c.State
	}

	// If check type is skip, force result to INFO
	if c.Type == "skip" {
		c.State = INFO
		return c.State
	}

	// If check type is manual force result to WARN
	if c.Type == MANUAL {
		c.State = WARN
		return c.State
	}

	lastCommand := c.Audit
	hasAuditConfig := c.ConfigCommands != nil

	state, finalOutput, retErrmsgs := performTest(c.Audit, c.Commands, c.Tests)
	if len(state) > 0 {
		c.State = state
		return c.State
	}
	errmsgs := retErrmsgs

	// If something went wrong with the 'Audit' command
	// and an 'AuditConfig' command was provided, use it to
	// execute tests
	if (finalOutput == nil || !finalOutput.testResult) && hasAuditConfig {
		lastCommand = c.AuditConfig

		nItems := len(c.Tests.TestItems)
		// The reason we're creating a copy of the "tests"
		// is so that tests can executed
		// with the AuditConfig command
		// against the Path only
		currentTests := &tests{
			BinOp:     c.Tests.BinOp,
			TestItems: make([]*testItem, nItems),
		}

		for i := 0; i < nItems; i++ {
			ti := c.Tests.TestItems[i]
			nti := &testItem{
				// Path is used to test Command Param values
				// AuditConfig ==> Path
				Path:    ti.Path,
				Set:     ti.Set,
				Compare: ti.Compare,
			}
			currentTests.TestItems[i] = nti
		}

		state, finalOutput, retErrmsgs = performTest(c.AuditConfig, c.ConfigCommands, currentTests)
		if len(state) > 0 {
			c.State = state
			return c.State
		}
		errmsgs += retErrmsgs
	}

	if finalOutput != nil && finalOutput.testResult {
		c.State = PASS
		c.ActualValue = finalOutput.actualResult
		c.ExpectedResult = finalOutput.ExpectedResult
		glog.V(3).Infof("Check.ID: %s Command: %q TestResult: %t Score: %q \n", c.ID, lastCommand, finalOutput.testResult, c.State)
	} else {
		if c.Scored {
			c.State = FAIL
		} else {
			c.State = WARN
		}
	}

	if finalOutput == nil {
		glog.V(3).Infof("Check.ID: %s Command: %q TestResult: <<EMPTY>> \n", c.ID, lastCommand)
	}

	if errmsgs != "" {
		glog.V(2).Info(errmsgs)
	}
	return c.State
}

// textToCommand transforms an input text representation of commands to be
// run into a slice of commands.
// TODO: Make this more robust.
func textToCommand(s string) []*exec.Cmd {
	glog.V(3).Infof("textToCommand: %q\n", s)
	cmds := []*exec.Cmd{}

	cp := strings.Split(s, "|")

	for _, v := range cp {
		v = strings.Trim(v, " ")

		// TODO:
		// GOAL: To split input text into arguments for exec.Cmd.
		//
		// CHALLENGE: The input text may contain quoted strings that
		// must be passed as a unit to exec.Cmd.
		// eg. bash -c 'foo bar'
		// 'foo bar' must be passed as unit to exec.Cmd if not the command
		// will fail when it is executed.
		// eg. exec.Cmd("bash", "-c", "foo bar")
		//
		// PROBLEM: Current solution assumes the grouped string will always
		// be at the end of the input text.
		re := regexp.MustCompile(`^(.*)(['"].*['"])$`)
		grps := re.FindStringSubmatch(v)

		var cs []string
		if len(grps) > 0 {
			s := strings.Trim(grps[1], " ")
			cs = strings.Split(s, " ")

			s1 := grps[len(grps)-1]
			s1 = strings.Trim(s1, "'\"")

			cs = append(cs, s1)
		} else {
			cs = strings.Split(v, " ")
		}

		cmd := exec.Command(cs[0], cs[1:]...)
		cmds = append(cmds, cmd)
	}

	return cmds
}

func isShellCommand(s string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+s)

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if strings.Contains(string(out), s) {
		return true
	}
	return false
}

func performTest(audit string, commands []*exec.Cmd, tests *tests) (State, *testOutput, string) {
	if len(strings.TrimSpace(audit)) == 0 {
		return "", failTestItem("missing command"), ""
	}

	var out bytes.Buffer
	state, retErrmsgs := runExecCommands(audit, commands, &out)
	if len(state) > 0 {
		return state, nil, ""
	}
	errmsgs := retErrmsgs

	finalOutput := tests.execute(out.String())
	if finalOutput == nil {
		errmsgs += fmt.Sprintf("Final output is <<EMPTY>>. Failed to run: %s\n", audit)
	}

	return "", finalOutput, errmsgs
}

func runExecCommands(audit string, commands []*exec.Cmd, out *bytes.Buffer) (State, string) {
	var err error
	errmsgs := ""

	// Check if command exists or exit with WARN.
	for _, cmd := range commands {
		if !isShellCommand(cmd.Path) {
			return WARN, errmsgs
		}
	}

	// Run commands.
	n := len(commands)
	if n == 0 {
		// Likely a warning message.
		return WARN, errmsgs
	}

	// Each command runs,
	//   cmd0 out -> cmd1 in, cmd1 out -> cmd2 in ... cmdn out -> os.stdout
	//   cmd0 err should terminate chain
	cs := commands

	// Initialize command pipeline
	cs[n-1].Stdout = out
	i := 1

	for i < n {
		cs[i-1].Stdout, err = cs[i].StdinPipe()
		if err != nil {
			errmsgs += fmt.Sprintf("failed to run: %s, command: %s, error: %s\n", audit, cs[i].Args, err)
		}
		i++
	}

	// Start command pipeline
	i = 0
	for i < n {
		err := cs[i].Start()
		if err != nil {
			errmsgs += fmt.Sprintf("failed to run: %s, command: %s, error: %s\n", audit, cs[i].Args, err)
		}
		i++
	}

	// Complete command pipeline
	i = 0
	for i < n {
		err := cs[i].Wait()
		if err != nil {
			errmsgs += fmt.Sprintf("failed to run: %s, command: %s, error: %s\n", audit, cs[i].Args, err)
		}

		if i < n-1 {
			cs[i].Stdout.(io.Closer).Close()
		}
		i++
	}

	glog.V(3).Infof("Command %q - Output:\n\n %s\n", audit, out.String())
	return "", errmsgs
}
