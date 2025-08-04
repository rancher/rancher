package deployer

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/version"
)

type sccOperatorParams struct {
	rancherVersion   string
	rancherGitCommit string
	sccOperatorImage string

	refreshHash string
}

// setConfigHash generates a hash based on the relevant configuration details
func setConfigHash(operatorParams *sccOperatorParams) error {
	var hashInputData []byte
	hasher := sha256.New()
	// You can add more configuration items to this hash as needed
	hashInputData = append(hashInputData, []byte(operatorParams.rancherVersion)...)
	hashInputData = append(hashInputData, []byte(operatorParams.rancherGitCommit)...)
	hashInputData = append(hashInputData, []byte(operatorParams.sccOperatorImage)...)

	// Generate the hash...
	if _, err := hasher.Write(hashInputData); err != nil {
		return err
	}
	operatorParams.refreshHash = hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func extractSccOperatorParams() (*sccOperatorParams, error) {
	// TODO: this may need to take some input to get current state
	params := &sccOperatorParams{
		rancherVersion:   version.Version,
		rancherGitCommit: version.GitCommit,
		sccOperatorImage: settings.FullSCCOperatorImage(),
	}
	if err := setConfigHash(params); err != nil {
		return nil, err
	}

	return params, nil
}

func (p sccOperatorParams) Labels() map[string]string {
	return map[string]string{
		consts.LabelK8sManagedBy:              "rancher",
		consts.LabelK8sPartOf:                 "rancher",
		consts.LabelK8sManagedBy + "-version": p.rancherVersion,
	}
}
