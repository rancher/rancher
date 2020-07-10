#!/usr/bin/env bats

load '/usr/local/lib/bats-support/load.bash'
load '/usr/local/lib/bats-assert/load.bash'
load '../setup_and_teardown'

setup() {
    setup_environment
}

@test "inspect cluster" {
    ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 --subscription-id 1029384857 my-super-cluster-name-1 > /dev/null 2>&1
    run ./kontainer-engine inspect my-super-cluster-name-1

    assert_output --partial "12345"
    assert_output --partial "67890"
    assert_output --partial "1029384857"
    assert_output --partial "eastus"
    assert_output --partial "./integration-tests/test-key.pub"
    assert_output --partial "my-super-cluster-name-1"
    assert_output --partial "aks"
    assert_output --partial  "kube"
}
