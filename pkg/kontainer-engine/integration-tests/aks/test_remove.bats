#!/usr/bin/env bats

load '/usr/local/lib/bats-support/load.bash'
load '/usr/local/lib/bats-assert/load.bash'
load '../setup_and_teardown'

setup() {
    setup_environment
}

@test "able to remove cluster" {
    ./kontainer-engine create --base-url http://localhost:8500 --driver aks --resource-group kube --public-key ./integration-tests/test-key.pub --client-id 12345 --client-secret 67890 --subscription-id 1029384857 my-super-cluster-name-1 > /dev/null 2>&1
    ./kontainer-engine remove my-super-cluster-name-1 > /dev/null 2>&1

    # TODO this is bad but its the only way to get the requests from hoverctl because logs is broken and there is no
    # `hoverctl journal` command
    output=$(curl localhost:8888/api/v2/journal | jq ".journal[-1].request.method")
    assert_output --partial "DELETE"
}

@test "delete cluster that does not exist" {
    run ./kontainer-engine remove my-super-cluster-name-1 > /dev/null 2>&1
    assert_output --partial "cluster my-super-cluster-name-1 can't be found"

    # TODO this is bad but its the only way to get the requests from hoverctl because logs is broken and there is no
    # `hoverctl journal` command
    output=$(curl localhost:8888/api/v2/journal | jq ".journal[-1].request.method")
    refute_output --partial "DELETE"
}
