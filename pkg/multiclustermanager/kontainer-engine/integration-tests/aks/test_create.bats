#!/usr/bin/env bats

load '/usr/local/lib/bats-support/load.bash'
load '/usr/local/lib/bats-assert/load.bash'
load '../setup_and_teardown'

setup() {
    setup_environment
}

#########################
# TEST VALIDATIONS WORK #
#########################
@test "create should require a resource group" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks my-super-cluster-name

  assert_output --partial "resource group is required"
}

@test "create should require a path to a public key" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube my-super-cluster-name

  assert_output --partial path to ssh public key is required
}

@test "create should require a client id" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub my-super-cluster-name

  assert_output --partial client id is required
}

@test "create should require a client secret" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 my-super-cluster-name

  assert_output --partial client secret is required
}

@test "create should require a subscription id" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 my-super-cluster-name

  assert_output --partial subscription id is required
}

######################
# TEST START CLUSTER #
######################
@test "set up cluster" {
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 --subscription-id 1029384857 my-super-cluster-name

  assert_output --partial Cluster provisioned successfully
}

@test "it prevents duplicate cluster names" {
  ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 --subscription-id 1029384857 my-super-cluster-name
  run ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 --subscription-id 1029384857 my-super-cluster-name

  assert_output --partial Cluster my-super-cluster-name already exists
}
