#!/bin/bash

cat <&0 > "$CATTLE_KUSTOMIZE_YAML"/all.yaml

kustomize build "$CATTLE_KUSTOMIZE_YAML" && rm "$CATTLE_KUSTOMIZE_YAML"/all.yaml