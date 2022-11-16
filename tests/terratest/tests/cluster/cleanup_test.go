package tests

import (
	"testing"

	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
)

func TestCleanup(t *testing.T) {
	t.Parallel()

	terratest.ForceCleanup(t)
}
