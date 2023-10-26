package tfexec

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/pkg/errors"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const (
	debugFlag            = "--trace"
	skipCleanupFlag      = "--skip-cleanup"
	tfVersionConstraint  = "<= 1.5.7"
	tfWorkspacePrefix    = "dartboard-"
	tfPlanFilePathPrefix = "tfexec_plan_"
)

type Client struct {
	// Client used to access Terraform
	Terraform       *tfexec.Terraform
	TerraformConfig *Config
}

// NewClient initializes a new instance of the tfexec Client
func NewClient() (*Client, error) {
	tfConfig := TerraformConfig()

	tf, err := tfexec.NewTerraform(tfConfig.WorkingDir, tfConfig.ExecPath)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create new Terraform instance")
	}

	c := &Client{
		Terraform:       tf,
		TerraformConfig: tfConfig,
	}
	return c, nil
}

func (c *Client) InitTerraform(opts ...tfexec.InitOption) error {
	err := c.Terraform.Init(context.Background(), opts...)
	if err != nil {
		return errors.Wrap(err, "InitTerraform: ")
	}
	return nil
}

// InstallTfVersion uses hc-install in order to install the desired Terraform version
// Returns the execPath of the newly installed Terraform binary
func InstallTfVersion(tfVersion string) (string, error) {
	v, err := version.NewVersion(tfVersion)
	if err != nil {
		return "", errors.Wrap(err, "InstallTfVersion: ")
	}
	tfConstraint, err := version.NewConstraint(tfVersionConstraint)
	if err != nil {
		return "", errors.Wrap(err, "InstallTfVersion: ")
	}
	if !tfConstraint.Check(v) {
		return "", errors.New("InstallTfVersion: version string '" + tfVersion + "'did not meet constraint criteria of " + tfVersionConstraint)
	}

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(tfVersion)),
	}
	execPath, err := installer.Install(context.Background())
	if err != nil {
		return "", errors.Wrap(err, "InstallTfVersion: failed to install Terraform")
	}

	return execPath, nil
}

func (c *Client) ShowState(opts ...tfexec.ShowOption) (*tfjson.State, error) {
	state, err := c.Terraform.Show(context.Background(), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "ShowState: ")
	}
	return state, nil
}

func (c *Client) WorkspaceExists(workspace string) (bool, error) {
	wsList, _, err := c.Terraform.WorkspaceList(context.Background())
	if err != nil {
		return false, errors.Wrap(err, "WorkspaceExists: ")
	}
	for _, str := range wsList {
		if str == workspace {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) SetupWorkspace(opts ...tfexec.WorkspaceNewCmdOption) error {
	ws := ""
	if c.TerraformConfig.WorkspaceName != "" {
		ws = c.TerraformConfig.WorkspaceName
	}

	var err error
	if ws != "" {
		wsExists, _ := c.WorkspaceExists(ws)
		if wsExists {
			err = c.Terraform.WorkspaceSelect(context.Background(), ws)
		} else {
			err = c.Terraform.WorkspaceNew(context.Background(), ws, opts...)
		}
		c.TerraformConfig.WorkspaceName = ws
	} else {
		c.TerraformConfig.WorkspaceName = tfWorkspacePrefix + namegen.RandStringLower(5)
		err = c.Terraform.WorkspaceNew(context.Background(), c.TerraformConfig.WorkspaceName, opts...)
	}
	if err != nil {
		return errors.Wrap(err, "SetupWorkspace: ")
	}
	return nil
}

func (c *Client) PlanJSON(w io.Writer, opts ...tfexec.PlanOption) error {
	hasOutOpt, hasVarFileOpt := false, false
	var parsedOpts []tfexec.PlanOption
	for _, opt := range opts {
		switch opt.(type) {
		case *tfexec.OutOption:
			hasOutOpt = true
		case *tfexec.VarFileOption:
			hasVarFileOpt = true
		default:
			parsedOpts = append(parsedOpts, tfexec.PlanOption(opt))
		}
	}

	if !hasOutOpt {
		if c.TerraformConfig.PlanFilePath == "" {
			if c.TerraformConfig.PlanOpts == nil || c.TerraformConfig.PlanOpts.OutDir == "" {
				errors.New("PlanJSON: PlanOpts configuration field is nil or PlanOpts.OutDir is empty. Could not generate PlanFilePath for output")
			}
			c.TerraformConfig.PlanFilePath = c.TerraformConfig.PlanOpts.OutDir + tfPlanFilePathPrefix + time.Now().Format(time.RFC3339)
		}
		parsedOpts = append(parsedOpts, tfexec.PlanOption(tfexec.Out(c.TerraformConfig.PlanFilePath)))
	}
	if !hasVarFileOpt && c.TerraformConfig.VarFilePath != "" {
		parsedOpts = append(parsedOpts, tfexec.PlanOption(tfexec.VarFile(c.TerraformConfig.VarFilePath)))
	}

	_, err := c.Terraform.PlanJSON(context.Background(), w, parsedOpts...)
	if err != nil {
		return errors.Wrap(err, "PlanJSON: ")
	}
	return nil
}

func (c *Client) ApplyPlanJSON(w io.Writer, opts ...tfexec.ApplyOption) error {
	hasPlanFileOpt := false
	var parsedOpts []tfexec.ApplyOption
	for _, opt := range opts {
		switch opt.(type) {
		case *tfexec.DirOrPlanOption:
			hasPlanFileOpt = true
		default:
			parsedOpts = append(parsedOpts, tfexec.ApplyOption(opt))
		}
	}

	if !hasPlanFileOpt {
		if c.TerraformConfig.PlanFilePath == "" {
			return errors.New("ApplyPlanJSON: No PlanFilePath or tfexec.DirOrPlanOption was provided")
		}
		parsedOpts = append(parsedOpts, tfexec.ApplyOption(tfexec.DirOrPlan(c.TerraformConfig.PlanFilePath)))
	}

	err := c.Terraform.ApplyJSON(context.Background(), w, parsedOpts...)
	if err != nil {
		return errors.Wrap(err, "ApplyPlanJSON: ")
	}

	return nil
}

func (c *Client) Output(opts ...tfexec.OutputOption) ([]map[string]any, error) {
	outputs, err := c.Terraform.Output(context.Background(), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "Output: ")
	}

	var parsedOutput []map[string]any
	for key, output := range outputs {
		var val any

		err := json.Unmarshal(output.Value, &val)
		if err != nil {
			return nil, errors.Wrap(err, "Output: ")
		}

		tempMap := map[string]any{key: val}
		parsedOutput = append(parsedOutput, tempMap)
	}
	return parsedOutput, nil
}

func (c *Client) WorkingDir() string {
	return c.Terraform.WorkingDir()
}

func (c *Client) DestroyJSON(w io.Writer, opts ...tfexec.DestroyOption) error {
	hasVarFileOpt := false
	var parsedOpts []tfexec.DestroyOption
	for _, opt := range opts {
		switch opt.(type) {
		case *tfexec.VarFileOption:
			hasVarFileOpt = true
		default:
			parsedOpts = append(parsedOpts, tfexec.DestroyOption(opt))
		}
	}

	if !hasVarFileOpt {
		if c.TerraformConfig.VarFilePath != "" {
			parsedOpts = append(parsedOpts, tfexec.DestroyOption(tfexec.VarFile(c.TerraformConfig.VarFilePath)))
		}
	}

	err := c.Terraform.DestroyJSON(context.Background(), w, parsedOpts...)
	if err != nil {
		return errors.Wrap(err, "Destroy: ")
	}

	return nil
}
