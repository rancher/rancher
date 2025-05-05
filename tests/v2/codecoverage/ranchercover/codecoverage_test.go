package main

import (
	"testing"

	codecoverage "github.com/rancher/rancher/tests/v2/codecoverage/actions"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
)

func TestRetrieveCoverageReports(t *testing.T) {
	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	require.NoError(t, err)

	err = codecoverage.KillAgentTestServicesRetrieveCoverage(client)
	require.NoError(t, err)

	err = codecoverage.KillRancherTestServicesRetrieveCoverage(client)
	require.NoError(t, err)

}
