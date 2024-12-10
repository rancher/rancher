package main

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/slickwarren/rancher-tests/actions/codecoverage"
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
