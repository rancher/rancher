#!/bin/bash

if ! command -v setup-envtest &> /dev/null
then
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
fi

if [ -z "$KUBEBUILDER_ASSETS" ];
then 
    KUBEBUILDER_ASSETS=$(setup-envtest use --use-env -p path $ENVTEST_K8S_VERSION)
    export KUBEBUILDER_ASSETS
fi

go test -v $(dirname "$0")/...