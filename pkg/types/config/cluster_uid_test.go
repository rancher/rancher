package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesClusterUID(t *testing.T) {
	cluster := &metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1"}

	assert.True(t, MatchesClusterUID("uid-1", cluster))
	assert.False(t, MatchesClusterUID("uid-2", cluster), "a cluster created again under the same name")
	assert.True(t, MatchesClusterUID("", cluster), "controllers started without a UID behave as before")
}

func TestIsCluster(t *testing.T) {
	c := &UserContext{ClusterName: "c-m-test", ClusterUID: "uid-1"}

	assert.True(t, c.IsCluster(&metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1"}))
	assert.False(t, c.IsCluster(&metav1.ObjectMeta{Name: "c-m-test", UID: "uid-2"}))
	assert.False(t, c.IsCluster(&metav1.ObjectMeta{Name: "c-m-other", UID: "uid-1"}))
	assert.True(t, (&UserContext{ClusterName: "c-m-test"}).IsCluster(&metav1.ObjectMeta{Name: "c-m-test", UID: "uid-2"}))
}
