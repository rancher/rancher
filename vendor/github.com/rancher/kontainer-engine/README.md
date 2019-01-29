kontainer-engine
========

A tool like docker-machine to provision kubernetes cluster for different cloud providers

## Building

`make`

## Usage

`kontainer-engine create --driver $driverName [OPTIONS] cluster-name`

`kontainer-engine inspect cluster-name`

`kontainer-engine ls`

`kontainer-engine update [OPTIONS] cluster-name`

`kontainer-engine rm cluster-name`

To see what driver create options it has , run
`kontainer-engine create --driver $driverName --help`

To see what update options for a cluster , run
`kontainer-engine update --help cluster-ame`

A serviceAccountToken which binds to the clusterAdmin is automatically created for you, to see what it is, run
`kontainer-engine inspect clusterName`

The current supported driver is gke(https://cloud.google.com/container-engine/)

Before running gke driver, make sure you have the credential. To get the credential, you can run any of the steps below

`gcloud auth login` or

`export GOOGLE_APPLICATION_CREDENTIALS=$HOME/gce-credentials.json` or 

`kontainer-engine create --driver gke --gke-credential-path /path/to/credential cluster-name`


## Running

`./bin/kontainer-engine`

## Tests

Run tests with:

```
    ./run_integration_tests
```

You must have Go and [Bats](https://github.com/sstephenson/bats) installed for the tests to run.

If you are adding new tests, note that they must have a `.bats` extension to be recognized by the runner.

## Generated Code

To generate the protobuf files install protoc binary in your PATH from [v3.6.1](https://github.com/protocolbuffers/protobuf/releases/tag/v3.6.1) and then build protoc-gen-go from commit 1643683e1b54a9e88ad26d98f81400c8c9d9f4f9 and install to your PATH.  Do not update the protobuf version in the vendor.conf.  The run `./scripts/generate-protoc.sh`.

## License
Copyright (c) 2014-2016 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
