package main

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/codecoverage"
	"github.com/rancher/rancher/tests/framework/pkg/session"
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
