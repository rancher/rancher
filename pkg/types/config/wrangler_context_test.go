package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

var _ MultiClusterManager = noopMCM{}

func TestNoopMultiClusterManager(t *testing.T) {
	manager := noopMCM{}
	clients, err := manager.UserContext("missing")
	require.Nil(t, clients)
	require.EqualError(t, err, "no cluster manager")
	k8s, err := manager.K8sClient("missing")
	require.Nil(t, k8s)
	require.NoError(t, err)
	conn, err := manager.ClusterDialer("missing")(context.Background(), "tcp", "localhost:443")
	require.Nil(t, conn)
	require.EqualError(t, err, "no cluster manager")
}

func TestEnableProtobufCopiesConfig(t *testing.T) {
	original := &rest.Config{Host: "https://localhost", ContentConfig: rest.ContentConfig{
		AcceptContentTypes: "original-accept", ContentType: "original-content",
	}}
	enabled := enableProtobuf(original)
	require.NotSame(t, original, enabled)
	require.Equal(t, original.Host, enabled.Host)
	require.Equal(t, "application/vnd.kubernetes.protobuf, application/json", enabled.AcceptContentTypes)
	require.Equal(t, "application/json", enabled.ContentType)
	require.Equal(t, "original-accept", original.AcceptContentTypes)
	require.Equal(t, "original-content", original.ContentType)
}
