package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const fakeCommit = "9p8pvpkuorqgf7lq9nrhyp3cc0xh5wejf2g5z1s6"
const validUrl = "git@github.com:rancher/rancher.git"
const invalidUrl = "%1A invalid not real url"
const testPath = "pkg/git/git_test.go"

func TestClone(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		commandOutput   string
		commandExitCode int
		wantErr         bool
	}{
		{
			name:            "basic clone",
			url:             validUrl,
			commandOutput:   "",
			commandExitCode: 0,
			wantErr:         false,
		},
		{
			name:            "clone, invalid url",
			url:             invalidUrl,
			commandOutput:   "",
			commandExitCode: 0,
			wantErr:         true,
		},
		{
			name:            "valid url command failed",
			url:             validUrl,
			commandOutput:   "no such repo",
			commandExitCode: -128,
			wantErr:         true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			setTestOutput(t, test.commandOutput)
			setTestExitCode(t, test.commandExitCode)
			err := setMockGitExecutable(t)
			assert.NoError(t, err, "got error when setting up mock git executable")
			err = Clone("rancher", test.url, "main")
			if test.wantErr {
				assert.Error(t, err, "expected an error but did not get one")
			} else {
				assert.NoError(t, err, "got an error but did not expect one")
			}
		})
	}
}

func TestRemoteBranchHeadCommit(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		branch          string
		commandOutput   string
		commandExitCode int
		wantErr         bool
		desiredOutput   string
	}{
		{
			name:            "basic remote",
			url:             validUrl,
			branch:          "master",
			commandOutput:   fakeCommit + "\t" + "refs/head/master",
			commandExitCode: 0,
			wantErr:         false,
			desiredOutput:   fakeCommit,
		},
		{
			name:            "invalid url",
			url:             invalidUrl,
			branch:          "master",
			commandOutput:   "Function not expected to be called",
			commandExitCode: 0,
			wantErr:         true,
			desiredOutput:   "",
		},
		{
			name:            "command fails",
			url:             validUrl,
			branch:          "master",
			commandOutput:   "failed to run command",
			commandExitCode: -1,
			wantErr:         true,
			desiredOutput:   "",
		},
		{
			name:            "bad data returned from command",
			url:             validUrl,
			branch:          "master",
			commandOutput:   "",
			commandExitCode: 0,
			wantErr:         true,
			desiredOutput:   "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			setTestOutput(t, test.commandOutput)
			setTestExitCode(t, test.commandExitCode)
			err := setMockGitExecutable(t)
			assert.NoError(t, err, "got error when setting up mock git executable")
			output, err := RemoteBranchHeadCommit(test.url, test.branch)
			if test.wantErr {
				assert.Error(t, err, "expected an error but did not get one")
			} else {
				assert.NoError(t, err, "got an error but did not expect one")
			}
			assert.Equal(t, test.desiredOutput, output, "output was not as expected")
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		commandOutput   string
		commandExitCode int
		wantOutput      bool
	}{
		{
			name:            "valid url",
			url:             validUrl,
			commandOutput:   fakeCommit + "\t" + "refs/head/master",
			commandExitCode: 0,
			wantOutput:      true,
		},
		{
			name:            "valid url, failed command",
			url:             validUrl,
			commandOutput:   "ref unavailable",
			commandExitCode: -1,
			wantOutput:      false,
		},
		{
			name:            "invalid url, bad characters",
			url:             invalidUrl,
			commandOutput:   "Function not expected to be called",
			commandExitCode: 0,
			wantOutput:      false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			setTestOutput(t, test.commandOutput)
			setTestExitCode(t, test.commandExitCode)
			err := setMockGitExecutable(t)
			assert.NoError(t, err, "got error when setting up mock git executable")
			isValid := IsValid(test.url)
			assert.Equal(t, test.wantOutput, isValid, "did not get expected output")
		})
	}
}

