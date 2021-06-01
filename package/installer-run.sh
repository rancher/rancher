#!/usr/bin/env sh
./helm upgrade --create-namespace --install --values "$RANCHER_VALUES" --namespace cattle-system rancher ./*.tgz
