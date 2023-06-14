//go:build validation

package standalone

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/corral"

	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CorralStandaloneTestSuite struct {
	suite.Suite
	session *session.Session
}

func (r *CorralStandaloneTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *CorralStandaloneTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	corralConfig := corral.Configurations()
	err := corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(r.T(), err, "error reading corral configs")
}

func (r *CorralStandaloneTestSuite) TestGenericCorralPackage() {
	corralPackage := corral.PackagesConfig()
	// Expecting in the future, we will be mainly running from publically available corral images
	if corralPackage.HasCustomRepo != "" {
		err := corral.SetCustomRepo(corralPackage.HasCustomRepo)
		require.Nil(r.T(), err, "error setting remote repo")
	}
	newPackages := []string{}
	if len(corralPackage.CorralPackageImages) == 0 {
		r.T().Error("No Corral Packages to Test")
	}
	for packageName, packageImage := range corralPackage.CorralPackageImages {
		newPackageName := namegen.AppendRandomString(packageName)
		newPackages = append(newPackages, newPackageName)
		corralRun, err := corral.CreateCorral(r.session, newPackageName, packageImage, corralPackage.HasDebug, corralPackage.HasDebug)
		require.NoError(r.T(), err, "error creating corral %v", packageName)
		r.T().Logf("Corral %v created successfully", packageName)
		require.NotNil(r.T(), corralRun, "corral run had no restConfig")
	}
}

func TestCorralStandaloneTestSuite(t *testing.T) {
	suite.Run(t, new(CorralStandaloneTestSuite))
}