func TestCloneWithDepth(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		url             string
		branch          string
		depth           int
		commandOutput   string
		commandExitCode int
		wantError       bool
	}{
		{
			name:            "basic clone",
			path:            "rancher",
			url:             validUrl,
			branch:          "master",
			depth:           1,
			commandExitCode: 0,
			commandOutput:   "cloning into repo",
			wantError:       false,
		},
		{
			name:            "invalid url",
			path:            "rancher",
			url:             invalidUrl,
			branch:          "master",
			depth:           1,
			commandExitCode: 0,
			commandOutput:   "Function not expected to be called",
			wantError:       true,
		},
		{
			name:            "command error",
			path:            "rancher",
			url:             validUrl,
			branch:          "master",
			depth:           1,
			commandOutput:   "could not run command",
			commandExitCode: -1,
			wantError:       true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			setTestOutput(t, test.commandOutput)
			setTestExitCode(t, test.commandExitCode)
			err := setMockGitExecutable(t)
			assert.NoError(t, err, "got error when setting up mock git executable")
			err = CloneWithDepth(test.path, test.url, test.branch, test.depth)
			if test.wantError {
				assert.Errorf(t, err, "wanted an error but did not get one")
			} else {
				assert.NoError(t, err, "got an error but did not want one")
			}
		})
	}
}

// Function used when you need control of test main for various reasons (see pkg/git/git_test.go for an example of
// mocking calls to os.Exec)
func TestMain(m *testing.M) {
	// the path to your test. You should set this in your individual test so that you don't impact the behavior of
	// other's tests
	testPath := os.Getenv("TEST_PATH")
	switch testPath {
	case "":
		os.Exit(m.Run())
	case "pkg/git/git_test.go":
		mockGitTestBehavior()
	default:
		fmt.Printf("Undefined behavior for TEST_PATH of %s", testPath)
		os.Exit(-1)
	}
}

// mockGitTestBehavior provides mocks for pkg/git/git_test.go
func mockGitTestBehavior() {
	testOutput := os.Getenv("TEST_OUTPUT")
	testExitCode := os.Getenv("TEST_EXIT_CODE")
	exitCode, err := strconv.Atoi(testExitCode)
	if err != nil {
		_, logErr := fmt.Fprintf(os.Stderr, "Unable to convert %s to an int - exit code must be an int", testExitCode)
		if logErr != nil {
			fmt.Printf("unable to write to stderr %s", logErr)
			os.Exit(-2)
		}
		os.Exit(-1)
	}
	if exitCode != 0 {
		// non 0 exit code indicates and error, so send requested output to stderr
		_, err = fmt.Fprint(os.Stderr, testOutput)
		if err != nil {
			fmt.Printf("unable to print to stderr %s \n", err)
		}
	} else {
		fmt.Print(testOutput)
	}
	os.Exit(exitCode)
}

// setMockExecutable copies the test executable and adds it to PATH as git, so that when os.Exec calls out for git,
// it interacts with the behavior defined in main_test.go
func setMockGitExecutable(t *testing.T) error {
	t.Setenv("TEST_PATH", testPath)
	mainExec, err := os.Executable()
	if err != nil {
		return err
	}
	// add the temp directory of the test to the path, so that the mock git executable can be discovered
	tempDir := t.TempDir()
	path := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+path)
	// copy the main test executable to the new temp directory under the name of git
	testExecFile, err := os.Open(mainExec)
	defer testExecFile.Close()
	if err != nil {
		return err
	}
	testGitExec, err := os.Create(filepath.Join(tempDir, "git"))
	defer testGitExec.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(testGitExec, testExecFile)
	if err != nil {
		return err
	}
	return os.Chmod(testGitExec.Name(), 755)
}

func setTestOutput(t *testing.T, output string) {
	t.Setenv("TEST_OUTPUT", output)
}

func setTestExitCode(t *testing.T, code int) {
	t.Setenv("TEST_EXIT_CODE", strconv.Itoa(code))
}
